all: install

install:
	go install github.com/opentok/blue/cmd/otc

test: build
	go test github.com/opentok/blue/logging

build:
	go build github.com/opentok/blue/logging
