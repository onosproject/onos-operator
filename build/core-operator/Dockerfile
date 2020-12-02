FROM alpine:3.8

RUN apk upgrade --update --no-cache

USER nobody

ADD build/core-operator/_output/bin/core-operator /usr/local/bin/core-operator

ENTRYPOINT ["core-operator"]
