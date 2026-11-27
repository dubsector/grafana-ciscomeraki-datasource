# Changelog

## 2.0.0 (unreleased)

### Breaking changes

- The plugin is now licensed under the Elastic License 2.0. You can still run it for free in your own Grafana, but you may not republish it or offer it as a competing product. Versions 1.1.0 and earlier stay under the Apache License 2.0.

### Features

- MT sensor readings: the newest reading per metric, and readings history as one time series per sensor and metric.
- Sensors dashboard.
- SD-WAN health: uplink loss and latency across the org, loss, latency and jitter history for one uplink, and uplink usage in bits per second.
- Alerts: open organization alerts, most severe first.
- Multiple organizations: every query can pick any organization the API key can see, and each dashboard has an Organization picker. The data source's organization stays the default.
- Annotations: network events, security events and alerts can mark charts. Resolved alerts show as a region from start to resolution, and the Overview dashboard marks alerts on its uplink charts.
- Alerts can include resolved ones that started in the time range.
- Top Applications and Top Clients by data used in the time range. The Overview dashboard shows both.
- License Overview: licensing status and expiry for one organization or every organization, soonest expiry first, with both co-termination and per-device licensing. Organizations the API key can't read get a row saying why.
- Organizations dashboard: licensing and expiry for every organization, for MSPs.
- Overview dashboard with device counts, open alerts, the worst WAN uplinks and uplink detail for a chosen network.

### Fixes

- The India and US Government API hosts were wrong. The data source settings now offer every Meraki region by name: Global, Canada, China, India and US Government.

### Changes

- Meraki error messages are shown as plain text instead of raw JSON.
- Save & test names the connected organization, and says which organizations the API key can see when the configured one isn't among them.
- Requests are paced to Meraki's limit of 10 a second per organization, so a busy dashboard no longer runs into rate limit errors. Panels asking for the same data within 30 seconds share one response.
- Built with Grafana's standard `create-plugin` tooling. The release zip now includes the README, LICENSE and CHANGELOG.
- The end-to-end test runs against the emulator's release image.

## 1.1.0 (2026-09-30)

### Features

- Device tables show model and IP. The Switching dashboard lists switches.
- Dashboard overhaul with network filters and stable columns.

### Fixes

- Organization IDs are handled as numbers.

### Changes

- End-to-end test of every API call against meraki-api-emulator.

## 1.0.7 (2026-09-29)

### Fixes

- Network events stop paging at the end of the time range.
- `rel=next` Link headers without quotes are followed.
- Endpoints that did not match the Meraki API spec are corrected.

### Changes

- Frontend and backend lint in CI.

## 1.0.6 (2026-09-29)

- Dependency updates.

## 1.0.5 (2026-09-10)

### Changes

- Moved to Grafana 13.2 packages and React 19.

## 1.0.4 (2026-08-08)

### Fixes

- Raised the dompurify override to 3.4.13 for GHSA-55q2-fjhq-7xh7.

## 1.0.3 (2026-08-03)

### Changes

- Separate build and release workflows, with npm and Go caching.

## 1.0.2 (2026-07-16)

### Changes

- Hardened GitHub Actions and added dependency review.

## 1.0.1 (2026-07-15)

### Changes

- Automated dependency updates.

## 1.0.0 (2026-07-15)

- Initial release.
