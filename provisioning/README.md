# Provisioning

`docker compose up` starts Grafana with this plugin and [meraki-api-emulator](https://github.com/dubsector/meraki-api-emulator), a local stand-in for the Meraki Dashboard API. These files add a data source that points at the emulator and load the bundled dashboards into a "Cisco Meraki" folder, so every panel shows data without a real API key.

Build the plugin first (`npm run build` and `go tool mage -v`), then open http://localhost:3000.
