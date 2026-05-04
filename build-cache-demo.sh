#!/usr/bin/env bash
set -euo pipefail

# Demo cache: czyści lokalny cache i puszcza build drugi raz.
# Większość kroków powinna być oznaczona CACHED, bo cache leci z registry.

echo "==> Czyszczę lokalny cache buildera..."
docker buildx prune -f --builder zad1builder

echo "==> Drugi build - powinien lecieć z cache..."
docker buildx build \
  --builder zad1builder \
  --platform linux/amd64,linux/arm64 \
  --cache-from type=registry,ref=hubertkolejko/zad1-weather:buildcache \
  --cache-to type=registry,ref=hubertkolejko/zad1-weather:buildcache,mode=max \
  -f Dockerfile.dod \
  -t hubertkolejko/zad1-weather:multiarch \
  --push \
  .
