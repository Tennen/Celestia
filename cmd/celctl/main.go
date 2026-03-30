package main

import (
	"fmt"
	"os"

	gatewayapi "github.com/chentianyu/celestia/internal/api/gateway"
)

func main() {
	if err := newRootCommand().Execute(); err != nil {
		status := gatewayapi.StatusCode(err)
		if status > 0 {
			fmt.Fprintf(os.Stderr, "%s (status %d)\n", err.Error(), status)
		} else {
			fmt.Fprintln(os.Stderr, err.Error())
		}
		os.Exit(1)
	}
}
