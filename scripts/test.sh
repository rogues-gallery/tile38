#!/bin/bash

set -e
cd $(dirname "${BASH_SOURCE[0]}")/..

export CGO_ENABLED=0

if [ "$NOMODULES" != "1" ]; then
	export GO111MODULE=on
	export GOFLAGS=-mod=vendor
fi

cd tests && go test && cd ..
go test $(go list ./... | grep -v /vendor/ | grep -v /tests)
