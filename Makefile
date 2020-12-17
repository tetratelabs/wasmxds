# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

IMG ?= getenvoy/wasmxds:0.0.1
GO_BUILD_TAGS ?= ""
PROTOCOLS ?= local_fs s3 oci http https

# Build manager binary
build:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/manager main.go

build.wasm:
	cd e2e/providers/testdata && for p in ${PROTOCOLS}; do tinygo build -tags=$${p} -o filter.$${p}.wasm -scheduler=none -target=wasi; done

build.wasm.docker:
	docker run -it -w /tmp/wasmxds -v $(shell pwd)/e2e/providers/testdata:/tmp/wasmxds tinygo/tinygo:latest /bin/bash \
		-c 'for p in ${PROTOCOLS}; do tinygo build -tags=$${p} -o filter.$${p}.wasm -scheduler=none -target=wasi; done'

# generate crd and kustomize"d" raw configuration
codegen: controller-gen kustomize
	$(CONTROLLER_GEN) "crd:trivialVersions=true" rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=manifests/crd/bases output:rbac:artifacts:config=manifests/rbac
	cd manifests/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build manifests > manifests/wasmxds.yaml
	$(CONTROLLER_GEN) object:headerFile="LICENSE.header" paths="./..."

# run go tests (docker-compose required)
gotest:
	docker-compose up -d
	go test -tags=${GO_BUILD_TAGS} -v $(shell go list ./... | grep -Ev 'e2e|controllers' | sed 's/github.com\/tetratelabs\/wasmxds/./g')

# run e2e test for controllers (kind required)
e2e.controllers: build
	go test -tags=${GO_BUILD_TAGS} -v ./controllers/... --count=1

# run e2e test for providers (kind and docker-compose required)
e2e.providers: build
	docker-compose up -d
	kubectl apply -f manifests/crd/bases/
	go test -tags=${GO_BUILD_TAGS} -v ./e2e/providers/... --count=1

# run e2e in k8s to check the manifest and latest image works (kind required)
e2e.k8s:
	kubectl apply -f manifests/wasmxds.yaml
	go test -v ./e2e/k8s/... --count=1

# find or download controller-gen
# download controller-gen if necessary
controller-gen:
ifeq (, $(shell which controller-gen))
	@{ \
	set -e ;\
	CONTROLLER_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$CONTROLLER_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.3.0 ;\
	rm -rf $$CONTROLLER_GEN_TMP_DIR ;\
	}
CONTROLLER_GEN=$(GOBIN)/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif

kustomize:
ifeq (, $(shell which kustomize))
	@{ \
	set -e ;\
	KUSTOMIZE_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$KUSTOMIZE_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get sigs.k8s.io/kustomize/kustomize/v3@v3.5.4 ;\
	rm -rf $$KUSTOMIZE_GEN_TMP_DIR ;\
	}
KUSTOMIZE=$(GOBIN)/kustomize
else
KUSTOMIZE=$(shell which kustomize)
endif
