VERSION    ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT     ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
IMAGE      ?= smartscaler
TAG        ?= $(VERSION)

LDFLAGS := -s -w \
	-X main.version=$(VERSION) \
	-X main.commit=$(COMMIT) \
	-X main.buildDate=$(BUILD_DATE)

.PHONY: help build test lint docker-build docker-push install uninstall \
        observe stress fmt vet tidy

help: 
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
	  awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

#  Go 

build: 
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o bin/smartscaler ./cmd

test: 
	go test -race -count=1 -timeout=60s ./...

lint: 
	golangci-lint run ./...

fmt: 
	gofmt -s -w .

vet: 
	go vet ./...

tidy: 
	go mod tidy

#  Docker 

docker-build: 
	docker build \
	  --build-arg VERSION=$(VERSION) \
	  --build-arg COMMIT=$(COMMIT) \
	  --build-arg BUILD_DATE=$(BUILD_DATE) \
	  -t $(IMAGE):$(TAG) \
	  -t $(IMAGE):latest \
	  .

docker-push: docker-build 
	docker push $(IMAGE):$(TAG)
	docker push $(IMAGE):latest

minikube-load: docker-build 
	eval $$(minikube docker-env) && \
	docker build \
	  --build-arg VERSION=$(VERSION) \
	  --build-arg COMMIT=$(COMMIT) \
	  -t $(IMAGE):$(TAG) .

#  Kubernetes 

install: 
	kubectl apply -f deploy/crd/crd.yaml
	kubectl wait --for=condition=Established \
	  crd/smartscalers.autoscale.mycompany --timeout=30s
	kubectl apply -f deploy/rbac/rbac.yaml
	kubectl apply -f deploy/operator/operator.yaml
	kubectl rollout status -n kube-system deployment/smartscaler-operator --timeout=120s
	kubectl apply -f deploy/workloads/deployment.yaml
	kubectl apply -f deploy/samples/scaler.yaml

uninstall: 
	kubectl delete -f deploy/samples/scaler.yaml      --ignore-not-found
	kubectl delete -f deploy/workloads/deployment.yaml --ignore-not-found
	kubectl delete -f deploy/operator/operator.yaml   --ignore-not-found
	kubectl delete -f deploy/rbac/rbac.yaml           --ignore-not-found
	kubectl delete -f deploy/crd/crd.yaml             --ignore-not-found

restart: 
	kubectl rollout restart -n kube-system deployment/smartscaler-operator
	kubectl rollout status  -n kube-system deployment/smartscaler-operator --timeout=60s

#  Dev tools 

observe: 
	./scripts/observe.sh

stress: 
	./scripts/stress.sh $(CPU) $(DURATION)

port-forward: 
	kubectl port-forward -n kube-system svc/smartscaler-metrics 8080:8080

setup: 
	./scripts/setup.sh

clean: 
	./scripts/setup.sh clean
