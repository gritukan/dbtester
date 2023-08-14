#!/usr/bin/env bash
set -e

go get -v github.com/etcd-io/zetcd/cmd/zetcd
zetcd -h
