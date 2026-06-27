.PHONY: test build release run tidy zip

test:
	go test ./...

build:
	mkdir -p bin
	CGO_ENABLED=1 go build -trimpath -ldflags="-s -w" -o bin/ai-shortlink ./cmd/server

release:
	mkdir -p dist/ai-shortlink-linux-amd64
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o dist/ai-shortlink-linux-amd64/ai-shortlink ./cmd/server
	cp deploy/app.conf.example dist/ai-shortlink-linux-amd64/shortlink.env.example
	cp deploy/README_BINARY.md dist/ai-shortlink-linux-amd64/README_BINARY.md
	cd dist && zip -qr ai-shortlink-linux-amd64.zip ai-shortlink-linux-amd64

run:
	go run ./cmd/server

tidy:
	go mod tidy

zip:
	cd .. && zip -r ai-shortlink.zip ai-shortlink -x 'ai-shortlink/.git/*' 'ai-shortlink/data/*'
