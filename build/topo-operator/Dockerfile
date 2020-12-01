FROM alpine:3.8

RUN apk upgrade --update --no-cache

USER nobody

ADD build/topo-operator/_output/bin/topo-operator /usr/local/bin/topo-operator

ENTRYPOINT ["topo-operator"]
