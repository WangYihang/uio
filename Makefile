.PHONY: all build test race cover lint fmt vet tidy integration clean

all: fmt vet lint test

build:
	go build ./...

test:
	go test ./...

race:
	go test -race ./...

cover:
	go test -race -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out | tail -1

lint:
	golangci-lint run ./...

fmt:
	gofmt -w .

vet:
	go vet ./...

tidy:
	go mod tidy

# integration runs the S3 tests against the docker-compose MinIO service.
integration:
	docker compose up -d
	UIO_TEST_S3=1 go test -race ./...
	docker compose down -v

clean:
	rm -f coverage.out
