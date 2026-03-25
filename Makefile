GOCACHE ?= $(CURDIR)/.cache/go-build
GOMODCACHE ?= $(CURDIR)/.cache/gomod

.PHONY: test build build-web run docker-build

test:
	GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) go test ./...

build:
	mkdir -p bin
	GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) go build -o ./bin/gateway ./cmd/gateway
	GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) go build -o ./bin/xiaomi-plugin ./plugins/xiaomi/cmd/xiaomi-plugin
	GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) go build -o ./bin/petkit-plugin ./plugins/petkit/cmd/petkit-plugin
	GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) go build -o ./bin/haier-plugin ./plugins/haier/cmd/haier-plugin

build-web:
	npm run build --workspace web/admin

run: build
	CELESTIA_ADDR=127.0.0.1:8080 ./bin/gateway

docker-build:
	docker build -t celestia:latest .

