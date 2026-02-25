.PHONY: all lint test install-tools ci release release-patch release-minor deploy

GCP_REGION ?= us-central1

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
	@echo "Releasing $(NEW_VERSION)..."
	git tag $(NEW_VERSION)
	git push
	git push --tags

release-patch:
	$(MAKE) release TYPE=patch

release-minor:
	$(MAKE) release TYPE=minor

deploy:
	gcloud run deploy expmod \
		--source . \
		--region $(GCP_REGION) \
		--set-secrets GITHUB_TOKEN=github-token:latest \
		--allow-unauthenticated
