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

build-lite:
	@mkdir -p $(dir $(BINARY))
	@go build -tags lite -trimpath -ldflags="-s -w" -o plugins/kizunax/bin/kizunax-lite ./cmd/kizunax
	@echo "Built (lite, regex-only): plugins/kizunax/bin/kizunax-lite"

test-lite:
	@go test -tags lite ./internal/...
