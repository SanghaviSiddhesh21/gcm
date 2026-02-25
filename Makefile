.PHONY: test test-verbose test-coverage coverage-check build clean help

# Packages included in coverage measurement.
# internal/ui is excluded — TUI code requires a real terminal and cannot be coverage-tested.
# UI tests still run (compile + correctness check) via `go test ./internal/ui/...` in the test target.
COVERAGE_PKGS = ./internal/git/... ./internal/store/... ./internal/config/... ./internal/ai/...

## help: Show this help message
help:
	@echo "Available targets:"
	@sed -n 's/^## //p' ${MAKEFILE_LIST} | column -t -s ':'

## test: Run all tests with coverage
test:
	@go test ./internal/ui/... \
		&& go test -coverprofile=coverage.out $(COVERAGE_PKGS) \
		&& echo "" \
		&& go tool cover -func=coverage.out | tail -1

## coverage-check: Fail if coverage is below 80%
coverage-check:
	@go tool cover -func=coverage.out | grep ^total | awk '{ \
		gsub(/%/, "", $$3); \
		if ($$3+0 < 80) { \
			print "Coverage " $$3 "% is below the 80% threshold"; exit 1 \
		} else { \
			print "Coverage " $$3 "% meets the 80% threshold" \
		} \
	}'

## test-verbose: Run all tests with verbose output
test-verbose:
	@go test -v ./...

## test-coverage: Generate HTML coverage report
test-coverage:
	@go test -coverprofile=coverage.out $(COVERAGE_PKGS) && go tool cover -html=coverage.out -o coverage.html && echo "✓ Coverage report generated: coverage.html"

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
