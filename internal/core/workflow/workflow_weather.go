package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/chentianyu/celestia/internal/models"
)

const workflowWeatherBodyLimit = 1 << 20

type weatherNodeConfig struct {
	Location         string   `json:"location"`
	Latitude         float64  `json:"latitude,omitempty"`
	Longitude        float64  `json:"longitude,omitempty"`
	Timezone         string   `json:"timezone,omitempty"`
	Providers        []string `json:"providers,omitempty"`
	WeatherComCityID string   `json:"weather_com_city_id,omitempty"`
	QWeatherKey      string   `json:"qweather_key,omitempty"`
	CaiyunToken      string   `json:"caiyun_token,omitempty"`
}

type workflowWeatherPoint struct {
	Location  string
	Latitude  float64
	Longitude float64
	Timezone  string
}

type workflowWeatherReport struct {
	Provider string
	Text     string
	Raw      string
}

func (e *workflowExecutor) executeWeatherNode(node models.AgentWorkflowNode) (workflowNodeValue, string, map[string]any, error) {
	config, err := decodeWorkflowNodeData[weatherNodeConfig](node.Data)
	if err != nil {
		return workflowNodeValue{}, "", nil, err
	}
	triggerEdges := e.incomingByHandle(node.ID, "trigger")
	if len(triggerEdges) > 0 {
		triggerInputs, inputErr := e.collect(node.ID, "trigger")
		if inputErr != nil {
			return workflowNodeValue{}, "", nil, inputErr
		}
		if triggerInputs.hasBlockingWindow() {
			return workflowNodeValue{Blocked: true, BlockedByWindow: true}, "Weather outside time window", map[string]any{
				"blocked_by_window": true,
				"input_count":       triggerInputs.count(),
			}, nil
		}
		if triggerInputs.triggers == 0 && triggerInputs.onlyBlockedByTimer() {
			return workflowNodeValue{Blocked: true, BlockedByTimer: true}, "Weather waiting for timer trigger", map[string]any{
				"blocked_by_timer": true,
				"input_count":      triggerInputs.count(),
			}, nil
		}
		if !triggerInputs.hasActiveContent() {
			return workflowNodeValue{Blocked: true}, "Weather waiting for trigger input", map[string]any{
				"blocked_by_upstream": true,
				"input_count":         triggerInputs.count(),
			}, nil
		}
	}
	point, err := workflowWeatherPointFromConfig(e.ctx, config)
	if err != nil {
		return workflowNodeValue{}, "", nil, err
	}
	providers := workflowWeatherProviders(config)
	reports := make([]workflowWeatherReport, 0, len(providers))
	errorCount := 0
	for _, provider := range providers {
		report, fetchErr := fetchWorkflowWeather(e.ctx, provider, point, config)
		if fetchErr != nil {
			errorCount++
			e.fetchErrors = append(e.fetchErrors, models.AgentRunError{Target: "weather:" + provider, Error: fetchErr.Error()})
			continue
		}
		reports = append(reports, report)
	}
	if len(reports) == 0 {
		return workflowNodeValue{}, "", nil, errors.New("weather node could not fetch any provider")
	}
	text := workflowWeatherText(point, reports)
	return workflowNodeValue{Text: text}, fmt.Sprintf("%d weather providers", len(reports)), map[string]any{
		"location":       point.Location,
		"provider_count": len(reports),
		"error_count":    errorCount,
	}, nil
}

func workflowWeatherPointFromConfig(ctx context.Context, config weatherNodeConfig) (workflowWeatherPoint, error) {
	timezone := firstNonEmpty(strings.TrimSpace(config.Timezone), "Asia/Shanghai")
	location := strings.TrimSpace(config.Location)
	point := workflowWeatherPoint{Location: location, Latitude: config.Latitude, Longitude: config.Longitude, Timezone: timezone}
	if validWeatherCoordinate(point.Latitude, point.Longitude) {
		if point.Location == "" {
			point.Location = fmt.Sprintf("%.4f,%.4f", point.Latitude, point.Longitude)
		}
		return point, nil
	}
	if location == "" {
		return workflowWeatherPoint{}, errors.New("weather node requires location or latitude/longitude")
	}
	geo, err := geocodeWorkflowWeatherOpenMeteo(ctx, location, timezone)
	if err != nil {
		return workflowWeatherPoint{}, err
	}
	return geo, nil
}

func workflowWeatherProviders(config weatherNodeConfig) []string {
	if len(config.Providers) > 0 {
		return uniqueWorkflowStrings(config.Providers)
	}
	providers := []string{"open_meteo", "wttr_in"}
	if strings.TrimSpace(config.WeatherComCityID) != "" {
		providers = append(providers, "weather_com_cn")
	}
	if strings.TrimSpace(config.QWeatherKey) != "" {
		providers = append(providers, "qweather")
	}
	if strings.TrimSpace(config.CaiyunToken) != "" {
		providers = append(providers, "caiyun")
	}
	return providers
}

