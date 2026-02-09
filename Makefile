# GoPlatform Makefile

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# Binary names
OPERATOR_BINARY=goplatform
CLI_BINARY=gpctl

# Build directory
BUILD_DIR=bin

# Image
IMG ?= ghcr.io/abd-ulbasit/goplatform:latest

# Kubernetes
KUBEBUILDER_VERSION ?= 3.14.0
ENVTEST_K8S_VERSION = 1.28.0

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# CONTAINER_TOOL defines the container tool to be used for building images.
CONTAINER_TOOL ?= docker

# Setting SHELL to bash allows bash commands to be executed by recipes.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

.PHONY: all
all: build

##@ General

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: manifests
manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=deploy/kubernetes/crds

.PHONY: generate
generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: lint
lint: golangci-lint ## Run golangci-lint linter
	$(GOLANGCI_LINT) run

.PHONY: test
test: manifests generate fmt vet envtest ## Run tests.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" go test ./... -coverprofile cover.out

.PHONY: test-e2e
test-e2e: ## Run end-to-end tests
	go test ./test/e2e/... -v

##@ Build

.PHONY: build
build: manifests generate fmt vet ## Build operator binary.
	$(GOBUILD) -o $(BUILD_DIR)/$(OPERATOR_BINARY) ./cmd/goplatform

.PHONY: build-cli
build-cli: ## Build CLI binary.
	$(GOBUILD) -o $(BUILD_DIR)/$(CLI_BINARY) ./cmd/gpctl

.PHONY: build-all
build-all: build build-cli ## Build all binaries.

.PHONY: run
run: manifests generate fmt vet ## Run a controller from your host.
	go run ./cmd/goplatform

.PHONY: docker-build
docker-build: ## Build docker image with the operator.
	$(CONTAINER_TOOL) build -t ${IMG} .

.PHONY: docker-push
docker-push: ## Push docker image with the operator.
	$(CONTAINER_TOOL) push ${IMG}

##@ Deployment

ifndef ignore-not-found
  ignore-not-found = false
endif

.PHONY: install
install: manifests ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	kubectl apply -f deploy/kubernetes/crds/

.PHONY: uninstall
uninstall: manifests ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config.
	kubectl delete --ignore-not-found=$(ignore-not-found) -f deploy/kubernetes/crds/

.PHONY: deploy
deploy: manifests ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	helm install goplatform deploy/helm/goplatform

.PHONY: undeploy
undeploy: ## Undeploy controller from the K8s cluster specified in ~/.kube/config.
	helm uninstall goplatform

##@ Dependencies

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

## Tool Binaries
KUBECTL ?= kubectl
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
ENVTEST ?= $(LOCALBIN)/setup-envtest
GOLANGCI_LINT = $(LOCALBIN)/golangci-lint

## Tool Versions
CONTROLLER_TOOLS_VERSION ?= v0.14.0
GOLANGCI_LINT_VERSION ?= v1.56.2

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary.
$(CONTROLLER_GEN): $(LOCALBIN)
	$(call go-install-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen,$(CONTROLLER_TOOLS_VERSION))

.PHONY: envtest
envtest: $(ENVTEST) ## Download setup-envtest locally if necessary.
$(ENVTEST): $(LOCALBIN)
	$(call go-install-tool,$(ENVTEST),sigs.k8s.io/controller-runtime/tools/setup-envtest,latest)

.PHONY: golangci-lint
golangci-lint: $(GOLANGCI_LINT) ## Download golangci-lint locally if necessary.
$(GOLANGCI_LINT): $(LOCALBIN)
	$(call go-install-tool,$(GOLANGCI_LINT),github.com/golangci/golangci-lint/cmd/golangci-lint,$(GOLANGCI_LINT_VERSION))

# go-install-tool will 'go install' any package with custom target and target path.
define go-install-tool
@[ -f $(1) ] || { \
set -e; \
package=$(2)@$(3); \
echo "Downloading $${package}"; \
GOBIN=$(LOCALBIN) go install $${package}; \
}
endef

##@ Terraform

.PHONY: tf-init
tf-init: ## Initialize Terraform modules
	cd deploy/terraform/modules && terraform init

.PHONY: tf-validate
tf-validate: ## Validate Terraform modules
	cd deploy/terraform/modules && terraform validate

##@ Local Development

.PHONY: kind-create
kind-create: ## Create a kind cluster for local development
	kind create cluster --name goplatform --config hack/kind-config.yaml

.PHONY: kind-delete
kind-delete: ## Delete the kind cluster
	kind delete cluster --name goplatform

.PHONY: kind-load
kind-load: docker-build ## Load docker image into kind cluster
	kind load docker-image ${IMG} --name goplatform

##@ Clean

.PHONY: clean
clean: ## Clean build artifacts
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)
	rm -f cover.out
