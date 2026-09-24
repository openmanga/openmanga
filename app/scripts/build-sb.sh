#!/bin/sh
# Builds the sb CLI from ../cli into the Tauri sidecar slot.
set -e
cd "$(dirname "$0")/../../cli"
go build -trimpath -ldflags="-s -w" -o ../app/src-tauri/binaries/sb-aarch64-apple-darwin .
