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

A release is cut manually with the **Release** workflow. Do not create a tag or release directly from the GitHub Releases page. `src/plugin.json` carries `"version": "%VERSION%"`, substituted from `package.json` at build time, so `package.json` is the single source of truth for the plugin version.

The workflow accepts a commit SHA, defaulting to the latest commit on `main`. It requires that exact commit to have a successful `main` CI run, reads its version from `package.json`, downloads the packaged plugin from that run, and publishes a GitHub release with tag `v<version>` and generated notes. A version suffix such as `-rc1` creates a pre-release.

The optional `dry_run` input performs every validation and downloads the artifact without creating the tag or release. The workflow fails if the selected commit has no successful `main` build, its artifact is unavailable, or its version tag already exists.

To cut a release:

1. Merge a pull request that sets the desired version in `package.json`.
2. Wait for CI to go green across all five platform builds and the Playwright matrix.
3. Open **Actions → Release → Run workflow**, select the commit to release (or leave `main`), and run it. Use `dry_run` first when validating a release.

The same workflow can be started with the GitHub CLI:

```bash
gh workflow run release.yml -f sha=<commit>
```

### Bumping DuckDB

`.github/workflows/bump-duckdb.yml` is triggered manually. It takes an optional `duckdb-go` version (defaulting to the latest), runs `go get` and `go mod tidy`, and opens a pull request. That pull request goes through normal CI; the version bump that cuts the release is a separate commit afterwards.