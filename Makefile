all: install

install:
	go install github.com/ernestrc/logd/cmd/logd
	go install github.com/ernestrc/logd/logging

test:
	@go test github.com/ernestrc/logd/logging

coverage:
	@ go list -f '{{if len .TestGoFiles}}"go test -coverprofile={{.Dir}}/.coverprofile {{.ImportPath}}"{{end}}' ./... | grep -v vendor | xargs -L 1 sh -c
	@ go list -f '{{if len .TestGoFiles}}"go tool cover -html={{.Dir}}/.coverprofile "{{end}}' ./... | grep -v vendor | xargs -L 1 sh -c
bench:
	@go test github.com/ernestrc/logd/logging -test.bench .

build:
	@go build github.com/ernestrc/logd/logging
