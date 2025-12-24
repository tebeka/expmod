all:
	$(error please pick a target)

lint:
	go tool staticcheck ./...
	go tool gosec ./...
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

release-patch:
	$(eval NEW_VERSION := $(shell go tool svu patch))
	$(eval COMMIT := $(shell git rev-parse --short HEAD))
	go run bump_version.go -version $(NEW_VERSION) -commit $(COMMIT) < main.go > main.go.tmp && mv main.go.tmp main.go
	go fmt main.go
	git add main.go
	git commit -m "Bump version to $(NEW_VERSION)"
	git tag $(NEW_VERSION)
	git push && git push --tags

release-minor:
	$(eval NEW_VERSION := $(shell go tool svu minor))
	$(eval COMMIT := $(shell git rev-parse --short HEAD))
	go run bump_version.go -version $(NEW_VERSION) -commit $(COMMIT) < main.go > main.go.tmp && mv main.go.tmp main.go
	go fmt main.go
	git add main.go
	git commit -m "Bump version to $(NEW_VERSION)"
	git tag $(NEW_VERSION)
	git push && git push --tags
