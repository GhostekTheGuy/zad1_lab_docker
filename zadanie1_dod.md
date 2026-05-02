# Zadanie 1 — część NIEOBOWIĄZKOWA, wariant 3 (max. +80%)

**Autor:** Hubert Kolejko
**Repozytorium GitHub:** <https://github.com/GhostekTheGuy/zad1_lab_docker>
**Repozytorium DockerHub:** <https://hub.docker.com/r/hubertkolejko/zad1-weather>

> **Wybrany poziom:** **3 (max. +80%)** — multi-arch (linux/amd64 + linux/arm64) z builderem `docker-container`, rozszerzony frontend BuildKit, `--mount=type=ssh` / `--mount=type=secret` do pobrania kodu z publicznego repozytorium GitHub w trakcie buildu, eksporter cache `registry` w trybie `max`.

---

## Plik Dockerfile

Wykorzystano osobny plik [`Dockerfile.dod`](Dockerfile.dod) z dyrektywą `# syntax=docker/dockerfile:1.7-labs` (rozszerzony frontend BuildKit). Etap `source` używa równolegle dwóch typów mount-ów:

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

Logika etapu `source`:

1. Najpierw próba klonowania przez SSH (gdy host przekazał agenta `--ssh default` z dostępnym kluczem) — preferowane, demonstruje **mount=type=ssh**.
2. Fallback do HTTPS z PAT-em (gdy host przekazał `--secret id=gh_token,...`) — demonstruje **mount=type=secret**.
3. Ostatnia linia obrony: anonimowy klon HTTPS (możliwy, bo repozytorium jest publiczne).

Kolejne etapy (`builder` na bazie `golang:1.22-alpine`, `runtime` na bazie `scratch`) — analogicznie jak w wersji obowiązkowej.

---

## 1. Builder oparty na sterowniku `docker-container`

Konfiguracja BuildKit (plik `/tmp/buildkitd.toml`) — dopuszcza HTTP do registry użytego do cache'a (lokalnie w trakcie testów `localhost:5050`; do zgłoszenia DockerHub):

```toml
[registry."localhost:5050"]
  http = true
  insecure = true
```

Utworzenie buildera:

```bash
docker buildx create --name zad1builder \
  --driver docker-container \
  --driver-opt network=host \
  --buildkitd-config /tmp/buildkitd.toml \
  --bootstrap --use
```

Weryfikacja:

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

---

## 2. Build multi-arch z mount=ssh/secret + cache registry max

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

(Opcjonalnie: dodać `--secret id=gh_token,src=$HOME/.gh_token` jeśli klonowanie ma iść przez PAT zamiast SSH.)

---

## 3. Weryfikacja: manifest zawiera obie platformy

```bash
docker buildx imagetools inspect hubertkolejko/zad1-weather:multiarch
```

Wynik (wycinek):

```
Name:      docker.io/hubertkolejko/zad1-weather:multiarch
MediaType: application/vnd.oci.image.index.v1+json
Digest:    sha256:96020f58651be7c8d78383ecd5a74de4c6339751d8f8ac34e22d9e6f24a1590d

Manifests:
  Name:        docker.io/hubertkolejko/zad1-weather:multiarch@sha256:d54644705bfd2679...
  MediaType:   application/vnd.oci.image.manifest.v1+json
  Platform:    linux/amd64

  Name:        docker.io/hubertkolejko/zad1-weather:multiarch@sha256:b208b96c17b2ab6e...
  MediaType:   application/vnd.oci.image.manifest.v1+json
  Platform:    linux/arm64

  Name:        docker.io/hubertkolejko/zad1-weather:multiarch@sha256:455495901fc17b4a...
  MediaType:   application/vnd.oci.image.manifest.v1+json
  Platform:    unknown/unknown
  Annotations:
    vnd.docker.reference.type:   attestation-manifest
    vnd.docker.reference.digest: sha256:d54644705bfd2679...

  Name:        docker.io/hubertkolejko/zad1-weather:multiarch@sha256:376c3089dd8f2a3e...
  MediaType:   application/vnd.oci.image.manifest.v1+json
  Platform:    unknown/unknown
  Annotations:
    vnd.docker.reference.type:   attestation-manifest
    vnd.docker.reference.digest: sha256:b208b96c17b2ab6e...
```

✅ Manifest zawiera deklaracje dla **linux/amd64** i **linux/arm64**, plus dwa attestation manifesty (provenance) dla każdej platformy — co jest standardowym artefaktem buildx z BuildKit ≥ 0.11.

---

## 4. Weryfikacja: cache z DockerHub jest poprawnie wykorzystywany

Tag `buildcache` istnieje na DockerHub:

```bash
docker buildx imagetools inspect hubertkolejko/zad1-weather:buildcache
```

