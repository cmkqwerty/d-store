build:
	@go build -o bin/ds

run: build
	@./bin/ds

test:
	@go test -v ./...