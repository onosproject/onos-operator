FROM alpine:3.8

RUN apk upgrade --update --no-cache

USER nobody

ADD build/onos-operator/_output/bin/onos-operator /usr/local/bin/onos-operator

ENTRYPOINT ["onos-operator"]
