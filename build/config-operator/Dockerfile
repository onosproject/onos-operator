FROM alpine:3.8

RUN apk upgrade --update --no-cache

USER nobody

ADD build/config-operator/_output/bin/config-operator /usr/local/bin/config-operator

ENTRYPOINT ["config-operator"]
