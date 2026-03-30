ARG TARGETOS
ARG TARGETARCH

FROM golang:1.23-bookworm AS go-builder
ARG TARGETOS
ARG TARGETARCH
RUN apt-get update && apt-get install -y --no-install-recommends build-essential ca-certificates && rm -rf /var/lib/apt/lists/*
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
COPY plugins ./plugins
COPY proto ./proto
RUN target_os="${TARGETOS:-$(go env GOOS)}"; \
    target_arch="${TARGETARCH:-$(go env GOARCH)}"; \
    CGO_ENABLED=1 GOOS="$target_os" GOARCH="$target_arch" go build -tags hikvision_sdk -o /out/hikvision-plugin ./plugins/hikvision/cmd

FROM debian:bookworm-slim

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        ca-certificates \
        libstdc++6 \
        libgcc-s1 \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /opt/celestia
COPY --from=go-builder /out/hikvision-plugin /opt/celestia/bin/hikvision-plugin
COPY plugins/hikvision/sdk/lib/arm64 /opt/celestia/sdk/lib/arm64

ENV CELESTIA_HIKVISION_SDK_LIB_DIR=/opt/celestia/sdk/lib/arm64
ENV LD_LIBRARY_PATH=/opt/celestia/sdk/lib/arm64:/opt/celestia/sdk/lib/arm64/HCNetSDKCom
ENV CELESTIA_HIKVISION_PLUGIN_MODE=server

ENTRYPOINT ["/opt/celestia/bin/hikvision-plugin"]
