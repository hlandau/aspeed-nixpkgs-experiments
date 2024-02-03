#!/usr/bin/env bash
set -eo pipefail
(cd service; GOARCH=arm GOOS=linux go build .;)
nom-build -A deviceImage
