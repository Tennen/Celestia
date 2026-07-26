package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestHandleProxyAppChatSendUsesFixedWeComEndpoint(t *testing.T) {
	previousTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = previousTransport
	})

	var upstreamPath string
	var upstreamBody string
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		upstreamPath = req.URL.RequestURI()
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read upstream body: %v", err)
		}
		upstreamBody = string(body)
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"errcode":0,"errmsg":"ok"}`)),
			Header:     make(http.Header),
		}, nil
	})

	request := httptest.NewRequest(http.MethodPost, "/proxy/appchat/send", strings.NewReader(
		`{"access_token":"token with space","message":{"chatid":"chat-ops","msgtype":"text","text":{"content":"alarm"}}}`,
	))
	recorder := httptest.NewRecorder()
	handleProxyAppChatSend(recorder, request, bridgeConfig{})

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", recorder.Code, recorder.Body.String())
	}
	if upstreamPath != "/cgi-bin/appchat/send?access_token=token+with+space" {
		t.Fatalf("upstream path = %q", upstreamPath)
	}
	if !strings.Contains(upstreamBody, `"chatid":"chat-ops"`) {
		t.Fatalf("upstream body = %s", upstreamBody)
	}
}
