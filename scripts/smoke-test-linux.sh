#!/usr/bin/env bash
# Runs each Linux binary inside the oldest supported Grafana ubuntu image, so
# incompatibilities (like the glibc floor of issue #76) fail here instead of as
# "Failed to read any lines from plugin's stdout" in a user's Grafana.
#
# A healthy plugin binary reaches go-plugin's "This binary is a plugin" notice;
# a binary the image cannot load dies in the dynamic loader before printing it.
#
# The README promises "glibc 2.35 or later" because this image is Ubuntu 22.04.
# When grafanaDependency moves to a Grafana on a newer Ubuntu base, this test's
# floor rises with it — re-check the README line then.
set -euo pipefail

dist=${1:-dist}
grafana_version=$(sed -n 's/.*"grafanaDependency": *">=\([0-9.]*\)".*/\1/p' src/plugin.json)
image="grafana/grafana:${grafana_version}-ubuntu"
status=0

for binary in "$dist"/gpx_duckdb_datasource_linux_*; do
  [ -f "$binary" ] || continue
  arch=${binary##*_}
  output=$(docker run --rm --platform "linux/$arch" \
             -v "$(cd "$dist" && pwd)":/plugin:ro \
             --entrypoint "/plugin/$(basename "$binary")" "$image" 2>&1 || true)
  if grep -q "This binary is a plugin" <<< "$output"; then
    echo "ok    $(basename "$binary") runs on $image"
  else
    echo "FAIL  $(basename "$binary") does not run on $image:"
    echo "$output" | head -5 | sed 's/^/      /'
    status=1
  fi
done

exit $status
