package workflow

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/chentianyu/celestia/internal/models"
)

type workflowWeatherTransport struct {
	t        *testing.T
	requests []string
}

func (t *workflowWeatherTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.requests = append(t.requests, req.URL.Host+req.URL.Path)
	switch req.URL.Host {
	case "api.open-meteo.com":
		return response(http.StatusOK, "application/json", `{
			"current":{"temperature_2m":24.4,"relative_humidity_2m":62,"apparent_temperature":25.1,"precipitation":0,"weather_code":2,"wind_speed_10m":11.5},
			"daily":{"weather_code":[2],"temperature_2m_max":[29.2],"temperature_2m_min":[21.4],"precipitation_sum":[0.2],"precipitation_probability_max":[30]}
		}`), nil
	case "wttr.in":
		return response(http.StatusOK, "application/json", `{
			"current_condition":[{"temp_C":"24","FeelsLikeC":"25","humidity":"64","windspeedKmph":"10","precipMM":"0.0","lang_zh":[{"value":"局部多云"}]}],
			"weather":[{"maxtempC":"29","mintempC":"21","hourly":[{"chanceofrain":"10"},{"chanceofrain":"35"}]}]
		}`), nil
	case "www.weather.com.cn":
		return response(http.StatusOK, "application/json", `{"weatherinfo":{"city":"上海","cityid":"101020100","temp1":"21℃","temp2":"29℃","weather":"多云","ptime":"08:00"}}`), nil
	default:
		t.t.Fatalf("unexpected weather request host %q", req.URL.Host)
		return nil, nil
	}
}

func TestRunWorkflowWeatherAggregatesProviderText(t *testing.T) {
	ctx := context.Background()
	svc, _ := newWorkflowPersistenceTestService(t)
	workflow := models.AgentWorkflow{
		ID:   "workflow-weather",
		Name: "Weather Workflow",
		Nodes: []models.AgentWorkflowNode{{
			ID:       "weather-main",
			Type:     workflowNodeTypeWeather,
			Label:    "Weather",
			Position: models.AgentNodePoint{X: 80, Y: 80},
			Data: map[string]any{
				"location":            "上海",
				"latitude":            31.2304,
				"longitude":           121.4737,
				"timezone":            "Asia/Shanghai",
				"providers":           []string{"open_meteo", "wttr_in", "weather_com_cn"},
				"weather_com_city_id": "101020100",
			},
		}},
	}
	if _, err := svc.SaveWorkflow(ctx, models.AgentWorkflowSnapshot{
		ActiveWorkflowID: workflow.ID,
		Workflows:        []models.AgentWorkflow{workflow},
	}); err != nil {
		t.Fatalf("SaveWorkflow() error = %v", err)
	}

	transport := &workflowWeatherTransport{t: t}
	previousTransport := http.DefaultClient.Transport
	http.DefaultClient.Transport = transport
	defer func() {
		http.DefaultClient.Transport = previousTransport
	}()

	run, err := svc.RunWorkflow(ctx, workflow.ID)
	if err != nil {
		t.Fatalf("RunWorkflow() error = %v", err)
	}
	if run.Status != "succeeded" {
		t.Fatalf("run status = %q, want succeeded", run.Status)
	}
	for _, want := range []string{"天气查询：上海", "Open-Meteo：多云", "wttr.in：局部多云", "中国天气：上海 多云"} {
		if !strings.Contains(run.OutputText, want) {
			t.Fatalf("weather output missing %q in:\n%s", want, run.OutputText)
		}
	}
	if len(transport.requests) != 3 {
		t.Fatalf("weather requests = %d, want 3 (%v)", len(transport.requests), transport.requests)
	}
}
