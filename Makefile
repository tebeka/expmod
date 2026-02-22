.PHONY: all lint test install-tools ci release release-patch release-minor

all:
	$(error please pick a target)

lint:
	go tool staticcheck ./...
	go tool gosec -verbose golint ./...
	go tool govulncheck ./...

test: lint
	go test -v

install-tools:
	go install github.com/caarlos0/svu@latest
	go install github.com/securego/gosec/v2/cmd/gosec@latest
	go install golang.org/x/vuln/cmd/govulncheck@latest
	go install honnef.co/go/tools/cmd/staticcheck@latest
	@echo "Tools installed from go.mod tool directive"

ci: install-tools test

# Usage: make release TYPE=patch  (or TYPE=minor)
release:
	@test -n "$(TYPE)" || (echo "error: TYPE required (patch or minor)" && exit 1)
	$(eval NEW_VERSION := $(shell go tool svu $(TYPE)))
	$(eval COMMIT := $(shell git rev-parse --short HEAD))
	@echo "Releasing $(NEW_VERSION)..."
	go run bump_version.go -version $(NEW_VERSION) -commit $(COMMIT) < main.go > main.go.tmp
	mv main.go.tmp main.go
	go fmt main.go
	git add main.go
	git commit -m "Bump version to $(NEW_VERSION)"
	git tag $(NEW_VERSION)
	git push
	git push --tags

release-patch:
	$(MAKE) release TYPE=patch

release-minor:
	$(MAKE) release TYPE=minor
