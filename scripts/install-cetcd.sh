#!/usr/bin/env bash
set -e

go get -v github.com/etcd-io/cetcd/cmd/cetcd
cetcd -h
