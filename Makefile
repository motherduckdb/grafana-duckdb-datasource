# Set VERSION to run against a published release instead of a local build,
# e.g. `make dev VERSION=v0.4.5`.
VERSION ?=
GO_IMAGE ?= golang:1.24-bookworm

.PHONY: dev dist test test-local clean

# Grafana with the plugin on http://localhost:3000
dev: dist
	docker compose up

# Populate ./dist, either from a release or by building this working tree.
dist:
ifeq ($(VERSION),)
	npm ci
	npm run build
	docker run --rm -v "$(PWD)":/repo -w /repo -e GOFLAGS=-buildvcs=false $(GO_IMAGE) \
		sh -c 'go install github.com/magefile/mage@latest && \
			case $$(uname -m) in aarch64) target=build:LinuxARM64;; *) target=build:Linux;; esac && \
			mage -v $$target build:GenerateManifestFile'
else
	docker build -f dist.Dockerfile --build-arg VERSION=$(VERSION) --output dist .
endif

# End-to-end tests, needing nothing but Docker.
test: dist
	docker compose run --rm e2e; status=$$?; docker compose down; exit $$status

# The same tests using a local Node.js, as CI runs them.
test-local: dist
	npm ci
	npx playwright install chromium --with-deps
	docker compose up -d
	until curl -sf http://localhost:3000/api/health >/dev/null; do sleep 1; done
	npm run e2e; status=$$?; docker compose down; exit $$status

clean:
	rm -rf dist node_modules playwright-report test-results
