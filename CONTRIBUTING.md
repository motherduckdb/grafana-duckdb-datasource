# Grafana DuckDB Data Source Plugin - How To Contribute

no op change, please don't merge. Want to test grafana latest with no changes

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
