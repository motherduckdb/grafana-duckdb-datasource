#!/usr/bin/env bash
# Fails when a Linux binary needs a newer glibc than the README promises.
#
# A binary built on too new a base still has the right ELF architecture, so
# Grafana reports the loader failure only as "Failed to read any lines from
# plugin's stdout".
set -euo pipefail

dist=${1:-dist}
max=${2:-2.35}
status=0

for binary in "$dist"/gpx_duckdb_datasource_linux_*; do
  [ -f "$binary" ] || continue
  needs=$(objdump -T "$binary" | grep -o 'GLIBC_[0-9.]*' | sed 's/GLIBC_//' | sort -V | tail -1)

  if [ "$(printf '%s\n%s\n' "$needs" "$max" | sort -V | tail -1)" = "$max" ]; then
    echo "ok    $(basename "$binary") needs glibc $needs"
  else
    echo "FAIL  $(basename "$binary") needs glibc $needs, newer than $max"
    status=1
  fi
done

exit $status
