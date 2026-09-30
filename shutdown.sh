#!/usr/bin/env sh
set -eu
cd "$(dirname "$0")"
for service in api user video favorite comment relation message; do
  if [ -f ".runtime/$service.pid" ]; then
    pid="$(cat ".runtime/$service.pid")"
    case "$pid" in ''|*[!0-9]*) echo "Invalid pid for $service" >&2; exit 1;; esac
    if [ -e "/proc/$pid/exe" ] && [ "$(readlink "/proc/$pid/exe")" = "$(pwd)/bin/$service" ]; then
      kill -TERM "$pid"
    fi
    rm ".runtime/$service.pid"
  fi
done