func fetchWorkflowWeather(ctx context.Context, provider string, point workflowWeatherPoint, config weatherNodeConfig) (workflowWeatherReport, error) {
	switch strings.TrimSpace(provider) {
	case "open_meteo":
		return fetchWorkflowWeatherOpenMeteo(ctx, point)
	case "wttr_in":
		return fetchWorkflowWeatherWttr(ctx, point)
	case "weather_com_cn":
		return fetchWorkflowWeatherComCN(ctx, config)
	case "qweather":
		return fetchWorkflowWeatherQWeather(ctx, point, config)
	case "caiyun":
		return fetchWorkflowWeatherCaiyun(ctx, point, config)
	default:
		return workflowWeatherReport{}, fmt.Errorf("unsupported weather provider %q", provider)
	}
}

func geocodeWorkflowWeatherOpenMeteo(ctx context.Context, location string, timezone string) (workflowWeatherPoint, error) {
	endpoint := "https://geocoding-api.open-meteo.com/v1/search?count=1&language=zh&format=json&name=" + url.QueryEscape(location)
	var payload struct {
		Results []struct {
			Name      string  `json:"name"`
			Country   string  `json:"country"`
			Admin1    string  `json:"admin1"`
			Latitude  float64 `json:"latitude"`
			Longitude float64 `json:"longitude"`
			Timezone  string  `json:"timezone"`
		} `json:"results"`
	}
	if err := fetchWorkflowWeatherJSON(ctx, endpoint, &payload); err != nil {
		return workflowWeatherPoint{}, err
	}
	if len(payload.Results) == 0 {
		return workflowWeatherPoint{}, fmt.Errorf("weather geocoding found no location for %q", location)
	}
	result := payload.Results[0]
	name := strings.Join(orderedWorkflowStrings([]string{result.Country, result.Admin1, result.Name}), " ")
	return workflowWeatherPoint{
		Location:  firstNonEmpty(name, location),
		Latitude:  result.Latitude,
		Longitude: result.Longitude,
		Timezone:  firstNonEmpty(result.Timezone, timezone, "Asia/Shanghai"),
	}, nil
}

func fetchWorkflowWeatherOpenMeteo(ctx context.Context, point workflowWeatherPoint) (workflowWeatherReport, error) {
	params := url.Values{}
	params.Set("latitude", strconv.FormatFloat(point.Latitude, 'f', 5, 64))
	params.Set("longitude", strconv.FormatFloat(point.Longitude, 'f', 5, 64))
	params.Set("timezone", point.Timezone)
	params.Set("forecast_days", "1")
	params.Set("current", "temperature_2m,relative_humidity_2m,apparent_temperature,precipitation,weather_code,wind_speed_10m")
	params.Set("daily", "weather_code,temperature_2m_max,temperature_2m_min,precipitation_sum,precipitation_probability_max")
	var payload struct {
		Current struct {
			Temperature   float64 `json:"temperature_2m"`
			Apparent      float64 `json:"apparent_temperature"`
			Humidity      float64 `json:"relative_humidity_2m"`
			Precipitation float64 `json:"precipitation"`
			WeatherCode   int     `json:"weather_code"`
			WindSpeed     float64 `json:"wind_speed_10m"`
		} `json:"current"`
		Daily struct {
			WeatherCode       []int     `json:"weather_code"`
			TemperatureMax    []float64 `json:"temperature_2m_max"`
			TemperatureMin    []float64 `json:"temperature_2m_min"`
			PrecipitationSum  []float64 `json:"precipitation_sum"`
			PrecipitationProb []float64 `json:"precipitation_probability_max"`
		} `json:"daily"`
	}
	if err := fetchWorkflowWeatherJSON(ctx, "https://api.open-meteo.com/v1/forecast?"+params.Encode(), &payload); err != nil {
		return workflowWeatherReport{}, err
	}
	condition := openMeteoWeatherCodeText(payload.Current.WeatherCode)
	if len(payload.Daily.WeatherCode) > 0 {
		condition = openMeteoWeatherCodeText(payload.Daily.WeatherCode[0])
	}
	text := fmt.Sprintf("Open-Meteo：%s，当前 %s，体感 %s，湿度 %s，风速 %s，当前降水 %s",
		condition,
		formatWeatherTemperature(payload.Current.Temperature),
		formatWeatherTemperature(payload.Current.Apparent),
		formatWeatherPercent(payload.Current.Humidity),
		formatWeatherSpeed(payload.Current.WindSpeed),
		formatWeatherMillimeter(payload.Current.Precipitation),
	)
	if len(payload.Daily.TemperatureMax) > 0 && len(payload.Daily.TemperatureMin) > 0 {
		text += fmt.Sprintf("；今日 %s-%s，降水概率 %s，降水量 %s",
			formatWeatherTemperature(payload.Daily.TemperatureMin[0]),
			formatWeatherTemperature(payload.Daily.TemperatureMax[0]),
			formatWeatherPercent(firstWeatherFloat(payload.Daily.PrecipitationProb)),
			formatWeatherMillimeter(firstWeatherFloat(payload.Daily.PrecipitationSum)),
		)
	}
	return workflowWeatherReport{Provider: "open_meteo", Text: text}, nil
}

