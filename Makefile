TARGET = kubernetes
M = $(shell printf "\033[34;1m🐱\033[0m")

.PHONY: apply
apply: ## Apply config to the current cluster
	@echo "$(M) ko apply on config/$(TARGET)"
	@ko apply -f config/$(TARGET)

.PHONY: test-unit
test-unit: ## Run unit tests
	@echo "$(M) Running unit tests"
	go test -v -race ./...

.PHONY: lint
lint: ## Run go vet
	@echo "$(M) Running go vet"
	go vet ./...

.PHONY: fmt
fmt: ## Run gofmt
	@echo "$(M) Running gofmt"
	@find . -name '*.go' -not -path './vendor/*' -not -path './third_party/*' | xargs gofmt -l -w

GITLINT_COMMITS ?= HEAD

.PHONY: gitlint
gitlint: ## Lint commit messages (override range with GITLINT_COMMITS=origin/main..HEAD)
	@echo "$(M) Running gitlint"
	python3 -m gitlint.cli --commits $(GITLINT_COMMITS)
