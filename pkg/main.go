package main

import (
	"os"

	"github.com/grafana/grafana-plugin-sdk-go/backend/datasource"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/dubsector/meraki-datasource/pkg/plugin"
)

func main() {
	if err := datasource.Manage(
		"dubsector-ciscomeraki-datasource",
		plugin.NewDS,
		datasource.ManageOpts{},
	); err != nil {
		log.DefaultLogger.Error("plugin stopped", "err", err)
		os.Exit(1)
	}
}
