.PHONY: test lint build check fix

# Run all tests with race detection and coverage
test:
	go test -v -race -coverprofile=coverage.out ./...

# Run linter
lint:
	golangci-lint run ./...

# Build all packages
build:
	go build ./...

# Run all checks (build, test, lint)
check: build test lint

# Run go fix
fix:
	go fix ./...
	go mod tidy
