# Zadanie 1 - część dodatkowa, wariant 3 (max. +80%)

Autor: Hubert Kolejko
GitHub: https://github.com/GhostekTheGuy/zad1_lab_docker
DockerHub: https://hub.docker.com/r/hubertkolejko/zad1-weather

Wybrałem wariant 3: multi-arch (linux/amd64 + linux/arm64) na builderze `docker-container`, rozszerzony frontend BuildKit, `mount=type=ssh` / `mount=type=secret` do pobrania kodu z publicznego repo GitHub w trakcie buildu, cache registry w trybie `max`.

## Plik Dockerfile

Osobny plik `Dockerfile.dod` z `# syntax=docker/dockerfile:1.7-labs` (rozszerzony frontend). W stage `source` użyłem od razu obu typów mount-ów, bo chciałem żeby działało zarówno z agentem SSH jak i z PAT-em:

```dockerfile
RUN --mount=type=ssh \
    --mount=type=secret,id=gh_token,required=false \
    set -eu; \
    mkdir -p /root/.ssh && ssh-keyscan -t rsa,ed25519 github.com >> /root/.ssh/known_hosts 2>/dev/null; \
    SSH_REPO=$(printf '%s' "$REPO_URL" | sed 's|https://github.com/|git@github.com:|'); \
    if [ -n "${SSH_AUTH_SOCK:-}" ] && git clone --depth 1 --branch "$REPO_REF" "$SSH_REPO" /src 2>/dev/null; then \
        echo "[source] kod pobrany przez SSH (mount=type=ssh)"; \
    elif [ -s /run/secrets/gh_token ] && \
         git clone --depth 1 --branch "$REPO_REF" \
            "https://$(cat /run/secrets/gh_token)@${REPO_URL#https://}" /src 2>/dev/null; then \
        echo "[source] kod pobrany przez HTTPS+token (mount=type=secret)"; \
    else \
        git clone --depth 1 --branch "$REPO_REF" "$REPO_URL" /src; \
        echo "[source] kod pobrany anonimowo (repo publiczne)"; \
    fi
```

Logika jest taka: najpierw próbuję klonować przez SSH (jeśli host przekazał `--ssh default` i agent ma klucz). Jeśli nie, próbuję przez HTTPS z tokenem z `--secret id=gh_token`. Jako ostatnia deska ratunku - anonimowy klon HTTPS, bo repo jest publiczne. Dzięki temu obraz da się zbudować w różnych warunkach a Dockerfile pokazuje obie funkcjonalności (mount ssh i mount secret) o które chodzi w treści zadania.

Reszta pliku (stage `builder` na `golang:1.26-alpine` i runtime na `scratch`) wygląda jak w wersji obowiązkowej.

## 1. Builder docker-container

Konfiguracja BuildKit (plik `/tmp/buildkitd.toml`) - potrzebna była przy testach na lokalnym registry, dla DockerHub można pominąć:

```toml
[registry."localhost:5050"]
  http = true
  insecure = true
```

Builder:

```bash
docker buildx create --name zad1builder \
  --driver docker-container \
  --driver-opt network=host \
  --buildkitd-config /tmp/buildkitd.toml \
  --bootstrap --use
```

```bash
docker buildx inspect zad1builder
```

```
Name:          zad1builder
Driver:        docker-container
Driver Options: network="host"
BuildKit version: v0.29.0
Platforms:     linux/arm64, linux/amd64, linux/amd64/v2, linux/riscv64, linux/ppc64le, linux/s390x, linux/386, linux/arm/v7, linux/arm/v6
Status:        running
```

Driver `docker-container` jest tu wymagany, bo zwykły `docker` driver nie obsługuje builda multi-platform.

## 2. Build multi-arch z mount=ssh + cache registry max

```bash
docker buildx build \
  --builder zad1builder \
  --platform linux/amd64,linux/arm64 \
  --ssh default \
  --cache-to type=registry,ref=hubertkolejko/zad1-weather:buildcache,mode=max \
  --cache-from type=registry,ref=hubertkolejko/zad1-weather:buildcache \
  -f Dockerfile.dod \
  -t hubertkolejko/zad1-weather:multiarch \
  -t hubertkolejko/zad1-weather:1.0.0-dod \
  --push \
  .
```

Jeśli zamiast SSH chcemy użyć tokena, dochodzi `--secret id=gh_token,src=$HOME/.gh_token`.

## 3. Manifest zawiera obie platformy

```bash
docker buildx imagetools inspect hubertkolejko/zad1-weather:multiarch
```

```
Name:      docker.io/hubertkolejko/zad1-weather:multiarch
MediaType: application/vnd.oci.image.index.v1+json
Digest:    sha256:96020f58651be7c8d78383ecd5a74de4c6339751d8f8ac34e22d9e6f24a1590d

Manifests:
  Name:        ...@sha256:d54644705bfd2679...
  Platform:    linux/amd64

  Name:        ...@sha256:b208b96c17b2ab6e...
  Platform:    linux/arm64

  Name:        ...@sha256:455495901fc17b4a...
  Platform:    unknown/unknown
  Annotations:
    vnd.docker.reference.type:   attestation-manifest
    vnd.docker.reference.digest: sha256:d54644705bfd2679...

  Name:        ...@sha256:376c3089dd8f2a3e...
  Platform:    unknown/unknown
  Annotations:
    vnd.docker.reference.type:   attestation-manifest
    vnd.docker.reference.digest: sha256:b208b96c17b2ab6e...
```

