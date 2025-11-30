.PHONY: help cluster-create cluster-delete cluster-load deploy-flux deploy-platform deploy-workloads install uninstall clean

CLUSTER_NAME ?= spectre-demo
CLUSTER_CONTEXT = kind-$(CLUSTER_NAME)
KIND_VERSION ?= v0.20.0
FLUX_VERSION ?= 2.2.0

help:
	@echo "Demo Kubernetes Platform Management"
	@echo ""
	@echo "Available targets:"
	@echo "  make cluster-create         - Create a 5-node Kind cluster"
	@echo "  make cluster-delete         - Delete the Kind cluster"
	@echo "  make deploy-flux            - Install Flux on the cluster"
	@echo "  make deploy-platform        - Install platform components (cert-manager, external-secrets, vault, kyverno)"
	@echo "  make deploy-workloads       - Deploy demo workloads"
	@echo "  make install                - Create cluster, deploy flux, platform, and workloads"
	@echo "  make uninstall              - Delete all resources and cluster"
	@echo "  make clean                  - Clean up local files"
	@echo ""
	@echo "Variables:"
	@echo "  CLUSTER_NAME (default: spectre-demo)"
	@echo "  KIND_VERSION (default: v0.20.0)"
	@echo "  FLUX_VERSION (default: 2.2.0)"

.check-kind:
	@which kind > /dev/null || (echo "kind not found. Please install kind: https://kind.sigs.k8s.io/docs/user/quick-start/" && exit 1)
	@which kubectl > /dev/null || (echo "kubectl not found. Please install kubectl" && exit 1)
	@which flux > /dev/null || (echo "flux CLI not found. Please install flux: https://fluxcd.io/flux/installation/" && exit 1)

cluster-create: .check-kind
	@echo "Creating Kind cluster '$(CLUSTER_NAME)' with 5 nodes..."
		kind create cluster --name $(CLUSTER_NAME) --config config/kind.yaml
	@echo "Cluster created successfully!"
	@echo "To use the cluster, run:"
	@echo "  kubectl cluster-info --context $(CLUSTER_CONTEXT)"
	@kubectl cluster-info --context $(CLUSTER_CONTEXT)

cluster-delete:
	@echo "Deleting Kind cluster '$(CLUSTER_NAME)'..."
	kind delete cluster --name $(CLUSTER_NAME)
	@echo "Cluster deleted successfully!"

deploy-flux: .check-kind
	@echo "Installing Flux on the cluster..."
	kubectl apply -f flux/flux-install.yaml
	@echo "Flux installation complete!"

deploy-platform: deploy-flux
	@echo "Deploying platform components..."
	kubectl create namespace platform --dry-run=client -o yaml | kubectl apply -f -
	kubectl apply -f flux/helmrepo/ -f flux/helmrelease/
	@echo "Platform components deployed!"

deploy-workloads:
	@echo "Deploying demo workloads..."
	kubectl create namespace workloads --dry-run=client -o yaml | kubectl apply -f -
	kubectl apply -f workloads/
	@echo "Workloads deployed!"
	@echo ""
	@echo "View deployments: kubectl get deployments -n workloads"
	@echo "View pods: kubectl get pods -n workloads"

install: cluster-create deploy-platform deploy-workloads
	@echo ""
	@echo "✓ Demo Kubernetes platform is ready!"
	@echo ""
	@echo "To access the cluster:"
	@echo "  kubectl cluster-info --context $(CLUSTER_CONTEXT)"
	@echo "  kubectl get nodes --context $(CLUSTER_CONTEXT)"
	@echo ""
	@echo "To monitor Flux:"
	@echo "  flux logs --context $(CLUSTER_CONTEXT) --follow"
	@echo ""
	@echo "To see workloads:"
	@echo "  kubectl get deployments,pods -n workloads --context $(CLUSTER_CONTEXT)"

uninstall: cluster-delete clean
	@echo "Demo platform uninstalled!"

clean:
	@echo "Cleaning up local files..."
	@rm -f flux/flux-install.yaml
	@echo "Done!"

.PHONY: help .check-kind cluster-create cluster-delete deploy-flux deploy-platform deploy-workloads install uninstall clean
