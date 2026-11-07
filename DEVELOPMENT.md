# Development

## Building from source

You need **Node.js 22.23+** and **Go 1.26+**. [Mage](https://magefile.org/) is pinned as a Go tool in `go.mod`, so there is nothing extra to install.

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

The `dist/` directory is the compiled plugin. Run `docker compose up` to start Grafana with the plugin and the emulator (see [provisioning/README.md](provisioning/README.md)).

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
docker run -d -p 8765:8765 ghcr.io/dubsector/meraki-api-emulator --now 2026-09-15T12:00:00Z --rate-limit 0
MERAKI_E2E_URL=http://127.0.0.1:8765/api/v1 MERAKI_E2E_NOW=2026-09-15T12:00:00Z \
  go test -tags e2e ./pkg/meraki -run TestE2E
```

## CI/CD

`.github/workflows/ci.yml` runs on every pull request, every push to `main` and once a week: type check, lint, tests, the end-to-end test against the emulator's latest release image, frontend and backend builds, then packages the plugin as a downloadable zip artifact.

Pushing a tag like `v2.0.0` runs `.github/workflows/release.yml`, which builds the plugin and publishes a GitHub Release with the zip attached. The tag must match the version in both `package.json` and `src/plugin.json`.
