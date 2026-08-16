# Populates ./dist with a published release, so Grafana can be run locally
# without building the plugin. Used by `make dist VERSION=v0.4.5`.
#
#   docker build -f dist.Dockerfile --output dist .                          # latest
#   docker build -f dist.Dockerfile --build-arg VERSION=v0.4.5 --output dist .

FROM alpine:3 AS fetch
ARG VERSION=latest
ARG REPO=motherduckdb/grafana-duckdb-datasource

RUN apk add --no-cache curl jq

WORKDIR /out
RUN set -eu; \
    tag="$VERSION"; \
    if [ "$tag" = "latest" ]; then \
      tag=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | jq -er .tag_name); \
    fi; \
    # Release assets are named without the tag's leading "v".
    curl -fsSL -o plugin.zip \
      "https://github.com/${REPO}/releases/download/${tag}/motherduck-duckdb-datasource-${tag#v}.zip"; \
    unzip -q plugin.zip; \
    mv motherduck-duckdb-datasource/* .; \
    rm -rf motherduck-duckdb-datasource plugin.zip

FROM scratch
COPY --from=fetch /out /
