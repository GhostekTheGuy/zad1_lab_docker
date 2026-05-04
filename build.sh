#!/usr/bin/env bash
set -euo pipefail

# Build multi-arch z cache registry (mode=max), push na DockerHub.
# Użyj --prune żeby najpierw wyczyścić lokalny cache (demo cache).

if [[ "${1:-}" == "--prune" ]]; then
  echo "==> Czyszczę lokalny cache buildera..."
  docker buildx prune -f --builder zad1builder
fi

docker buildx build \
  --builder zad1builder \
  --platform linux/amd64,linux/arm64 \
  --cache-from type=registry,ref=hubertkolejko/zad1-weather:buildcache \
  --cache-to type=registry,ref=hubertkolejko/zad1-weather:buildcache,mode=max \
  -f Dockerfile.dod \
  -t hubertkolejko/zad1-weather:multiarch \
  -t hubertkolejko/zad1-weather:1.0.0-dod \
  --push \
  .
