module github.com/onosproject/onos-operator

go 1.13

require (
	github.com/onosproject/onos-api/go v0.7.6
	github.com/onosproject/onos-config-model v0.1.2
	github.com/onosproject/onos-lib-go v0.7.3
	github.com/rogpeppe/go-internal v1.3.0
	github.com/stretchr/testify v1.7.0
	google.golang.org/grpc v1.33.2
	k8s.io/api v0.17.3
	k8s.io/apimachinery v0.17.3
	k8s.io/client-go v0.17.3
	sigs.k8s.io/controller-runtime v0.5.2
)

replace github.com/onosproject/onos-config-model => github.com/kuujo/onos-config-model v0.0.2-0.20210304064049-85862e92ff2a
