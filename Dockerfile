FROM node:20-bookworm AS web-builder
WORKDIR /src
COPY package.json package-lock.json ./
COPY web/admin/package.json ./web/admin/package.json
RUN npm ci
COPY web/admin ./web/admin
RUN npm run build --workspace web/admin

FROM golang:1.23-bookworm AS go-builder
RUN apt-get update && apt-get install -y --no-install-recommends build-essential ca-certificates && rm -rf /var/lib/apt/lists/*
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
COPY plugins ./plugins
COPY proto ./proto
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -o /out/bin/gateway ./cmd/gateway && \
    CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -o /out/bin/xiaomi-plugin ./plugins/xiaomi/cmd/xiaomi-plugin && \
    CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -o /out/bin/petkit-plugin ./plugins/petkit/cmd/petkit-plugin && \
    CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -o /out/bin/haier-plugin ./plugins/haier/cmd/haier-plugin

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=go-builder /out/bin ./bin
COPY --from=web-builder /src/web/admin/dist ./web/admin/dist
RUN mkdir -p /app/data
ENV CELESTIA_ADDR=:8080
ENV CELESTIA_DB_PATH=/app/data/celestia.db
EXPOSE 8080
VOLUME ["/app/data"]
CMD ["./bin/gateway"]

