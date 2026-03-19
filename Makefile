.PHONY: dev build clean

WAILS := $(shell which wails3 2>/dev/null || echo $(HOME)/go/bin/wails3)

# Development with hot-reload
dev:
	$(WAILS) dev

# Production build for current platform
build:
	$(WAILS) build

clean:
	rm -rf build/bin
	rm -rf frontend/dist
	rm -rf frontend/node_modules
