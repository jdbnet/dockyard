#!/bin/bash

cd ui && npm install && npm run build && cd ..

go build -o build/dockyard-amd64 ./cmd/dockyard

CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o build/dockyard-arm64 ./cmd/dockyard