VERSION   ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS   := -ldflags "-s -w -X main.version=$(VERSION)"
BINARY  := vox
WAILS3  := $(shell which wails3 2>/dev/null || echo $(HOME)/go/bin/wails3)

.PHONY: build headless bindings frontend dev run tidy test clean

# Generate Wails v3 bindings for frontend
bindings:
	$(WAILS3) generate bindings

# Build frontend (Svelte via Vite)
frontend: bindings
	cd frontend && npm install && npm run build

# Desktop build (bindings + frontend + Go binary + macOS .app bundle)
build: frontend
	go build $(LDFLAGS) -o build/bin/$(BINARY) .
	@if [ -d build/bin/$(BINARY).app ]; then \
		cp build/bin/$(BINARY) build/bin/$(BINARY).app/Contents/MacOS/$(BINARY); \
		echo "Updated $(BINARY).app bundle"; \
	fi

# Headless build (-tags nogui): transcribe CLI only, no Wails, no frontend,
# no GTK/WebKit — what the release ships as vox-headless for Linux servers.
headless:
	go build -tags nogui $(LDFLAGS) -o build/bin/$(BINARY)-headless .

# Development: run frontend dev server + Go binary separately
dev:
	@echo "Run in two terminals:"
	@echo "  1) cd frontend && npm run dev"
	@echo "  2) go run $(LDFLAGS) ."

run: build
	./build/bin/$(BINARY)

tidy:
	go mod tidy

test:
	go test ./...

clean:
	rm -f $(BINARY) $(BINARY).exe
	rm -rf build/bin/ frontend/bindings/
