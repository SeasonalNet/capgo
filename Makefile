.PHONY: check quality fmt-check lint gitleaks test vet

check: quality gitleaks

quality: fmt-check lint vet test

fmt-check:
	@test -z "$$(gofmt -l .)" || { gofmt -l .; exit 1; }

lint:
	golangci-lint run ./...

gitleaks:
	gitleaks dir . --config .gitleaks.toml --redact --verbose --no-banner

vet:
	go vet ./...

test:
	go test ./...
