#!/usr/bin/env bash
set -e

GIT_PATH=github.com/etcd-io/etcd

USER_NAME=etcd-io
#BRANCH_NAME=release-3.2
BRANCH_NAME=release-3.5
#BRANCH_NAME=master

<<COMMENT
USER_NAME=gyuho
BRANCH_NAME=new-balancer-april-2018
COMMENT

rm -rf ${GOPATH}/src/${GIT_PATH}
mkdir -p ${GOPATH}/src/github.com/etcd-io

git clone https://github.com/${USER_NAME}/etcd \
  --branch ${BRANCH_NAME} \
  ${GOPATH}/src/${GIT_PATH}

cd ${GOPATH}/src/${GIT_PATH}

<<COMMENT
git reset --hard 67b1ff6724637f0a00f693471ddb17b5adde38cf
COMMENT

sudo apt install make
make build

${GOPATH}/src/${GIT_PATH}/bin/etcd --version
${GOPATH}/src/${GIT_PATH}/bin/etcdctl version

mkdir -p ${GOPATH}/bin
cp ${GOPATH}/src/${GIT_PATH}/bin/etcd ${GOPATH}/bin/etcd
sudo cp ${GOPATH}/src/${GIT_PATH}/bin/etcd /etcd

cp ${GOPATH}/src/${GIT_PATH}/bin/etcdctl ${GOPATH}/bin/etcdctl
sudo cp ${GOPATH}/src/${GIT_PATH}/bin/etcdctl /etcdctl

${GOPATH}/bin/etcd --version
ETCDCTL_API=3 ${GOPATH}/bin/etcdctl version
ETCDCTL_API=3 etcdctl version

