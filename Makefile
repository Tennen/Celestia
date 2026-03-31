GOCACHE ?= $(CURDIR)/.cache/go-build
GOMODCACHE ?= $(CURDIR)/.cache/gomod

.PHONY: test build build-web run docker-build docker-build-hikvision

test:
	GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) go test ./...

build:
	mkdir -p bin
	GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) go build -o ./bin/gateway ./cmd/gateway
	GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) go build -o ./bin/xiaomi-plugin ./plugins/xiaomi/cmd
	GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) go build -o ./bin/petkit-plugin ./plugins/petkit/cmd
	GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) go build -o ./bin/haier-plugin ./plugins/haier/cmd
	target_os="$$(go env GOOS)"; \
	target_arch="$$(go env GOARCH)"; \
	if [ "$$target_os" = "linux" ] && [ "$$target_arch" = "arm64" ]; then \
		GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) go build -tags hikvision_sdk -o ./bin/hikvision-plugin ./plugins/hikvision/cmd; \
	else \
		GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) go build -o ./bin/hikvision-plugin ./plugins/hikvision/cmd; \
	fi

build-web:
	npm run build --workspace web/admin

run: build
	CELESTIA_ADDR=127.0.0.1:8080 ./bin/gateway

docker-build:
	docker build -t celestia:latest .

docker-build-hikvision:
	docker build -f plugins/hikvision/Dockerfile -t celestia-hikvision-plugin:latest .
