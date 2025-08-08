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
	git tag $(shell go tool svu patch)
	git push --tags

release-minor:
	git tag $(shell go tool svu minor)
	git push --tags
