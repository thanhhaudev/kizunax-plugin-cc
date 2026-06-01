#!/usr/bin/env bash
set -e
cd "$(dirname "$0")/.."
mkdir -p plugins/kizunax/bin
go build -trimpath -ldflags="-s -w" -o plugins/kizunax/bin/kizunax ./cmd/kizunax
echo "Built: plugins/kizunax/bin/kizunax"
