FROM alpine:3.8

RUN apk upgrade --update --no-cache

USER nobody

ADD build/admission-init/_output/bin/admission-init /usr/local/bin/admission-init

ENTRYPOINT ["admission-init"]
