#!/usr/bin/env sh
set -eu
cd "$(dirname "$0")"
mkdir -p bin .runtime
for service in user video favorite comment relation message api; do
  if [ -f ".runtime/$service.pid" ]; then
    pid="$(cat ".runtime/$service.pid")"
    case "$pid" in ''|*[!0-9]*) echo "Invalid pid for $service" >&2; exit 1;; esac
    if [ -e "/proc/$pid/exe" ] && [ "$(readlink "/proc/$pid/exe")" = "$(pwd)/bin/$service" ]; then
      echo "$service is already running" >&2
      exit 1
    fi
  fi
  go build -o "bin/$service" "./cmd/$service"
  "./bin/$service" -config "config/kratos.yml" >".runtime/$service.log" 2>&1 &
  echo "$!" >".runtime/$service.pid"
done