func fetchWorkflowWeatherWttr(ctx context.Context, point workflowWeatherPoint) (workflowWeatherReport, error) {
	query := firstNonEmpty(strings.TrimSpace(point.Location), fmt.Sprintf("%.5f,%.5f", point.Latitude, point.Longitude))
	endpoint := "https://wttr.in/" + url.PathEscape(query) + "?format=j1&lang=zh"
	var payload struct {
		Current []struct {
			TempC       string `json:"temp_C"`
			FeelsLikeC  string `json:"FeelsLikeC"`
			Humidity    string `json:"humidity"`
			WindKmph    string `json:"windspeedKmph"`
			PrecipMM    string `json:"precipMM"`
			Description []struct {
				Value string `json:"value"`
			} `json:"lang_zh"`
		} `json:"current_condition"`
		Weather []struct {
			MaxTempC string `json:"maxtempC"`
			MinTempC string `json:"mintempC"`
			Hourly   []struct {
				ChanceOfRain string `json:"chanceofrain"`
			} `json:"hourly"`
		} `json:"weather"`
	}
	if err := fetchWorkflowWeatherJSON(ctx, endpoint, &payload); err != nil {
		return workflowWeatherReport{}, err
	}
	if len(payload.Current) == 0 {
		return workflowWeatherReport{}, errors.New("wttr.in returned no current condition")
	}
	current := payload.Current[0]
	condition := firstWeatherDescription(current.Description)
	text := fmt.Sprintf("wttr.in：%s，当前 %s，体感 %s，湿度 %s，风速 %s，降水 %s",
		firstNonEmpty(condition, "未知"),
		formatWeatherRawTemperature(current.TempC),
		formatWeatherRawTemperature(current.FeelsLikeC),
		formatWeatherRawPercent(current.Humidity),
		formatWeatherRawSpeed(current.WindKmph),
		formatWeatherRawMillimeter(current.PrecipMM),
	)
	if len(payload.Weather) > 0 {
		text += fmt.Sprintf("；今日 %s-%s，最高降雨概率 %s",
			formatWeatherRawTemperature(payload.Weather[0].MinTempC),
			formatWeatherRawTemperature(payload.Weather[0].MaxTempC),
			formatWeatherRawPercent(maxWttrRainChance(payload.Weather[0].Hourly)),
		)
	}
	return workflowWeatherReport{Provider: "wttr_in", Text: text}, nil
}

func fetchWorkflowWeatherComCN(ctx context.Context, config weatherNodeConfig) (workflowWeatherReport, error) {
	cityID := strings.TrimSpace(config.WeatherComCityID)
	if cityID == "" {
		return workflowWeatherReport{}, errors.New("weather.com.cn city id is required")
	}
	var payload struct {
		WeatherInfo struct {
			City    string `json:"city"`
			CityID  string `json:"cityid"`
			Temp1   string `json:"temp1"`
			Temp2   string `json:"temp2"`
			Weather string `json:"weather"`
			PTime   string `json:"ptime"`
		} `json:"weatherinfo"`
	}
	if err := fetchWorkflowWeatherJSON(ctx, "https://www.weather.com.cn/data/cityinfo/"+url.PathEscape(cityID)+".html", &payload); err != nil {
		return workflowWeatherReport{}, err
	}
	info := payload.WeatherInfo
	if strings.TrimSpace(info.City) == "" && strings.TrimSpace(info.Weather) == "" {
		return workflowWeatherReport{}, errors.New("weather.com.cn returned empty weather info")
	}
	text := fmt.Sprintf("中国天气：%s %s，今日 %s-%s，发布时间 %s",
		firstNonEmpty(info.City, info.CityID, cityID),
		firstNonEmpty(info.Weather, "未知"),
		firstNonEmpty(info.Temp1, "未知"),
		firstNonEmpty(info.Temp2, "未知"),
		firstNonEmpty(info.PTime, "未知"),
	)
	return workflowWeatherReport{Provider: "weather_com_cn", Text: text}, nil
}

