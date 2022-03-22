# SPDX-FileCopyrightText: 2022 2020-present Open Networking Foundation <info@opennetworking.org>
#
# SPDX-License-Identifier: Apache-2.0

export CGO_ENABLED=1
export GO111MODULE=on

.PHONY: build

ONOS_OPERATOR_VERSION ?= latest
KIND_CLUSTER_NAME ?= kind

build: # @HELP build the Go binaries and run all validations (default)
build:
	go build -o build/_output/admission-init ./cmd/admission-init
	go build -o build/_output/topo-operator ./cmd/topo-operator
	go build -o build/_output/app-operator ./cmd/app-operator

build-tools:=$(shell if [ ! -d "./build/build-tools" ]; then cd build && git clone https://github.com/onosproject/build-tools.git; fi)
include ./build/build-tools/make/onf-common.mk

version_check: # @HELP verify that release versions are correct
	./build/bin/check-versions

test: # @HELP run the unit tests and source code validation
test: build deps license linters version_check
	go test github.com/onosproject/onos-operator/pkg/...
	go test github.com/onosproject/onos-operator/cmd/...

jenkins-test:  # @HELP run the unit tests and source code validation producing a junit style report for Jenkins
jenkins-test: deps license linters version_check
	TEST_PACKAGES=github.com/onosproject/onos-operator/pkg/... ./build/build-tools/build/jenkins/make-unit

images: # @HELP build Docker images
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o build/admission-init/_output/bin/admission-init ./cmd/admission-init
	docker build . -f build/admission-init/Dockerfile -t onosproject/config-operator-init:${ONOS_OPERATOR_VERSION}
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o build/topo-operator/_output/bin/topo-operator ./cmd/topo-operator
	docker build . -f build/topo-operator/Dockerfile -t onosproject/topo-operator:${ONOS_OPERATOR_VERSION}
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o build/app-operator/_output/bin/app-operator ./cmd/app-operator
	docker build . -f build/app-operator/Dockerfile -t onosproject/app-operator:${ONOS_OPERATOR_VERSION}

kind: # @HELP build Docker images and add them to the currently configured kind cluster
kind: images
	@if [ "`kind get clusters | grep ${KIND_CLUSTER_NAME}`" = '' ]; then echo "no kind cluster found" && exit 1; fi
	kind load docker-image --name ${KIND_CLUSTER_NAME} onosproject/config-operator-init:${ONOS_OPERATOR_VERSION}
	kind load docker-image --name ${KIND_CLUSTER_NAME} onosproject/topo-operator:${ONOS_OPERATOR_VERSION}
	kind load docker-image --name ${KIND_CLUSTER_NAME} onosproject/app-operator:${ONOS_OPERATOR_VERSION}

all: build images

publish: # @HELP publish version on github and dockerhub
	./build/build-tools/publish-version ${VERSION} onosproject/config-operator-init onosproject/topo-operator onosproject/app-operator

jenkins-publish: # @HELP Jenkins calls this to publish artifacts
	./build/bin/push-images
	./build/build-tools/release-merge-commit

push: # @HELP push latest versions of the images to docker hub
	docker push onosproject/config-operator-init:${ONOS_OPERATOR_VERSION}
	docker push onosproject/topo-operator:${ONOS_OPERATOR_VERSION}
	docker push onosproject/app-operator:${ONOS_OPERATOR_VERSION}

clean:: # @HELP remove all the build artifacts
	rm -rf ./build/_output ./vendor ./cmd/dummy/dummy build/admission-init/_output build/topo-operator/_output build/app-operator/_output
