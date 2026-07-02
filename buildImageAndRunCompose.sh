#!/bin/bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LOCAL_CONFIG_FILE="$ROOT_DIR/.buildImageAndRunCompose.local"

if [ -f "$LOCAL_CONFIG_FILE" ]; then
	# Local-only overrides such as DOCKER_APP_DIR live here and stay out of Git.
	# shellcheck disable=SC1090
	source "$LOCAL_CONFIG_FILE"
fi

IMAGE="${1:-}"
BUILD=1

DOCKER_APP_DIR="${DOCKER_APP_DIR:-}"

if [ "$IMAGE" == "ui" ]; then
	BUILD=0
elif [ "$IMAGE" == "code" ]; then
	BUILD=2
fi


if [ $BUILD -ge 1 ]; then
	cd "$ROOT_DIR/code"
	./build.sh
fi

if [ $BUILD -le 1 ]; then
	cd "$ROOT_DIR/ui"
	./build.sh
fi

if [ -z "$DOCKER_APP_DIR" ]; then
	echo "DOCKER_APP_DIR is not set." >&2
	echo "Set it in the environment or in $LOCAL_CONFIG_FILE." >&2
	exit 1
fi

docker compose -f "$DOCKER_APP_DIR/docker-compose.yml" up -d

echo Ended at `date +%H:%M:%S`
