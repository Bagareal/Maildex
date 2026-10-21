.PHONY: dev run build test icons

# Run locally with data in ./data (served on http://localhost:8080).
dev run:
	MAILDEX_DATA=./data go run ./cmd/maildex

build:
	CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o dist/maildex ./cmd/maildex

test:
	go vet ./...
	go test ./...

# Regenerate the PWA icons in web/static/icons.
icons:
	go run ./tools/genicons
