# SPDX-FileCopyrightText: 2022 2020-present Open Networking Foundation <info@opennetworking.org>
#
# SPDX-License-Identifier: Apache-2.0

FROM alpine:3.8

RUN apk upgrade --update --no-cache

USER nobody

ADD build/_output/topo-operator /usr/local/bin/topo-operator

ENTRYPOINT ["topo-operator"]
