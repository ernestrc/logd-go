all: install

install:
	go install github.com/opentok/blue/cmd/smeagol
	go install github.com/opentok/blue/logging

test:
	go test github.com/opentok/blue/logging

bench:
	go test github.com/opentok/blue/logging -test.bench .

build:
	go build github.com/opentok/blue/logging
