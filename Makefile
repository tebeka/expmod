all:
	$(error please pick a target)

lint:
	gosec ./...
	govulncheck ./...
	staticcheck ./...

test: lint
	go test -v

install-deps:
	go install github.com/securego/gosec/cmd/gosec@latest
	go install golang.org/x/vuln/cmd/govulncheck@latest
	go install honnef.co/go/tools/cmd/staticcheck@latest
	@echo "Done. Don't forget to add '\$$(go env GOPATH)/bin' to your '\$$PATH'"

ci:
	install-deps
	test
