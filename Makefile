.PHONY: build docs install snapshot target-schemas test

CUE := go run cuelang.org/go/cmd/cue@v0.17.1

build: test
	go build -o bin/terraform-provider-dashboard .

docs:
	go tool tfplugindocs generate --provider-name dashboard

target-schemas:
	go run ./internal/targetschemagen grafanarenderer/target-schemas -- $(CUE)

install: build
	@printf '%s\n' \
		'Add to ~/.terraformrc for plans to use this build:' \
		'provider_installation {' \
		'  dev_overrides {' \
		'    "litebitus/dashboard" = "$(CURDIR)/bin"' \
		'  }' \
		'  direct {}' \
		'}'

snapshot: test
	goreleaser release --snapshot --clean --skip=sign

test:
	go vet -tags acceptance ./...
	@unformatted="$$(gofmt -l .)"; [ -z "$$unformatted" ] || { echo "not gofmt-formatted:"; echo "$$unformatted"; exit 1; }
	go test -tags acceptance -count=1 ./...
	@if validated="$$(go tool tfplugindocs validate --provider-name dashboard 2>&1)"; then \
		echo "tfplugindocs validate: the Registry pages match the provider"; \
	else \
		printf '%s\n' "$$validated" | awk '/\[DEBUG\]/ { next } /^Error/ { failed = 1 } \
			{ if (failed) printf "\033[31m%s\033[0m\n", $$0; else print }'; \
		printf '\033[31m%s\033[0m\n' "tfplugindocs validate: failed, the Registry pages do not match the provider"; \
		exit 1; \
	fi
