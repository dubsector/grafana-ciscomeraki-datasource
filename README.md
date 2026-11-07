# Cisco Meraki data source for Grafana

Monitor Cisco Meraki networks in Grafana: device health, network and security events, wireless and switch stats, uplinks, VPN and MT sensors, all from the [Meraki Dashboard API v1](https://developer.cisco.com/meraki/api-v1/). Six ready-made dashboards follow the order of the Meraki dashboard menu, and every query works with Grafana alerting.

## Features

| Query Type | Description |
|---|---|
| Device Availabilities | Live or historical device online/offline status |
| Device Statuses | Live device status with model, LAN IP and gateway |
| Network Events | DHCP, 802.11, VPN, and other event logs |
| Security Events | IDS/IPS and appliance security alerts |
| Network Clients | Clients currently seen on a network |
| Device Clients | Clients seen on a specific device |
| Wireless Latency Stats | Per-traffic-class latency for a network |
| Wireless Connection Stats | Auth, DHCP, DNS success counts |
| Wireless Client Count | Historical client count time-series |
| Switch Port Statuses | Live port status per switch device |
| Appliance Uplink Statuses | Live WAN uplink status across the org |
| Appliance LAN Ports | MX LAN port settings (VLAN, access or trunk) |
| VPN Stats | Site-to-site VPN latency, loss, and jitter |
| Sensor Readings | Newest MT sensor reading per metric, one row each |
| Sensor Readings History | MT sensor readings as one time series per sensor and metric |

- Template variable support: networks (optionally only those with given `productTypes`), devices (optionally filtered by `networkId` or `productType`), and `eventProductTypes` for a network's event log product types
- 6 pre-built dashboards, in the order of the Meraki dashboard menu: Network Events, Security & SD-WAN, Switching, Wireless, Sensors, Device Health
- Sensor queries can be narrowed to a network, a sensor and a metric. Each reading has one numeric value: door open, water present and similar switches are 1 or 0, and temperature is in °C unless the Fahrenheit switch is on. Button presses aren't included
- API key stored securely in Grafana's encrypted config
- Automatic pagination and retry with exponential backoff
- Time ranges are trimmed to each Meraki endpoint's limits (for example 7 days per request for wireless stats), with a notice on the panel when that happens
- 60-second TTL cache for network/device dropdown lists


## Requirements

- Grafana 11.5.0 or later
- A Cisco Meraki Dashboard API key with at least read-only organization access

## Installation

### Grafana Cloud

Install **Cisco Meraki** from the plugin catalog under **Connections > Add new connection**.

### Self-hosted Grafana

1. Download `dubsector-ciscomeraki-datasource.zip` from the latest [release](https://github.com/dubsector/grafana-ciscomeraki-datasource/releases).
2. Extract it into your Grafana plugins directory, for example `/var/lib/grafana/plugins/` on Linux.
3. Builds from GitHub are not signed by Grafana, so allow this plugin in `grafana.ini`:
   ```ini
   [plugins]
   allow_loading_unsigned_plugins = dubsector-ciscomeraki-datasource
   ```
4. Restart Grafana.

## Configuration

In Grafana, add a new datasource of type **Cisco Meraki** and fill in:

| Field | Description |
|---|---|
| Base URL | Meraki API base URL. Defaults to `https://api.meraki.com/api/v1`. Change for India (`api.in.meraki.com`) or FedRAMP (`api.gov.meraki.com`) regions. |
| Organization ID | Your org ID, found under **Organization → Settings** in the Meraki dashboard. |
| API Key | Your Dashboard API key. Generate one under **My Profile → API access**. |

Click **Save & Test**. A green check confirms connectivity.

To load the dashboards, open the data source and go to the **Dashboards** tab, then click **Import** next to each one.

## Support

Report bugs and ask questions in [GitHub issues](https://github.com/dubsector/grafana-ciscomeraki-datasource/issues).

## Contributing

Building, testing and running the plugin locally are covered in [DEVELOPMENT.md](https://github.com/dubsector/grafana-ciscomeraki-datasource/blob/main/DEVELOPMENT.md).

## License

[Elastic License 2.0](https://github.com/dubsector/grafana-ciscomeraki-datasource/blob/main/LICENSE). You can use the plugin for free in your own Grafana, but you may not republish it or offer it as a competing product. Versions 1.1.0 and earlier were released under the Apache License 2.0.
