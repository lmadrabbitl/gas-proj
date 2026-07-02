#!/bin/bash

set -euo pipefail

ensure_image() {
  local image="$1"

  if docker image inspect "$image" >/dev/null 2>&1; then
    echo "Using local base image: $image"
    return
  fi

  echo "Base image not found locally, pulling: $image"
  docker pull "$image"
}

ensure_image "golang:1.25-alpine"
ensure_image "alpine:3.22"

docker build --pull=false -t expensetracker-api:latest .
