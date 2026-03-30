ARG TARGETOS
ARG TARGETARCH

FROM golang:1.23-bookworm AS go-builder
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
COPY plugins ./plugins
COPY proto ./proto
RUN target_os="${TARGETOS:-$(go env GOOS)}"; \
    target_arch="${TARGETARCH:-$(go env GOARCH)}"; \
    CGO_ENABLED=1 GOOS="$target_os" GOARCH="$target_arch" go build -o /out/hikvision-plugin ./plugins/hikvision/cmd/hikvision-plugin

FROM ubuntu:22.04
ENV DEBIAN_FRONTEND=noninteractive \
    PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        ca-certificates \
        ffmpeg \
        python3 \
        python3-pip \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /opt/ezviz
COPY ha-hikvision-ezviz/addon/ezviz_hcnet/requirements.txt /tmp/requirements.txt
RUN pip3 install --no-cache-dir -r /tmp/requirements.txt

COPY ha-hikvision-ezviz/addon/ezviz_hcnet/service /opt/ezviz/service
COPY ha-hikvision-ezviz/addon/ezviz_hcnet/lib/arm64 /opt/ezviz/lib/arm64

RUN mkdir -p /opt/celestia/bin
COPY --from=go-builder /out/hikvision-plugin /opt/celestia/bin/hikvision-plugin
COPY plugins/hikvision/docker/entrypoint.sh /opt/celestia/entrypoint.sh

ENV EZVIZ_SDK_LIB_DIR=/opt/ezviz/lib/arm64
ENV EZVIZ_BACKEND_PORT=8099

ENTRYPOINT ["/opt/celestia/entrypoint.sh"]
