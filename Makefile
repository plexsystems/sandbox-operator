KUBERNETES_VERSION=v1.14.10
CLUSTER_NAME=operator-testing-$(KUBERNETES_VERSION)
OPERATOR_IMAGE=sandbox-operator:dev

.PHONY: image
image:
	docker build . -t $(OPERATOR_IMAGE)

.PHONY: cluster
cluster:
	kind create cluster --name $(CLUSTER_NAME) --image kindest/node:$(KUBERNETES_VERSION)
	kubectl wait --for=condition=Ready --timeout=60s node --all

.PHONY: deploy
deploy: image
	kind load docker-image $(OPERATOR_IMAGE) --name $(CLUSTER_NAME)
	kubectl delete pod --all
	kustomize build example | kubectl apply -f -
	kubectl wait --for=condition=Ready --timeout=60s pods --all

.PHONY: lint
lint:
	kustomize build example | kubeval --ignore-missing-schemas -

.PHONY: test-unit
test-unit: 
	go test ./controller -v -count=1

.PHONY: test-integration
test-integration: cluster deploy
	go test ./controller -v --tags=integration -count=1

.PHONY: destroy
destroy:
	kind delete cluster --name $(CLUSTER_NAME)
