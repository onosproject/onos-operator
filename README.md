# Kubernetes Operator for µONOS

This project provides a set of [Kubernetes operators][Operator pattern] for managing components of the µONOS
architecture. µONOS operators extend the Kubernetes API with [custom resources] and integrate µONOS subsystems
with the Kubernetes control plane.

To install the µONOS operator:

```bash
> kubectl create -f https://raw.githubusercontent.com/onosproject/onos-operator/master/deploy/onos-operator.yaml
customresourcedefinition.apiextensions.k8s.io/models.config.onosproject.org created
customresourcedefinition.apiextensions.k8s.io/modelregistries.config.onosproject.org created
customresourcedefinition.apiextensions.k8s.io/services.topo.onosproject.org created
customresourcedefinition.apiextensions.k8s.io/entities.topo.onosproject.org created
customresourcedefinition.apiextensions.k8s.io/relations.topo.onosproject.org created
customresourcedefinition.apiextensions.k8s.io/kinds.topo.onosproject.org created
serviceaccount/onos-operator created
clusterrole.rbac.authorization.k8s.io/onos-operator created
clusterrolebinding.rbac.authorization.k8s.io/onos-operator created
deployment.apps/config-operator created
mutatingwebhookconfiguration.admissionregistration.k8s.io/config-operator created
service/config-operator created
deployment.apps/topo-operator created
configmap/onos-operator-config created
```

The operator consists of a `topo-operator` pod and a `config-operator` pod which will be installed in the 
`kube-system` namespace by default.

```bash
> kubectl get pods -n kube-system
NAME                                         READY   STATUS    RESTARTS   AGE
config-operator-56dc64df8d-8xwkm             1/1     Running   0          25s
topo-operator-6f555cb86-b94kp                1/1     Running   0          24s
```

## Topology operator

The topology operator extends the Kubernetes API with custom resources for defining µONOS topology objects. Topology
resources are propagated from the Kubernetes API to the [onos-topo] service via the [onos-api]. When a topology resource
is created, the topology operator adds the object to µONOS topology. When a topology resource is deleted, the operator
will remove the associated object from the µONOS topology.

### Kind

To define a topology object kind, create a `Kind` resource:

```yaml
apiVersion: topo.onosproject.org/v1beta1
kind: Kind
metadata:
  name: e2-node
spec:
  attributes:
    foo: bar
```

### Entity

To define a topology entity, create an `Entity` resource:

```yaml
apiVersion: topo.onosproject.org/v1beta1
kind: Entity
metadata:
  name: e2-node-1
spec:
  kind:
    name: e2-node
  attributes:
    baz: foo
```

### Relation

To define a topology relation, create a `Relation` resource connecting a `source` and `target` entity:

```yaml
apiVersion: topo.onosproject.org/v1beta1
kind: Relation
metadata:
  name: e2-node-1-e2t-1
spec:
  kind:
    name: e2-connection
  source:
    name: e2-node-1
  target:
    name: e2t-1
```

### Dynamic topology management

The topology operator supports dynamic entity sets with Kubernetes label selectors using the `Service` resource:

```yaml
apiVersion: topo.onosproject.org/v1beta1
kind: Service
metadata:
  name: my-app
spec:
  selector:
    matchLabels:
      name: my-app
  kind:
    name: my-app-node
```

The operator will automatically populate the µONOS topology with an entity for each pod matching the service's label
selector. This allows dynamic/autoscaling Kubernetes components like `ReplicaSet`s to be represented as dynamic
objects in the µONOS topology.

## Config operator

The config operator extends the Kubernetes API, adding a custom `Model` resource for defining config (YANG) models.
The config operator automatically injects configured `Model` resources into [onos-config] pods, compiling plugins
on the fly.

```yaml
apiVersion: config.onosproject.org/v1beta1
kind: Model
metadata:
  name: ric
spec:
  plugin:
    type: ric
    version: 1.0.0
  modules:
  - name: test1
    version: 2020-11-18
    file: test1@2020-11-18.yang
  files:
    test1@2020-11-18.yang: |
      module test1 {
        namespace "http://opennetworking.org/oran/test1";
        prefix t1;

        organization
          "Open Networking Foundation.";
        contact
          "Adib Rastegarnia";
        description
          "To generate JSON from this use command
           pyang -f jtoxx test1.yang | python3 -m json.tool > test1.json
           Copied from YangUIComponents project";

        revision 2020-11-18 {
          description
            "Extended with new attributes on leaf2d, list2b";
          reference
            "RFC 6087";
        }

        container cont1a {
          description
            "The top level container";
          leaf leaf1a {
            type string {
              length "1..80";
            }
            description
              "display name to use in GUI or CLI";
          }
          leaf leaf2a {
            type string {
              length "1..255";
            }
            description
              "user plane name";
          }
        }
      }
```

[Operator pattern]: https://kubernetes.io/docs/concepts/extend-kubernetes/operator/
[custom resources]: https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/
[onos-api]: https://github.com/onosproject/onos-api
[onos-topo]: https://github.com/onosproject/onos-topo
[onos-config]: https://github.com/onosproject/onos-config
