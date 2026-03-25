package main

import (
	"log"

	"github.com/chentianyu/celestia/internal/pluginruntime"
	"github.com/chentianyu/celestia/plugins/petkit/internal/app"
)

func main() {
	if err := pluginruntime.Serve(app.New()); err != nil {
		log.Fatal(err)
	}
}

