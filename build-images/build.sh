

REGISTRY=registry.aibee.cn
REPO=${REGISTRY}/platform/kubernetes/ai-scheduler-ci

if [ "$TAG" == "" ]; then
    TAG=1.0
fi

cp ../Gopkg.* .

# docker build
docker build \
    --build-arg=http_proxy \
    --build-arg=https_proxy \
    -t ${REPO}:${TAG}\
    -f Dockerfile .

# docker push 
docker login http://${REGISTRY}
docker push ${REPO}:${TAG}

rm ./Gopkg.*