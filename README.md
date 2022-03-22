<!--
SPDX-FileCopyrightText: 2022 2020-present Open Networking Foundation <info@opennetworking.org>

SPDX-License-Identifier: Apache-2.0
-->

# Kubernetes Operator for µONOS

This project provides a set of [Kubernetes operators][Operator pattern] for managing components of the µONOS
architecture. µONOS operators extend the Kubernetes API with [custom resources] and integrate µONOS subsystems
with the Kubernetes control plane.

To install the µONOS operator you can use Helm as follows:

```bash
> helm install -n kube-system onos-operator onosproject/onos-operator --wait
NAME: onos-operator
LAST DEPLOYED: Tue Oct 12 20:02:04 2021
NAMESPACE: kube-system
STATUS: deployed
REVISION: 1
TEST SUITE: None
```
The operator consists of a `topo-operator` pod and `app-operator` pod, all of which will be installed in the 
`kube-system` namespace by default.

```bash
> kubectl get pods -n kube-system
NAME                                              READY   STATUS    RESTARTS   AGE
onos-operator-app-585d588d5c-ndvkr                1/1     Running   0          42m39s
onos-operator-topo-7ff4df6f57-6p8dv               1/1     Running   0          42m39s
```

## App Operator
The application operator registers a mutating admission webhook to intercept pod deployment requests. These
requests are inspected for presence of `proxy.onosproject.org/inject` metadata annotation. If this
annotation is present and its value is `true`, the deployment request will be augmented to include a
sidecar `onosproject/onos-proxy` container as part of the pod.

For more information about see [onos-proxy].

## Topology Operator

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

[Operator pattern]: https://kubernetes.io/docs/concepts/extend-kubernetes/operator/
[custom resources]: https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/
[onos-api]: https://github.com/onosproject/onos-api
[onos-topo]: https://github.com/onosproject/onos-topo
[onos-config]: https://github.com/onosproject/onos-config
[onos-proxy]: https://github.com/onosproject/onos-proxy
