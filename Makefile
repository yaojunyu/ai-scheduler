BIN_DIR=_output/bin
RELEASE_VER=v0.1.0-alpha0
REPO_PATH=gitlabe.aibee.cn/platform/ai-scheduler
GitSHA=`git rev-parse HEAD`
Date=`date "+%Y-%m-%d %H:%M:%S"`
REL_OSARCH="linux/amd64"
IMAGE_PREFIX=registry.aibee.cn/platform/kubernetes
LD_FLAGS=" \
    -X '${REPO_PATH}/pkg/version.GitSHA=${GitSHA}' \
    -X '${REPO_PATH}/pkg/version.Built=${Date}'   \
    -X '${REPO_PATH}/pkg/version.Version=${RELEASE_VER}'"

.EXPORT_ALL_VARIABLES:

all: ai-scheduler

ai-scheduler: init
	go build -ldflags ${LD_FLAGS} -o=${BIN_DIR}/ai-scheduler ./cmd/ai-scheduler

verify: generate-code
	hack/verify-gofmt.sh
	hack/verify-golint.sh
	hack/verify-gencode.sh
	hack/verify-boilerplate.sh
	hack/verify-spelling.sh

init:
	mkdir -p ${BIN_DIR}

rel_bins:
	go get github.com/mitchellh/gox
	# Build ai-scheduler binary
	CGO_ENABLED=0 gox -osarch=${REL_OSARCH} -ldflags ${LD_FLAGS} \
	-output=${BIN_DIR}/{{.OS}}/{{.Arch}}/ai-scheduler ./cmd/ai-scheduler

images: rel_bins
	# Build ai-scheduler images
	cp ${BIN_DIR}/${REL_OSARCH}/ai-scheduler ./deployment/images/
	docker build ./deployment/images -t $(IMAGE_PREFIX)/ai-scheduler:${RELEASE_VER}
	rm -f ./deployment/images/ai-scheduler

push: images
	# Push images
	docker push $(IMAGE_PREFIX)/ai-scheduler:${RELEASE_VER}

run-test:
	hack/make-rules/test.sh $(WHAT) $(TESTS)

e2e: ai-scheduler
	hack/run-e2e.sh

generate-code:
	./hack/update-gencode.sh

coverage:
	KUBE_COVER=y hack/make-rules/test.sh $(WHAT) $(TESTS)

clean:
	rm -rf _output/
	rm -f ai-scheduler