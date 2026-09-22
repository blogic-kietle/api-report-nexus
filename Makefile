VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: run build test lint fmt dup tidy openapi proto tailwind docker docker-run

run:
	@set -a; [ -f .env.development ] && . ./.env.development; set +a; go run -ldflags '$(LDFLAGS)' ./cmd/api

build:
	CGO_ENABLED=0 go build -trimpath -ldflags '$(LDFLAGS)' -o bin/report-nexus ./cmd/api

test:
	go test -race -count=1 ./...

lint:
	golangci-lint run ./...

fmt:
	golangci-lint fmt ./...

dup:
	npx -y jscpd@5 --no-tips --summary --summary-top 5 .

tidy:
	go mod tidy

docker:
	docker build --build-arg VERSION=$(VERSION) -t report-nexus:$(VERSION) -t report-nexus:latest .

docker-run:
	VERSION=$(VERSION) docker compose up --build

tailwind:
	@d=$$(mktemp -d) && cd $$d && npm i --silent tailwindcss@4.3.3 @tailwindcss/cli@4.3.3 && printf '@import "tailwindcss" source(none);\n@source "%s";\n' $(CURDIR)/assets/templates/delivery-fee-report.html > in.css && npx @tailwindcss/cli -i in.css -o $(CURDIR)/assets/vendor/tailwind.css -m

openapi:
	go run ./cmd/openapi > assets/openapi.yaml.new && mv assets/openapi.yaml.new assets/openapi.yaml && npx -y @redocly/cli lint assets/openapi.yaml

proto:
	go run github.com/bufbuild/buf/cmd/buf@v1.73.0 generate
