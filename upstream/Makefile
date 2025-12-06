TARGET = kubernetes
M = $(shell printf "\033[34;1m🐱\033[0m")

.PHONY: apply
apply: ## Apply config to the current cluster
	@echo "$(M) ko apply on config/$(TARGET)"
	@ko apply -f config/$(TARGET)
