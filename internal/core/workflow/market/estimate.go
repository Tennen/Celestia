package market

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

func FetchEastmoneyEstimate(ctx context.Context, code string, timeoutMS int) (Estimate, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return Estimate{}, errors.New("fund code is required")
	}
	reqCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMS)*time.Millisecond)
	defer cancel()
	endpoint := "https://fundgz.1234567.com.cn/js/" + code + ".js?rt=" + fmt.Sprintf("%d", time.Now().UnixMilli())
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, endpoint, nil)
	if err != nil {
		return Estimate{}, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return Estimate{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Estimate{}, fmt.Errorf("eastmoney HTTP %s", resp.Status)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return Estimate{}, err
	}
	matches := regexp.MustCompile(`jsonpgz\((.*)\);?`).FindSubmatch(raw)
	if len(matches) < 2 {
		return Estimate{}, errors.New("eastmoney response did not include jsonpgz payload")
	}
	var payload map[string]any
	if err := json.Unmarshal(matches[1], &payload); err != nil {
		return Estimate{}, err
	}
	return Estimate{
		EstimateNAV: parseFloat(payload["gsz"]),
		ChangePct:   parseFloat(payload["gszzl"]),
		AsOf:        firstNonEmpty(stringFrom(payload["gztime"]), stringFrom(payload["jzrq"])),
	}, nil
}
