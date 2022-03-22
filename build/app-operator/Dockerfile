# SPDX-FileCopyrightText: 2022 2020-present Open Networking Foundation <info@opennetworking.org>
#
# SPDX-License-Identifier: Apache-2.0

FROM alpine:3.8

RUN apk upgrade --update --no-cache

USER nobody

ADD build/app-operator/_output/bin/app-operator /usr/local/bin/app-operator

ENTRYPOINT ["app-operator"]
