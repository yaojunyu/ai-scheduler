BIN_DIR=_output/bin
MAJOR_VER=0
MINOR_VER=3
RELEASE_VER=v0.3.1
REPO_PATH=gitlab.aibee.cn/platform/ai-scheduler
GitTreeState="clean"
GitSHA=`git rev-parse HEAD`
Date=`date "+%Y-%m-%d %H:%M:%S"`
REL_OSARCH="linux/amd64"
IMAGE_PREFIX=registry.aibee.cn/platform/kubernetes
LD_FLAGS=" \
	-X '${REPO_PATH}/pkg/version.gitMajor=${MAJOR_VER}' \
	-X '${REPO_PATH}/pkg/version.gitMinor=${MINOR_VER}' \
	-X '${REPO_PATH}/pkg/version.gitTreeState=${GitTreeState}' \
    -X '${REPO_PATH}/pkg/version.gitCommit=${GitSHA}' \
    -X '${REPO_PATH}/pkg/version.buildDate=${Date}'   \
    -X '${REPO_PATH}/pkg/version.gitVersion=${RELEASE_VER}'"

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