```
Name:      docker.io/hubertkolejko/zad1-weather:buildcache
MediaType: application/vnd.oci.image.manifest.v1+json
Digest:    sha256:2f0e297a3f52c6618f5ee5d7079b66f354877648b5f76020c509690810d0eae9
```

**Test poprawności cache** — czyścimy lokalny cache buildera i powtarzamy build:

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

Wycinek z drugiego buildu — wszystkie znaczące kroki dla obu platform są oznaczone `CACHED`:

```
#3  CACHED   [internal] load metadata for docker.io/library/alpine:3.20
#10 CACHED   [linux/arm64->amd64 builder 4/6] RUN --mount=type=cache,target=/go/pkg/mod  go mod download
#15 CACHED   [linux/arm64 source 2/4] RUN apk add --no-cache git openssh-client ca-certificates
#17 CACHED   [linux/arm64->amd64 builder 6/6] RUN ... CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ...
#18 CACHED   [linux/arm64 source 4/4] RUN --mount=type=ssh --mount=type=secret,id=gh_token git clone ...
#20 CACHED   [linux/arm64 builder 6/6] RUN ... CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build ...
#22 CACHED   [linux/arm64 builder 4/6] RUN --mount=type=cache,target=/go/pkg/mod  go mod download
#11 #12 #13 #14 #16 #19 #21 #23 #24  CACHED   pozostałe kroki (COPY, WORKDIR, FROM)
```

✅ **16 kroków `CACHED`** — w tym kosztowne kompilacje Go (kroki #17, #20) i krok `mount=ssh` z klonowaniem repo (krok #18). Drugi build skończył się w kilkanaście sekund (głównie czas eksportu cache i pushu manifestu).

---

## 5. Skan podatności CVE (Trivy)

Zgodnie z wymogiem **PODSTAWOWYM nr 3** części dodatkowej obraz został przeskanowany pod kątem podatności CRITICAL/HIGH:

```bash
trivy image --severity CRITICAL,HIGH \
  --format table -o docs/trivy-report.txt \
  hubertkolejko/zad1-weather:multiarch
```

Pełny raport: [`docs/trivy-report.txt`](docs/trivy-report.txt) (single-arch) oraz [`docs/trivy-report-multiarch.txt`](docs/trivy-report-multiarch.txt) (multi-arch).

**Wynik**:

```
Report Summary
┌────────┬──────────┬─────────────────┬─────────┐
│ Target │   Type   │ Vulnerabilities │ Secrets │
├────────┼──────────┼─────────────────┼─────────┤
│ server │ gobinary │        0        │    -    │
└────────┴──────────┴─────────────────┴─────────┘
```

✅ **Zero podatności CRITICAL/HIGH** w obrazie zarówno dla wariantu single-arch jak i multi-arch.

**Komentarz do iteracji**: Pierwszy build (Go 1.22.12 z `golang:1.22-alpine`) ujawnił 10 podatności (1 CRITICAL + 9 HIGH) wszystkie w Go `stdlib`. Po podbiciu `ARG GO_VERSION=1.26` w obu Dockerfile'ach (toolchain `golang:1.26-alpine` zawiera Go 1.26.2, w którym wszystkie te CVE są załatane) ponowny skan zwrócił **0 podatności**. Pokazuje to ważność regularnego rebuildu obrazu z aktualnym toolchainem nawet dla aplikacji w obrazie scratch — choć w samym obrazie nie ma żadnych pakietów dystrybucji, statycznie wkompilowana stdlib Go jest objęta skanem `gobinary` w Trivy.

**Uzasadnienie minimalnej powierzchni ataku**:

- Obraz finalny `FROM scratch` — brak jakiegokolwiek systemu plików dystrybucji, brak menedżera pakietów, brak binarek systemowych.
- W obrazie znajdują się dosłownie **2 pliki**: skompilowana statycznie binarka Go (`/server`) oraz pakiet zaufanych certyfikatów (`/etc/ssl/certs/ca-certificates.crt`). To eliminuje cały typowy zakres CVE z poziomu OS (apk/apt/musl/glibc).
- **Brak zewnętrznych zależności w `go.mod`** — używana jest wyłącznie biblioteka standardowa Go. CVE w stdlib Go są łatane wraz z aktualizacją toolchainu, więc rebuild rozwiązuje problem.

---

## 6. Linki finalne

- **GitHub**: <https://github.com/GhostekTheGuy/zad1_lab_docker>
- **DockerHub**: <https://hub.docker.com/r/hubertkolejko/zad1-weather>
  - `multiarch` / `1.0.0-dod` — obraz multi-arch (linux/amd64 + linux/arm64) z części dodatkowej v3
  - `buildcache` — registry cache (mode=max) dla powyższego obrazu
  - `latest` / `1.0.0` — obraz z części obowiązkowej (single-arch)
