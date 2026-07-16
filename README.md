# Cisco Meraki Datasource for Grafana

A Grafana backend datasource plugin for the [Cisco Meraki Dashboard API v1](https://developer.cisco.com/meraki/api-v1/).

## Features

| Query Type | Description |
|---|---|
| Device Availabilities | Live or historical device online/offline status |
| Network Events | DHCP, 802.11, VPN, and other event logs |
| Security Events | IDS/IPS and appliance security alerts |
| Network Clients | Clients currently seen on a network |
| Device Clients | Clients seen on a specific device |
| Wireless Latency Stats | Per-traffic-class latency by network or org |
| Wireless Connection Stats | Auth, DHCP, DNS success counts |
| Wireless Client Count | Historical client count time-series |
| Switch Port Statuses | Live port status per switch device |
| Appliance Uplink Statuses | Live WAN uplink status across the org |
| VPN Stats | Site-to-site VPN latency, loss, and jitter |

- Template variable support (`networks` and `devices` queries)
- 4 pre-built dashboards (Device Health, Network Events, Wireless, Infrastructure)
- API key stored securely in Grafana's encrypted config
- Automatic pagination and retry with exponential backoff
- 60-second TTL cache for network/device dropdown lists

## Requirements

- Grafana >= 10.4.0
- Cisco Meraki Dashboard API key with at least read-only org access

## Installation

### Option A — Download from GitHub Releases (easiest for testing)

1. Go to [Releases](https://github.com/dubsector/meraki-datasource/releases) and download the latest `dubsector-ciscomeraki-datasource.zip`
2. Extract it to your Grafana plugins directory:
   - Linux/Mac: `/var/lib/grafana/plugins/`
   - Windows: `C:\Program Files\GrafanaLabs\grafana\data\plugins\`
3. Allow the unsigned plugin in `grafana.ini`:
   ```ini
   [plugins]
   allow_loading_unsigned_plugins = dubsector-ciscomeraki-datasource
   ```
4. Restart Grafana

### Option B — Docker (for local testing)

After building (see below), run:

```bash
docker-compose up
```

Grafana will start at http://localhost:3000 with the plugin pre-loaded (no login required).

## Configuration

In Grafana, add a new datasource of type **Cisco Meraki** and fill in:

| Field | Description |
|---|---|
| Base URL | Meraki API base URL. Defaults to `https://api.meraki.com/api/v1`. Change for India (`api.in.meraki.com`) or FedRAMP (`api.gov.meraki.com`) regions. |
| Organization ID | Your org ID, found under **Organization → Settings** in the Meraki dashboard. |
| API Key | Your Dashboard API key. Generate one under **My Profile → API access**. |

Click **Save & Test** — a green check confirms connectivity.

## Building from Source

You need: **Node.js 20+**, **Go 1.21+**, and **[mage](https://magefile.org/)**.

```bash
# Install mage (one-time)
go install github.com/magefile/mage@latest

# Install frontend dependencies
npm install

# Build frontend
npm run build

# Generate go.sum and build backend binaries into dist/
go mod tidy
mage -v

# Verify dist/ has everything
ls dist/
```

The `dist/` directory is the compiled plugin. Mount it in Grafana or run `docker-compose up`.

### Development mode

```bash
npm run dev   # webpack watch mode — rebuilds on TypeScript changes
```

## CI/CD

The included GitHub Actions workflow (`.github/workflows/ci.yml`) builds the plugin on every push to `main` and creates a downloadable zip artifact. When you push a tag like `v1.0.0`, it automatically publishes a GitHub Release with the zip attached.

## License

Apache License 2.0 — see [LICENSE](LICENSE).
