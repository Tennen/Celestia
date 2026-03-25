package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

func main() {
	path := "/api/v1/dashboard"
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "plugins":
			path = "/api/v1/plugins"
		case "devices":
			path = "/api/v1/devices"
		case "events":
			path = "/api/v1/events"
		}
	}
	baseURL := os.Getenv("CELESTIA_URL")
	if baseURL == "" {
		baseURL = "http://127.0.0.1:8080"
	}
	resp, err := http.Get(baseURL + path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	var payload any
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	pretty, _ := json.MarshalIndent(payload, "", "  ")
	fmt.Println(string(pretty))
}

