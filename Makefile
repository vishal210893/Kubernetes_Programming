# Makefile for Kubernetes Programming Project

# Project variables
PROJECT_NAME := Kubernetes_Programming
MODULE := Kubernetes_Programming
APIS_PKG := pkg/apis
GENERATED_PKG := pkg/generated
CRD_OUTPUT_DIR := hack
BOILERPLATE := hack/boilerplate.go.txt

# Go variables
GO := go
GOFLAGS := -mod=mod
GOOS := $(shell go env GOOS)
GOARCH := $(shell go env GOARCH)

# Binary variables
BIN_DIR := bin
CLIENT_BINARY := $(BIN_DIR)/at-client
MAIN_GO := main.go

# Tools
CONTROLLER_GEN := controller-gen
CODEGEN_SCRIPT := hack/update-codegen.sh

# Kubernetes variables
KUBECONFIG := $(HOME)/.kube/config
NAMESPACE := default

# Color output
BLUE := \033[0;34m
GREEN := \033[0;32m
YELLOW := \033[0;33m
RED := \033[0;31m
NC := \033[0m # No Color

.DEFAULT_GOAL := help

##@ General

.PHONY: help
help: ## Display this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make $(BLUE)<target>$(NC)\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  $(BLUE)%-15s$(NC) %s\n", $$1, $$2 } /^##@/ { printf "\n$(YELLOW)%s$(NC)\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: build
build: ## Build the client binary
	@echo "$(GREEN)Building $(CLIENT_BINARY)...$(NC)"
	@mkdir -p $(BIN_DIR)
	$(GO) build -o $(CLIENT_BINARY) $(MAIN_GO)
	@echo "$(GREEN)Build complete: $(CLIENT_BINARY)$(NC)"

.PHONY: run
run: ## Run the main application
	@echo "$(GREEN)Running application...$(NC)"
	$(GO) run $(MAIN_GO)

.PHONY: clean
clean: ## Clean build artifacts
	@echo "$(YELLOW)Cleaning build artifacts...$(NC)"
	rm -rf $(BIN_DIR)
	@echo "$(GREEN)Clean complete$(NC)"

.PHONY: fmt
fmt: ## Format Go code
	@echo "$(GREEN)Formatting Go code...$(NC)"
	$(GO) fmt ./...

.PHONY: vet
vet: ## Run go vet
	@echo "$(GREEN)Running go vet...$(NC)"
	$(GO) vet ./...

.PHONY: test
test: ## Run tests
	@echo "$(GREEN)Running tests...$(NC)"
	$(GO) test -v ./...

.PHONY: tidy
tidy: ## Tidy go modules
	@echo "$(GREEN)Tidying go modules...$(NC)"
	$(GO) mod tidy

.PHONY: vendor
vendor: tidy ## Vendor dependencies
	@echo "$(GREEN)Vendoring dependencies...$(NC)"
	$(GO) mod vendor

##@ Code Generation

.PHONY: generate
generate: ## Generate deepcopy, clientset, listers, and informers
	@echo "$(GREEN)Generating client code...$(NC)"
	@chmod +x $(CODEGEN_SCRIPT)
	@bash $(CODEGEN_SCRIPT)
	@echo "$(GREEN)Code generation complete$(NC)"

.PHONY: manifests
manifests: ## Generate CRD manifests
	@echo "$(GREEN)Generating CRD manifests...$(NC)"
	$(CONTROLLER_GEN) crd:crdVersions=v1 paths=./$(APIS_PKG)/cnat/v1alpha1 output:crd:dir=./$(CRD_OUTPUT_DIR)
	@echo "$(GREEN)CRD generation complete: $(CRD_OUTPUT_DIR)/cnat.programming-kubernetes.info_ats.yaml$(NC)"

.PHONY: deepcopy
deepcopy: ## Generate deepcopy code only
	@echo "$(GREEN)Generating deepcopy code...$(NC)"
	$(CONTROLLER_GEN) object:headerFile=$(BOILERPLATE) paths=./$(APIS_PKG)/...
	@echo "$(GREEN)Deepcopy generation complete$(NC)"

.PHONY: codegen
codegen: generate manifests ## Run all code generation (client code + CRDs)
	@echo "$(GREEN)All code generation complete$(NC)"

##@ Verification

.PHONY: verify
verify: fmt vet ## Run verification checks
	@echo "$(GREEN)Verification complete$(NC)"

.PHONY: check-tools
check-tools: ## Check if required tools are installed
	@echo "$(GREEN)Checking required tools...$(NC)"
	@which $(GO) > /dev/null || (echo "$(RED)Go is not installed$(NC)" && exit 1)
	@which kubectl > /dev/null || (echo "$(RED)kubectl is not installed$(NC)" && exit 1)
	@which $(CONTROLLER_GEN) > /dev/null || (echo "$(RED)controller-gen is not installed$(NC)" && exit 1)
	@echo "$(GREEN)All required tools are installed$(NC)"

##@ Setup

.PHONY: setup
setup: vendor check-tools ## Initial setup: vendor dependencies and check tools
	@echo "$(GREEN)Setup complete$(NC)"

.PHONY: install-tools
install-tools: ## Install required tools
	@echo "$(GREEN)Installing controller-gen...$(NC)"
	$(GO) install sigs.k8s.io/controller-tools/cmd/controller-gen@latest
	@echo "$(GREEN)Tools installed$(NC)"

##@ Full Workflow

.PHONY: all
all: clean vendor codegen build ## Clean, vendor, generate code, and build

.PHONY: deploy
deploy: manifests install-crd apply-cr ## Generate manifests, install CRD, and apply CR

.PHONY: undeploy
undeploy: delete-cr uninstall-crd ## Delete CR and uninstall CRD

.PHONY: refresh
refresh: undeploy deploy ## Refresh deployment (undeploy then deploy)