W manifeście są obie wymagane platformy (`linux/amd64`, `linux/arm64`) plus dwa attestation manifesty z provenance, które buildx dokleja domyślnie od BuildKit 0.11.

## 4. Cache poprawnie działa

Tag `buildcache` jest na DockerHub:

```bash
docker buildx imagetools inspect hubertkolejko/zad1-weather:buildcache
```

```
Name:      docker.io/hubertkolejko/zad1-weather:buildcache
MediaType: application/vnd.oci.image.manifest.v1+json
Digest:    sha256:2f0e297a3f52c6618f5ee5d7079b66f354877648b5f76020c509690810d0eae9
```

Żeby pokazać że cache faktycznie się przenosi przez registry, czyszczę lokalny cache buildera i puszczam ten sam build jeszcze raz:

```bash
docker buildx prune -f --builder zad1builder
docker buildx build \
  --builder zad1builder \
  --platform linux/amd64,linux/arm64 \
  --cache-from type=registry,ref=hubertkolejko/zad1-weather:buildcache \
  --cache-to   type=registry,ref=hubertkolejko/zad1-weather:buildcache,mode=max \
  -f Dockerfile.dod \
  -t hubertkolejko/zad1-weather:multiarch \
  --push .
```

W drugim buildzie wszystkie istotne kroki są oznaczone `CACHED`:

```
#3  CACHED   [internal] load metadata for docker.io/library/alpine:3.20
#10 CACHED   [linux/arm64->amd64 builder 4/6] RUN --mount=type=cache,target=/go/pkg/mod  go mod download
#15 CACHED   [linux/arm64 source 2/4] RUN apk add --no-cache git openssh-client ca-certificates
#17 CACHED   [linux/arm64->amd64 builder 6/6] RUN ... CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ...
#18 CACHED   [linux/arm64 source 4/4] RUN --mount=type=ssh --mount=type=secret,id=gh_token git clone ...
#20 CACHED   [linux/arm64 builder 6/6] RUN ... CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build ...
#22 CACHED   [linux/arm64 builder 4/6] RUN --mount=type=cache,target=/go/pkg/mod  go mod download
#11 #12 #13 #14 #16 #19 #21 #23 #24  CACHED  (COPY, WORKDIR, FROM)
```

W sumie 16 kroków z cache. Najważniejsze że cachują się też kosztowne `go build` dla obu architektur (#17, #20) i sam `mount=ssh` z `git clone` (#18). Drugi build przeszedł w kilkanaście sekund - większość czasu zajął sam push manifestu i eksport cache, bo żaden krok nie musiał być wykonany od zera.

## 5. Skan CVE (Trivy)

Zgodnie z wymogiem podstawowym 3 części dodatkowej zeskanowałem obraz Trivym:

```bash
trivy image --severity CRITICAL,HIGH \
  --format table -o docs/trivy-report.txt \
  hubertkolejko/zad1-weather:multiarch
```

Raporty są w `docs/trivy-report.txt` (single-arch) i `docs/trivy-report-multiarch.txt` (multi-arch).

```
Report Summary
┌────────┬──────────┬─────────────────┬─────────┐
│ Target │   Type   │ Vulnerabilities │ Secrets │
├────────┼──────────┼─────────────────┼─────────┤
│ server │ gobinary │        0        │    -    │
└────────┴──────────┴─────────────────┴─────────┘
```

Zero podatności CRITICAL/HIGH dla obu wariantów.

Drobna historia: pierwszy build leciał na `golang:1.22-alpine` i Trivy wykrył 10 podatności (1 CRITICAL + 9 HIGH) wszystkie w stdlib Go 1.22.12. Po podbiciu `ARG GO_VERSION=1.26` (czyli `golang:1.26-alpine` z Go 1.26.2) drugi skan zwrócił 0 podatności, bo wszystkie te CVE są tam już załatane. Sam obraz finalny to `scratch` z dwoma plikami (binarka i CA certs), więc nie ma w nim żadnego apk/apt/glibc do skanowania - jedyne co Trivy widzi to wkompilowana stdlib Go w binarce, i wystarczy odpowiednia wersja toolchainu żeby było czysto.

## Linki

- GitHub: https://github.com/GhostekTheGuy/zad1_lab_docker
- DockerHub: https://hub.docker.com/r/hubertkolejko/zad1-weather
  - `multiarch` / `1.0.0-dod` - obraz multi-arch z części dodatkowej
  - `buildcache` - cache registry (mode=max)
  - `latest` / `1.0.0` - obraz z części obowiązkowej (single-arch)
