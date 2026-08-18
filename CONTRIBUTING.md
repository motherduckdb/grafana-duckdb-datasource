# Grafana DuckDB Data Source Plugin - How To Contribute

## Local Development

### Prerequisites

- Node.js (v20+)
- Go (v1.21+), Mage, gcc (building the backend requires CGO)

### Building Locally

#### Backend

To build the backend plugin binary for your platform, run:

```bash
mage -v build:<platform> build:GenerateManifestFile
```
possible values for `<platform>` are: `Linux`, `Windows`, `Darwin`, `DarwinARM64`, `LinuxARM64`, `LinuxARM`.

Note: There's no clear way to cross-compile the plugin since it involves cross-compiling DuckDB via CGO.

#### Frontend

1. Install dependencies

   ```bash
   npm install
   ```

2. Build plugin in development mode and run in watch mode

   ```bash
   npm run dev
   ```

3. Build plugin in production mode

   ```bash
   npm run build
   ```

4. Run the tests (using Jest)

   ```bash
   # Runs the tests and watches for changes, requires git init first
   npm run test

   # Exits after running all the tests
   npm run test:ci
   ```

5. Spin up a Grafana instance and run the plugin inside it (using Docker)

   ```bash
   npm run server
   ```

6. Run the E2E tests (using Cypress)

   ```bash
   # Spins up a Grafana instance first that we tests against
   npm run server

   # Starts the tests
   npm run e2e
   ```

7. Run the linter

   ```bash
   npm run lint

   # or

   npm run lint:fix
   ```

## Build, test and release process

### Testing

The commands above are what CI runs too. `.github/workflows/ci.yml` runs on pushes to `main` and `cloud`, and on pull requests to `main`:

| Layer | What runs |
|-------|-----------|
| Backend | `mage coverage` (Go unit tests), once inside each platform build job |
| Frontend | `npm run typecheck`, `npm run lint`, `npm run test:ci` (Jest) |
| End-to-end | `npm run e2e` (Playwright, specs in `tests/`) against a matrix of Grafana versions resolved by `grafana/plugin-actions/e2e-version`. Each version gets a Grafana container from `docker-compose.yaml` with `dist/` mounted as the plugin directory. On failure, the server log and Playwright report are uploaded as artifacts |

A separate workflow, `.github/workflows/is-compatible.yml`, runs `@grafana/levitate is-compatible` on every pull request to flag use of Grafana APIs that are about to break.

### Building

Because DuckDB is a C++ library, the backend is built with CGO enabled (`Magefile.go`) and cannot be cross-compiled. CI therefore builds each target on its own runner:

| Target | Runner |
|--------|--------|
| `build:Linux` | ubuntu-22.04 |
| `build:LinuxARM64` | ubuntu-22.04, inside an `arm64v8/golang` container under QEMU |
| `build:DarwinARM64` | macos-15 |
| `build:Darwin` | macos-15-intel |
| `build:Windows` | windows-latest |

The `generate-manifest` job collects all five binaries into a single `dist/` and runs `build:GenerateManifestFile`. The `build` job then adds the webpack frontend bundle, signs the plugin if the `GRAFANA_ACCESS_POLICY_TOKEN` secret is set, and packages everything as `motherduck-duckdb-datasource-<version>.zip` alongside a `.sha1` checksum.

### Releasing

A release is triggered by bumping `version` in `package.json` on `main` — there is no tag to push. `src/plugin.json` carries `"version": "%VERSION%"`, substituted from `package.json` at build time, so `package.json` is the single source of truth for the plugin version.

The `check-version-bump` job diffs `package.json` against the previous commit. If the version changed and the ref is `main`, the `deploy` job calls `.github/workflows/release.yml`, which downloads the packaged zip from the same CI run and opens a **draft** GitHub release with generated notes.

Publishing that draft is manual. This is also where the release title gains its DuckDB version, as in `v0.4.5 + duckdb v1.5.4`.

To cut a release:

1. Merge a pull request bumping `version` in `package.json`.
2. Wait for CI to go green across all five platform builds and the Playwright matrix.
3. Open the draft release, edit the title to include the DuckDB version, and publish.

### Bumping DuckDB

`.github/workflows/bump-duckdb.yml` is triggered manually. It takes an optional `duckdb-go` version (defaulting to the latest), runs `go get` and `go mod tidy`, and opens a pull request. That pull request goes through normal CI; the version bump that cuts the release is a separate commit afterwards.