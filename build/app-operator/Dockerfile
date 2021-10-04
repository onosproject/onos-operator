FROM alpine:3.8

RUN apk upgrade --update --no-cache

USER nobody

ADD build/app-operator/_output/bin/app-operator /usr/local/bin/app-operator

ENTRYPOINT ["app-operator"]
