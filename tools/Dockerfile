FROM golang:1.9.1-alpine

ENV LIBRDKAFKA_VERSION 0.11.1

RUN apk add --no-cache python openssl yajl-dev zlib-dev openssl-dev build-base coreutils bash curl

RUN curl -Lk -o /root/librdkafka-${LIBRDKAFKA_VERSION}.tar.gz https://github.com/edenhill/librdkafka/archive/v${LIBRDKAFKA_VERSION}.tar.gz && \
    tar -xzf /root/librdkafka-${LIBRDKAFKA_VERSION}.tar.gz -C /root && \
    cd /root/librdkafka-${LIBRDKAFKA_VERSION} && \
    ./configure --disable-sasl --disable-ssl --enable-static && make && make install && make clean && ./configure --clean && mkdir /target

WORKDIR /target
VOLUME /target

CMD go install -tags static_all github.com/ernestrc/logd/cmd/logd && cp `which logd` /target
