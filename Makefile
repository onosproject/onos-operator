# SPDX-License-Identifier: Apache-2.0
# Copyright 2019 Open Networking Foundation
# Copyright 2024 Intel Corporation

export CGO_ENABLED=1
export GO111MODULE=on

.PHONY: build

ONOS_OPERATOR_VERSION ?= latest
KIND_CLUSTER_NAME ?= kind

GOLANG_CI_VERSION := v1.52.2

all: build docker-build

build: # @HELP build the Go binaries and run all validations (default)
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o build/_output/admission-init ./cmd/admission-init
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o build/_output/topo-operator ./cmd/topo-operator
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o build/_output/app-operator ./cmd/app-operator

test: # @HELP run the unit tests and source code validation
test: build lint license
	go test github.com/onosproject/onos-operator/pkg/...
	go test github.com/onosproject/onos-operator/cmd/...

docker-build-admission-init: # @HELP build admission-init Docker image
	docker build . -f build/admission-init/Dockerfile -t onosproject/config-operator-init:${ONOS_OPERATOR_VERSION}

docker-build-topo-operator: # @HELP build topo-operator Docker image
	docker build . -f build/topo-operator/Dockerfile -t onosproject/topo-operator:${ONOS_OPERATOR_VERSION}

docker-build-app-operator: # @HELP build app-operator Docker image
	docker build . -f build/app-operator/Dockerfile -t onosproject/app-operator:${ONOS_OPERATOR_VERSION}

docker-build: # @HELP build all Docker images
docker-build: build docker-build-admission-init docker-build-topo-operator docker-build-app-operator

docker-push-admission-init: # @HELP push admission-init Docker image
	docker push onosproject/config-operator-init:${ONOS_OPERATOR_VERSION}

docker-push-topo-operator: # @HELP push topo-operator Docker image
	docker push onosproject/topo-operator:${ONOS_OPERATOR_VERSION}

docker-push-app-operator: # @HELP push app-operator Docker image
	docker push onosproject/app-operator:${ONOS_OPERATOR_VERSION}


docker-push: # @HELP push docker images
docker-push: docker-push-admission-init docker-push-topo-operator docker-push-app-operator

lint: # @HELP examines Go source code and reports coding problems
	golangci-lint --version | grep $(GOLANG_CI_VERSION) || curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b `go env GOPATH`/bin $(GOLANG_CI_VERSION)
	golangci-lint run --timeout 15m

license: # @HELP run license checks
	rm -rf venv
	python3 -m venv venv
	. ./venv/bin/activate;\
	python3 -m pip install --upgrade pip;\
	python3 -m pip install reuse;\
	reuse lint

check-version: # @HELP check version is duplicated
	./build/bin/version_check.sh all

clean: # @HELP remove all the build artifacts
	rm -rf ./build/_output ./vendor ./cmd/dummy/dummy build/admission-init/_output build/topo-operator/_output build/app-operator/_output

help:
	@grep -E '^.*: *# *@HELP' $(MAKEFILE_LIST) \
    | sort \
    | awk ' \
        BEGIN {FS = ": *# *@HELP"}; \
        {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}; \
    '
