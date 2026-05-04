#!/usr/bin/env bash
set -euo pipefail

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
