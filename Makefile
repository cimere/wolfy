export CGO_ENABLED := 0

.PHONY: build test check fmt fmt-check audit demo linux smoke
build:
	go build -trimpath -o bin/wolfy ./cmd/wolfy

test:
	go test ./...

fmt:
	gofmt -w cmd internal

fmt-check:
	@test -z "$$(gofmt -l cmd internal)" || (gofmt -l cmd internal; exit 1)

check: fmt-check
	go vet ./...
	go test ./...
	go build -trimpath -o bin/wolfy ./cmd/wolfy

audit:
	go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...

smoke: build
	bash scripts/smoke-demo.sh

demo:
	WOLFY_MODE=demo WOLFY_DB=data/demo.db go run ./cmd/wolfy

linux:
	GOOS=linux GOARCH=amd64 go build -trimpath -o bin/wolfy-linux-amd64 ./cmd/wolfy
