FROM ubuntu:18.04

# Install godep, curl
RUN echo "\
deb http://mirrors.aliyun.com/ubuntu/ bionic main restricted universe multiverse \n\
deb http://mirrors.aliyun.com/ubuntu/ bionic-security main restricted universe multiverse \n\
deb http://mirrors.aliyun.com/ubuntu/ bionic-updates main restricted universe multiverse \n\
deb http://mirrors.aliyun.com/ubuntu/ bionic-proposed main restricted universe multiverse \n\
deb http://mirrors.aliyun.com/ubuntu/ bionic-backports main restricted universe multiverse \
" > /etc/apt/sources.list

RUN apt update && apt install -y go-dep curl make git build-essential && apt clean

# Install golang environment
RUN curl https://dl.google.com/go/go1.11.10.linux-amd64.tar.gz -O && \
    mkdir /tools && \
    tar xzf go1.11.10.linux-amd64.tar.gz -C /usr/local && \
    rm go1.11.10.linux-amd64.tar.gz && \
    mkdir -p /go/bin

ENV PATH=$PATH:/usr/local/go/bin
ENV GOPATH=/go

# dependence install
RUN mkdir -p /go/src/gitlab.aibee.cn/platform/ai-scheduler

# waiting for ladder available

ADD Gopkg.lock /go/src/gitlab.aibee.cn/platform/ai-scheduler/
ADD Gopkg.toml /go/src/gitlab.aibee.cn/platform/ai-scheduler/
RUN cd /go/src/gitlab.aibee.cn/platform/ai-scheduler/ && dep ensure --vendor-only -v
