BUILD_IMAGE=logd-build:latest
TARGET=$(PWD)/target
CONT_TARGET=/target
SRC=$(TARGET)/src
CONT_SRC=/usr/local/go/src/github.com/ernestrc/logd/

all: install

install:
	go install github.com/ernestrc/logd/cmd/logd
	go install github.com/ernestrc/logd/logging
	go install github.com/ernestrc/logd/lua
	go install github.com/ernestrc/logd/http

test:
	go test github.com/ernestrc/logd/logging
	go test github.com/ernestrc/logd/lua
	go test github.com/ernestrc/logd/http

coverage:
	go list -f '{{if len .TestGoFiles}}"go test -coverprofile={{.Dir}}/.coverprofile {{.ImportPath}}"{{end}}' ./... | grep -v vendor | xargs -L 1 sh -c
	go list -f '{{if len .TestGoFiles}}"go tool cover -html={{.Dir}}/.coverprofile "{{end}}' ./... | grep -v vendor | xargs -L 1 sh -c

bench:
	go test github.com/ernestrc/logd/logging -test.bench .
	go test github.com/ernestrc/logd/lua -test.bench .
	go test github.com/ernestrc/logd/http -test.bench .

build:
	go build github.com/ernestrc/logd/logging
	go build github.com/ernestrc/logd/lua
	go build github.com/ernestrc/logd/http

$(TARGET):
	@mkdir $(TARGET)

static: $(TARGET)
	@ mkdir -p $(SRC)
	@ cp -R ./logging $(SRC)
	@ cp -R ./lua $(SRC)
	@ cp -R ./http $(SRC)
	@ cp -R ./cmd $(SRC)
	@ cp -R ./vendor $(SRC)
	@ [ ! -z $(docker images -q $(BUILD_IMAGE)) ] || docker build -t $(BUILD_IMAGE) ./tools/
	docker run --rm -v $(SRC):$(CONT_SRC) -v $(TARGET):$(CONT_TARGET) $(BUILD_IMAGE)

clean:
	@rm -rf $(TARGET)
