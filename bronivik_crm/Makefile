.PHONY: build run test test-coverage lint clean docker-build docker-run

build:
	CGO_ENABLED=1 go build -o bin/bronivik-crm ./cmd/bot

run:
	go run ./cmd/bot

test:
	go test -v -race ./...

test-coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

lint:
	golangci-lint run

clean:
	rm -rf bin/ coverage.out

docker-build:
	docker build -t bronivik-crm:latest .

docker-run:
	docker run --rm -it \
		-v $(PWD)/configs:/app/configs \
		-v $(PWD)/data:/app/data \
		bronivik-crm:latest
