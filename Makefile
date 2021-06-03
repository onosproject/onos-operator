export CGO_ENABLED=1
export GO111MODULE=on

.PHONY: build

ONOS_OPERATOR_VERSION ?= latest

build: # @HELP build the Go binaries and run all validations (default)
build:
	go build -o build/_output/admission-init ./cmd/admission-init
	go build -o build/_output/config-operator ./cmd/config-operator
	go build -o build/_output/topo-operator ./cmd/topo-operator

version_check: # @HELP verify that release versions are correct
	./build/bin/check-versions

test: # @HELP run the unit tests and source code validation
test: build deps license_check linters version_check
	go test github.com/onosproject/onos-operator/pkg/...
	go test github.com/onosproject/onos-operator/cmd/...

jenkins-test:  # @HELP run the unit tests and source code validation producing a junit style report for Jenkins
jenkins-test: build-tools deps license_check linters version_check
	TEST_PACKAGES=github.com/onosproject/onos-operator/pkg/... ./../build-tools/build/jenkins/make-unit

coverage: # @HELP generate unit test coverage data
coverage: build deps linters license_check
	./../build-tools/build/coveralls/coveralls-coverage onos-operator

deps: # @HELP ensure that the required dependencies are in place
	go build -v ./...
	bash -c "diff -u <(echo -n) <(git diff go.mod)"
	bash -c "diff -u <(echo -n) <(git diff go.sum)"

linters: golang-ci # @HELP examines Go source code and reports coding problems
	golangci-lint run --timeout 5m

build-tools: # @HELP install the ONOS build tools if needed
	@if [ ! -d "../build-tools" ]; then cd .. && git clone https://github.com/onosproject/build-tools.git; fi

jenkins-tools: # @HELP installs tooling needed for Jenkins
	cd .. && go get -u github.com/jstemmer/go-junit-report && go get github.com/t-yuki/gocover-cobertura

golang-ci: # @HELP install golang-ci if not present
	golangci-lint --version || curl -sfL https://install.goreleaser.com/github.com/golangci/golangci-lint.sh | sh -s -- -b `go env GOPATH`/bin v1.36.0

license_check: build-tools # @HELP examine and ensure license headers exist
	./../build-tools/licensing/boilerplate.py -v --rootdir=${CURDIR}/apis --skipped-dir pkg/clientset

images: # @HELP build Docker images
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o build/admission-init/_output/bin/admission-init ./cmd/admission-init
	docker build . -f build/admission-init/Dockerfile -t onosproject/config-operator-init:${ONOS_OPERATOR_VERSION}
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o build/config-operator/_output/bin/config-operator ./cmd/config-operator
	docker build . -f build/config-operator/Dockerfile -t onosproject/config-operator:${ONOS_OPERATOR_VERSION}
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o build/topo-operator/_output/bin/topo-operator ./cmd/topo-operator
	docker build . -f build/topo-operator/Dockerfile -t onosproject/topo-operator:${ONOS_OPERATOR_VERSION}

kind: # @HELP build Docker images and add them to the currently configured kind cluster
kind: images
	@if [ "`kind get clusters`" = '' ]; then echo "no kind cluster found" && exit 1; fi
	kind load docker-image onosproject/config-operator-init:${ONOS_OPERATOR_VERSION}
	kind load docker-image onosproject/config-operator:${ONOS_OPERATOR_VERSION}
	kind load docker-image onosproject/topo-operator:${ONOS_OPERATOR_VERSION}

all: build images

publish: # @HELP publish version on github and dockerhub
	./../build-tools/publish-version ${VERSION} onosproject/config-operator-init onosproject/config-operator onosproject/topo-operator

jenkins-publish: build-tools jenkins-tools # @HELP Jenkins calls this to publish artifacts
	./build/bin/push-images
	../build-tools/release-merge-commit

push: # @HELP push latest versions of the images to docker hub
	docker push onosproject/config-operator-init:${ONOS_OPERATOR_VERSION}
	docker push onosproject/config-operator:${ONOS_OPERATOR_VERSION}
	docker push onosproject/topo-operator:${ONOS_OPERATOR_VERSION}

bumponosdeps: # @HELP update "onosproject" go dependencies and push patch to git.
	./../build-tools/bump-onos-deps ${VERSION}

clean: # @HELP remove all the build artifacts
	rm -rf ./build/_output ./vendor ./cmd/dummy/dummy build/admission-init/_output build/config-operator/_output build/topo-operator/_output

help:
	@grep -E '^.*: *# *@HELP' $(MAKEFILE_LIST) \
    | sort \
    | awk ' \
        BEGIN {FS = ": *# *@HELP"}; \
        {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}; \
    '
