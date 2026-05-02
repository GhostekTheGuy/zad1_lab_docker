# Zadanie 1 - część obowiązkowa

Laboratorium: Programowanie Aplikacji w Chmurze Obliczeniowej
Autor: Hubert Kolejko
GitHub: https://github.com/GhostekTheGuy/zad1_lab_docker
DockerHub: https://hub.docker.com/r/hubertkolejko/zad1-weather

## 1. Aplikacja (max. 30%)

Napisałem aplikację w Go (tylko stdlib, bez zewnętrznych pakietów). Wybrałem Go, bo daje statyczną binarkę i można odpalić w `scratch`, co liczy się do konkursu na rozmiar.

Dane pogodowe biorę z Open-Meteo (https://open-meteo.com) - darmowe, bez klucza API. Lista krajów i miast jest predefiniowana w kodzie (7 krajów, 18 miast - patrz zmienna `countries` w `cmd/server/main.go`).

### Logi przy starcie (punkt 1a)

Po uruchomieniu kontenera w logach pojawia się data, autor i port:

```
== Zadanie 1 - start aplikacji ==
Data uruchomienia : 2026-05-02T21:14:58Z
Autor             : Hubert Kolejko
Port TCP (nasłuch): 8080
Serwer gotowy: http://0.0.0.0:8080/
```

### UI (punkt 1b)

Endpointy:

- `GET /` - formularz HTML, dwa selecty (kraj, miasto). Po wybraniu kraju lista miast aktualizuje się przez prosty inline JS (dane jako embedded JSON).
- `GET /weather?country=PL&city=Warszawa` - pobiera pogodę z Open-Meteo i renderuje wynik (temperatura, odczuwalna, wilgotność, wiatr, opady, opis WMO).
- `GET /health` - zwraca 200 OK, używane przez healthcheck.

Aplikacja przyjmuje też flagę `-healthcheck`. W tym trybie binarka robi wewnętrzny GET na `/health` i kończy się kodem 0/1. Potrzebne, bo w `scratch` nie ma `curl` ani shella, więc klasyczny healthcheck nie zadziała.

Kod jest w `cmd/server/main.go` razem z komentarzami. Szablony HTML są wpisane jako stringi w kodzie, dzięki czemu obraz nie potrzebuje katalogu `templates/`.

## 2. Dockerfile (max. 50%)

Plik: `Dockerfile`. Build jest dwuetapowy:

1. Stage `builder` na bazie `golang:1.26-alpine`. Najpierw kopiuję sam `go.mod` i robię `go mod download`, dopiero potem kod - dzięki temu warstwa zależności cachuje się niezależnie od zmian w kodzie. Build statycznej binarki: `CGO_ENABLED=0 go build -trimpath -ldflags="-s -w"`. Cache go-modów i go-build trzymam w `--mount=type=cache`, więc nie powiększają obrazu.
2. Stage runtime na `FROM scratch`. Kopiuję tylko dwie rzeczy: `ca-certificates.crt` (potrzebne do HTTPS do Open-Meteo) i samą binarkę.

Wersja Go to 1.26 - podbiłem ją z 1.22, bo Trivy znalazł 10 CVE w stdlib Go 1.22 (patrz zadanie1_dod.md).

Etykiety OCI z autorem (zgodnie z treścią zadania):

```dockerfile
LABEL org.opencontainers.image.title="zad1-weather"
LABEL org.opencontainers.image.description="Aplikacja pogodowa - Zadanie 1"
LABEL org.opencontainers.image.authors="Hubert Kolejko"
LABEL org.opencontainers.image.source="https://github.com/GhostekTheGuy/zad1_lab_docker"
LABEL org.opencontainers.image.licenses="MIT"
LABEL org.opencontainers.image.version="1.0.0"
```

Healthcheck:

```dockerfile
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD ["/server", "-healthcheck"]
```

Pozostałe drobiazgi: `USER 65534:65534` żeby nie biegać jako root, `EXPOSE 8080`, `--platform=$BUILDPLATFORM` w stage builder + `ARG TARGETOS/TARGETARCH` żeby dało się robić cross-compile (przyda się w części dodatkowej).

## 3. Polecenia (max. 20%)

### a) Build

```bash
docker build -t hubertkolejko/zad1-weather:latest .
```

### b) Uruchomienie

```bash
docker run -d --name zad1 -p 8080:8080 hubertkolejko/zad1-weather:latest
```

Strona pod http://localhost:8080.

### c) Logi (punkt 1a)

```bash
docker logs zad1
```

Wynik:

```
== Zadanie 1 - start aplikacji ==
Data uruchomienia : 2026-05-02T21:14:58Z
Autor             : Hubert Kolejko
Port TCP (nasłuch): 8080
Serwer gotowy: http://0.0.0.0:8080/
```

### d) Warstwy i rozmiar

Liczba fizycznych warstw:

```bash
docker image inspect hubertkolejko/zad1-weather:latest --format '{{len .RootFS.Layers}}'
# 2
```

`docker history`:

```
CREATED BY                                      SIZE
ENTRYPOINT ["/server"]                          0B
HEALTHCHECK &{["CMD" "/server" "-healthche…     0B
EXPOSE map[8080/tcp:{}]                         0B
USER 65534:65534                                0B
LABEL org.opencontainers.image.version=1.0.0    0B
LABEL org.opencontainers.image.licenses=MIT     0B
LABEL org.opencontainers.image.source=https…    0B
LABEL org.opencontainers.image.authors=Hubert…  0B
LABEL org.opencontainers.image.description=A…   0B
LABEL org.opencontainers.image.title=zad1-we…   0B
COPY /out/server /server # buildkit             8.40MB
COPY /etc/ssl/certs/ca-certificates.crt /etc…   238kB
```

Rozmiar:

```bash
docker images hubertkolejko/zad1-weather:latest
```

```
IMAGE                               ID            DISK USAGE   CONTENT SIZE
hubertkolejko/zad1-weather:latest   663600a234a2       12.1MB         3.49MB
```

Czyli: 2 fizyczne warstwy, 3.49 MB skompresowany, 12.1 MB po rozpakowaniu. Sama binarka 8.4 MB, CA certy 238 kB.

## Zrzuty ekranu

Formularz:

![Formularz](docs/screenshot-form.png)

Wynik dla Warszawy:

![Pogoda](docs/screenshot.png)

## Linki

- GitHub: https://github.com/GhostekTheGuy/zad1_lab_docker
- DockerHub: https://hub.docker.com/r/hubertkolejko/zad1-weather
  - `latest` / `1.0.0` - wersja obowiązkowa (single-arch)
  - `multiarch` / `1.0.0-dod` - wersja z części dodatkowej (multi-arch, opis w `zadanie1_dod.md`)
  - `buildcache` - cache buildu z części dodatkowej
