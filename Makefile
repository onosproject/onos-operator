export CGO_ENABLED=0
export GO111MODULE=on

.PHONY: build

ONOS_OPERATOR_VERSION := latest

build: # @HELP build the Go binaries and run all validations (default)
build:
	go build -o build/_output/admission-init ./cmd/admission-init
	go build -o build/_output/core-operator ./cmd/core-operator
	go build -o build/_output/config-operator ./cmd/config-operator
	go build -o build/_output/topo-operator ./cmd/topo-operator

test: # @HELP run the unit tests and source code validation
test: build deps license_check linters
	go test github.com/onosproject/onos-operator/pkg/...
	go test github.com/onosproject/onos-operator/cmd/...

coverage: # @HELP generate unit test coverage data
coverage: build deps linters license_check
	./../build-tools/build/coveralls/coveralls-coverage onos-operator

deps: # @HELP ensure that the required dependencies are in place
	go build -v ./...
	bash -c "diff -u <(echo -n) <(git diff go.mod)"
	bash -c "diff -u <(echo -n) <(git diff go.sum)"

linters: # @HELP examines Go source code and reports coding problems
	golangci-lint run

license_check: # @HELP examine and ensure license headers exist
	@if [ ! -d "../build-tools" ]; then cd .. && git clone https://github.com/onosproject/build-tools.git; fi
	./../build-tools/licensing/boilerplate.py -v --rootdir=${CURDIR}

images: # @HELP build Docker images
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o build/admission-init/_output/bin/admission-init ./cmd/admission-init
	docker build . -f build/admission-init/Dockerfile -t onosproject/config-operator-init:${ONOS_OPERATOR_VERSION}
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o build/core-operator/_output/bin/core-operator ./cmd/core-operator
	docker build . -f build/core-operator/Dockerfile -t onosproject/core-operator:${ONOS_OPERATOR_VERSION}
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o build/config-operator/_output/bin/config-operator ./cmd/config-operator
	docker build . -f build/config-operator/Dockerfile -t onosproject/config-operator:${ONOS_OPERATOR_VERSION}
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o build/topo-operator/_output/bin/topo-operator ./cmd/topo-operator
	docker build . -f build/topo-operator/Dockerfile -t onosproject/topo-operator:${ONOS_OPERATOR_VERSION}

kind: # @HELP build Docker images and add them to the currently configured kind cluster
kind: images
	@if [ "`kind get clusters`" = '' ]; then echo "no kind cluster found" && exit 1; fi
	kind load docker-image onosproject/config-operator-init:${ONOS_OPERATOR_VERSION}
	kind load docker-image onosproject/core-operator:${ONOS_OPERATOR_VERSION}
	kind load docker-image onosproject/config-operator:${ONOS_OPERATOR_VERSION}
	kind load docker-image onosproject/topo-operator:${ONOS_OPERATOR_VERSION}

all: build images

publish: # @HELP publish version on github and dockerhub
	./../build-tools/publish-version ${VERSION} onosproject/core-operator onosproject/config-operator onosproject/topo-operator

push: # @HELP push latest versions of the images to docker hub
	docker push onosproject/core-operator:latest
	docker push onosproject/config-operator:latest
	docker push onosproject/topo-operator:latest

bumponosdeps: # @HELP update "onosproject" go dependencies and push patch to git.
	./../build-tools/bump-onos-deps ${VERSION}

clean: # @HELP remove all the build artifacts
	rm -rf ./build/_output ./vendor ./cmd/dummy/dummy build/core-operator/_output build/config-operator/_output build/topo-operator/_output

help:
	@grep -E '^.*: *# *@HELP' $(MAKEFILE_LIST) \
    | sort \
    | awk ' \
        BEGIN {FS = ": *# *@HELP"}; \
        {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}; \
    '
