#!/usr/bin/env sh
set -eu
cd "$(dirname "$0")/.."
mkdir -p .tools
GOBIN="$(pwd)/.tools" go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.11
GOBIN="$(pwd)/.tools" go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5.1
for proto in api/*/v1/*.proto; do
  protoc --proto_path=. --plugin="protoc-gen-go=$(pwd)/.tools/protoc-gen-go" --plugin="protoc-gen-go-grpc=$(pwd)/.tools/protoc-gen-go-grpc" --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative "$proto"
done