func fetchWorkflowWeatherQWeather(ctx context.Context, point workflowWeatherPoint, config weatherNodeConfig) (workflowWeatherReport, error) {
	key := strings.TrimSpace(config.QWeatherKey)
	if key == "" {
		return workflowWeatherReport{}, errors.New("qweather key is required")
	}
	location := fmt.Sprintf("%.5f,%.5f", point.Longitude, point.Latitude)
	params := url.Values{}
	params.Set("location", location)
	params.Set("key", key)
	var payload struct {
		Now struct {
			Temp      string `json:"temp"`
			FeelsLike string `json:"feelsLike"`
			Text      string `json:"text"`
			WindDir   string `json:"windDir"`
			WindScale string `json:"windScale"`
			Humidity  string `json:"humidity"`
			Precip    string `json:"precip"`
		} `json:"now"`
	}
	if err := fetchWorkflowWeatherJSON(ctx, "https://devapi.qweather.com/v7/weather/now?"+params.Encode(), &payload); err != nil {
		return workflowWeatherReport{}, err
	}
	text := fmt.Sprintf("和风天气：%s，当前 %s，体感 %s，湿度 %s，%s %s级，降水 %s",
		firstNonEmpty(payload.Now.Text, "未知"),
		formatWeatherRawTemperature(payload.Now.Temp),
		formatWeatherRawTemperature(payload.Now.FeelsLike),
		formatWeatherRawPercent(payload.Now.Humidity),
		firstNonEmpty(payload.Now.WindDir, "风向未知"),
		firstNonEmpty(payload.Now.WindScale, "未知"),
		formatWeatherRawMillimeter(payload.Now.Precip),
	)
	return workflowWeatherReport{Provider: "qweather", Text: text}, nil
}

func fetchWorkflowWeatherCaiyun(ctx context.Context, point workflowWeatherPoint, config weatherNodeConfig) (workflowWeatherReport, error) {
	token := strings.TrimSpace(config.CaiyunToken)
	if token == "" {
		return workflowWeatherReport{}, errors.New("caiyun token is required")
	}
	endpoint := fmt.Sprintf("https://api.caiyunapp.com/v2.6/%s/%.5f,%.5f/weather?dailysteps=1&hourlysteps=24&alert=true", url.PathEscape(token), point.Longitude, point.Latitude)
	var payload struct {
		Result struct {
			Realtime struct {
				Temperature float64 `json:"temperature"`
				Skycon      string  `json:"skycon"`
				Humidity    float64 `json:"humidity"`
				Wind        struct {
					Speed float64 `json:"speed"`
				} `json:"wind"`
				Precipitation struct {
					Local struct {
						Intensity float64 `json:"intensity"`
					} `json:"local"`
				} `json:"precipitation"`
			} `json:"realtime"`
			ForecastKeypoint string `json:"forecast_keypoint"`
		} `json:"result"`
	}
	if err := fetchWorkflowWeatherJSON(ctx, endpoint, &payload); err != nil {
		return workflowWeatherReport{}, err
	}
	text := fmt.Sprintf("彩云天气：%s，当前 %s，湿度 %s，风速 %s，降水强度 %s",
		caiyunSkyconText(payload.Result.Realtime.Skycon),
		formatWeatherTemperature(payload.Result.Realtime.Temperature),
		formatWeatherPercent(payload.Result.Realtime.Humidity*100),
		formatWeatherSpeed(payload.Result.Realtime.Wind.Speed),
		formatWeatherMillimeter(payload.Result.Realtime.Precipitation.Local.Intensity),
	)
	if strings.TrimSpace(payload.Result.ForecastKeypoint) != "" {
		text += "；" + strings.TrimSpace(payload.Result.ForecastKeypoint)
	}
	return workflowWeatherReport{Provider: "caiyun", Text: text}, nil
}

func fetchWorkflowWeatherJSON(ctx context.Context, endpoint string, out any) error {
	reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "Celestia workflow weather")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("weather request failed with %s", resp.Status)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, workflowWeatherBodyLimit))
	if err != nil {
		return err
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return err
	}
	return nil
}

func workflowWeatherText(point workflowWeatherPoint, reports []workflowWeatherReport) string {
	loc, err := time.LoadLocation(point.Timezone)
	if err != nil {
		loc = time.FixedZone("Asia/Shanghai", 8*60*60)
	}
	lines := []string{fmt.Sprintf("天气查询：%s（%s）", point.Location, time.Now().In(loc).Format("2006-01-02"))}
	for _, report := range reports {
		text := strings.TrimSpace(report.Text)
		if text == "" {
			text = strings.TrimSpace(report.Raw)
		}
		if text != "" {
			lines = append(lines, text)
		}
	}
	return strings.Join(lines, "\n")
}

func validWeatherCoordinate(lat float64, lon float64) bool {
	return lat >= -90 && lat <= 90 && lon >= -180 && lon <= 180 && (lat != 0 || lon != 0)
}
