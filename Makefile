.PHONY: test build run tidy zip

test:
	go test ./...

build:
	mkdir -p bin
	go build -o bin/ai-shortlink ./cmd/server

run:
	go run ./cmd/server

tidy:
	go mod tidy

zip:
	cd .. && zip -r ai-shortlink.zip ai-shortlink -x 'ai-shortlink/.git/*' 'ai-shortlink/data/*'
