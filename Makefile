.PHONY: build clean test

BINARY := plugins/kizunax/bin/kizunax

build:
	@mkdir -p $(dir $(BINARY))
	@go build -trimpath -ldflags="-s -w" -o $(BINARY) ./cmd/kizunax
	@echo "Built: $(BINARY)"

clean:
	@rm -f $(BINARY)

test:
	@go test ./internal/...

test-verbose:
	@go test -v ./internal/...

test-update-golden:
	@go test ./internal/render -update

cover:
	@go test -coverprofile=coverage.out ./internal/...
	@go tool cover -func=coverage.out | tail -20

run-setup: build
	@$(BINARY) setup --check

run-review: build
	@$(BINARY) review --working-tree
