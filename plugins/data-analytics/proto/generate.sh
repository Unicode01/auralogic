#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")"

protoc \
  --go_out=../pb --go_opt=paths=source_relative \
  --go-grpc_out=../pb --go-grpc_opt=paths=source_relative \
  plugin.proto

echo "Protobuf code generated in plugins/data-analytics/pb"
