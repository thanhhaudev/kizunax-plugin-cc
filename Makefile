.PHONY: build clean test

BINARY := plugins/kizunax/bin/kizunax

build:
	@mkdir -p $(dir $(BINARY))
	@go build -trimpath -ldflags="-s -w" -o $(BINARY) ./cmd/kizunax
	@echo "Built: $(BINARY)"

clean:
	@rm -f $(BINARY)

test:
	@go test ./...

run-setup: build
	@$(BINARY) setup --check

run-review: build
	@$(BINARY) review --working-tree
