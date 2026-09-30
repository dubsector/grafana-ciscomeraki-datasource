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
| Wireless Latency Stats | Per-traffic-class latency for a network |
| Wireless Connection Stats | Auth, DHCP, DNS success counts |
| Wireless Client Count | Historical client count time-series |
| Switch Port Statuses | Live port status per switch device |
| Appliance Uplink Statuses | Live WAN uplink status across the org |
| VPN Stats | Site-to-site VPN latency, loss, and jitter |

- Template variable support: networks (optionally only those with given `productTypes`), devices (optionally filtered by `networkId` or `productType`), and `eventProductTypes` for a network's event log product types
- 5 pre-built dashboards, in the order of the Meraki dashboard menu: Network Events, Security & SD-WAN, Switching, Wireless, Device Health
- API key stored securely in Grafana's encrypted config
- Automatic pagination and retry with exponential backoff
- Time ranges are trimmed to each Meraki endpoint's limits (for example 7 days per request for wireless stats), with a notice on the panel when that happens
- 60-second TTL cache for network/device dropdown lists

## Requirements

- Grafana >= 11.5.0
- Cisco Meraki Dashboard API key with at least read-only org access

## Installation

### Option A: Download from GitHub Releases (easiest for testing)

1. Go to [Releases](https://github.com/dubsector/grafana-ciscomeraki-datasource/releases) and download the latest `dubsector-ciscomeraki-datasource.zip`
2. Extract it to your Grafana plugins directory:
   - Linux/Mac: `/var/lib/grafana/plugins/`
   - Windows: `C:\Program Files\GrafanaLabs\grafana\data\plugins\`
3. Allow the unsigned plugin in `grafana.ini`:
   ```ini
   [plugins]
   allow_loading_unsigned_plugins = dubsector-ciscomeraki-datasource
   ```
4. Restart Grafana

### Option B: Docker (for local testing)

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

Click **Save & Test**. A green check confirms connectivity.

## Building from Source

You need **Node.js 20.19+** and **Go 1.26+**. [Mage](https://magefile.org/) is pinned as a Go tool in `go.mod`, so there is nothing extra to install.

```bash
# Install frontend dependencies
npm ci

# Build frontend
npm run build

# Build backend binaries into dist/
go tool mage -v

# Verify dist/ has everything
ls dist/
```

The `dist/` directory is the compiled plugin. Mount it in Grafana or run `docker-compose up`.

### Development mode

```bash
npm run dev   # webpack watch mode, rebuilds on TypeScript changes
```

### Checks

These are the same checks CI runs:

```bash
npm run typecheck
npm run lint        # oxlint
npm test
go test ./pkg/...   # includes a check of every API call against the Meraki OpenAPI spec
golangci-lint run   # config in .golangci.yml
```

### End-to-end test

`pkg/meraki/e2e_test.go` runs every API call against [meraki-api-emulator](https://github.com/dubsector/meraki-api-emulator), once with the normal page size and once split into about ten pages, and checks that both return the same data. Start the emulator with a frozen clock, then point the test at it:

```bash
node bin/meraki-api-emulator.js --now 2026-09-15T12:00:00Z --rate-limit 0   # in the emulator repo
MERAKI_E2E_URL=http://127.0.0.1:8765/api/v1 MERAKI_E2E_NOW=2026-09-15T12:00:00Z \
  go test -tags e2e ./pkg/meraki -run TestE2E
```

## CI/CD

`.github/workflows/ci.yml` runs on every pull request, every push to `master` and once a week: type check, lint, tests, the end-to-end test against the emulator's latest `main`, frontend and backend builds, then packages the plugin as a downloadable zip artifact.

Pushing a tag like `v1.0.7` runs `.github/workflows/release.yml`, which builds the plugin and publishes a GitHub Release with the zip attached. The tag must match the version in both `package.json` and `src/plugin.json`.

## License

Apache License 2.0. See [LICENSE](LICENSE).
