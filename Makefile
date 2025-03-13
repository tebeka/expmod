all:
	$(error please pick a target)

lint:
	staticcheck ./...
	gosec ./...
	govulncheck ./...

test: lint
	go test -v

install-tools:
	curl -sfL https://raw.githubusercontent.com/securego/gosec/master/install.sh | sh -s -- -b $(shell go env GOPATH)/bin v2.22.2
	go install golang.org/x/vuln/cmd/govulncheck@latest
	go install honnef.co/go/tools/cmd/staticcheck@latest
	go install github.com/caarlos0/svu@latest
	@echo "Done. Don't forget to add '\$$(go env GOPATH)/bin' to your '\$$PATH'"

ci: install-tools test

release-patch:
	git tag $(shell svu patch)
	git push --tags

release-minor:
	git tag $(shell svu minor)
	git push --tags
