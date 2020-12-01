FROM onosproject/golang-build:v0.6.6

ADD pkg/controller/config/model/modelplugin.tpl /etc/onos/codegen/modelplugin.tpl
ADD pkg/controller/config/model/module.tpl /etc/onos/codegen/module.tpl

ADD build/config-operator/_output/bin/config-operator /usr/local/bin/config-operator

ENTRYPOINT ["config-operator"]
