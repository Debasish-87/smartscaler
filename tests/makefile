ROOT := $(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))
PROJECT_ROOT := $(shell dirname $(ROOT))

.PHONY: test test-unit test-regression test-integration test-coverage test-watch help

help: 
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
	  awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

test: 
	cd $(PROJECT_ROOT) && ./tests/run_tests.sh

test-unit: 
	cd $(PROJECT_ROOT) && go test -race -count=1 -timeout=60s \
	  ./tests/pkg/decision/... \
	  ./tests/pkg/cost/... \
	  ./tests/pkg/utils/... \
	  ./tests/pkg/scaler/...

test-regression: 
	cd $(PROJECT_ROOT) && go test -race -count=1 \
	  ./tests/pkg/regression/...

test-integration: 
	cd $(PROJECT_ROOT) && go test -race -count=1 -timeout=60s \
	  ./tests/pkg/integration/...

test-coverage: 
	cd $(PROJECT_ROOT) && go test -race -count=1 -timeout=120s \
	  -coverprofile=coverage.out \
	  -covermode=atomic \
	  -coverpkg=./pkg/... \
	  ./tests/...
	cd $(PROJECT_ROOT) && go tool cover -func=coverage.out \
	  | grep -E "(total|decision|cost|scaler|utils)"
	cd $(PROJECT_ROOT) && go tool cover -html=coverage.out -o coverage.html
	@echo "coverage.html generated → xdg-open $(PROJECT_ROOT)/coverage.html"

test-watch: 
	find $(PROJECT_ROOT)/pkg $(PROJECT_ROOT)/tests -name "*.go" | \
	  entr -c make test