.PHONY: test test-verbose test-coverage build clean help

## help: Show this help message
help:
	@echo "Available targets:"
	@sed -n 's/^## //p' ${MAKEFILE_LIST} | column -t -s ':'

## test: Run all tests with coverage
test:
	@go test ./internal/ui/... \
		&& go test -coverprofile=coverage.out ./internal/git/... ./internal/store/... ./internal/config/... ./internal/ai/... \
		&& echo "" \
		&& go tool cover -func=coverage.out | tail -1

## test-verbose: Run all tests with verbose output
test-verbose:
	@go test -v ./...

## test-coverage: Run all tests and generate coverage report
test-coverage:
	@go test -coverprofile=coverage.out ./internal/git ./internal/store ./internal/config && go tool cover -html=coverage.out -o coverage.html && echo "✓ Coverage report generated: coverage.html"

## build: Build the gcm binary
build:
	@go build -o gcm ./

## clean: Remove build artifacts and coverage files
clean:
	@rm -f gcm coverage.out coverage.html
	@echo "✓ Cleaned build artifacts"

## lint: Run linter (requires golangci-lint)
lint:
	@golangci-lint run ./...
