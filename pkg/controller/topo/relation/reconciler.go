// Copyright 2019-present Open Networking Foundation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package relation

import (
	"context"
	"github.com/onosproject/onos-api/go/onos/topo"
	"github.com/onosproject/onos-lib-go/pkg/errors"
	"github.com/onosproject/onos-lib-go/pkg/logging"
	"github.com/onosproject/onos-operator/pkg/apis/topo/v1beta1"
	"github.com/onosproject/onos-operator/pkg/controller/util/grpc"
	"github.com/onosproject/onos-operator/pkg/controller/util/k8s"
	"google.golang.org/grpc/status"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logging.GetLogger("controller", "topo", "relation")

const topoService = "onos-topo"
const topoFinalizer = "topo"

// Add creates a new Relation controller and adds it to the Manager. The Manager will set fields on the
// controller and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	r := &Reconciler{
		client: mgr.GetClient(),
		scheme: mgr.GetScheme(),
		config: mgr.GetConfig(),
	}

	// Create a new controller
	c, err := controller.New("topo-relation-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource Relation
	err = c.Watch(&source.Kind{Type: &v1beta1.Relation{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for changes to secondary resource Kind and requeue the associated relations
	err = c.Watch(&source.Kind{Type: &v1beta1.Kind{}}, &handler.EnqueueRequestsFromMapFunc{
		ToRequests: &kindMapper{mgr.GetClient()},
	})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &Reconciler{}

// Reconciler reconciles a Relation object
type Reconciler struct {
	client client.Client
	scheme *runtime.Scheme
	config *rest.Config
}

// Reconcile reads that state of the cluster for a Relation object and makes changes based on the state read
// and what is in the Relation.Spec
func (r *Reconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	log.Infof("Reconciling Relation %s/%s", request.Namespace, request.Name)

	// Fetch the Relation instance
	relation := &v1beta1.Relation{}
	err := r.client.Get(context.TODO(), request.NamespacedName, relation)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	if relation.DeletionTimestamp == nil {
		return r.reconcileCreate(relation)
	}
	return r.reconcileDelete(relation)
}

func (r *Reconciler) reconcileCreate(relation *v1beta1.Relation) (reconcile.Result, error) {
	// If the relation's Kind is available, set the Kind as the relation's owner
	kind := &v1beta1.Kind{}
	name := types.NamespacedName{
		Namespace: relation.Namespace,
		Name:      relation.Spec.Kind.Name,
	}
	err := r.client.Get(context.TODO(), name, kind)
	if err != nil && !k8serrors.IsNotFound(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		err := controllerutil.SetOwnerReference(kind, relation, r.scheme)
		if err != nil {
			return reconcile.Result{}, err
		}

		err = r.client.Update(context.TODO(), relation)
		if err != nil {
			if k8serrors.IsNotFound(err) {
				return reconcile.Result{}, nil
			}
			return reconcile.Result{}, err
		}
		return reconcile.Result{}, nil
	}

	// Add the finalizer to the relation if necessary
	if !k8s.HasFinalizer(relation, topoFinalizer) {
		k8s.AddFinalizer(relation, topoFinalizer)
		err := r.client.Update(context.TODO(), relation)
		if err != nil {
			return reconcile.Result{}, err
		}
	}

	// Connect to the topology service
	conn, err := grpc.ConnectService(r.client, relation.Namespace, topoService)
	if err != nil {
		return reconcile.Result{}, err
	}
	defer conn.Close()

	client := topo.NewTopoClient(conn)

	// Check if the relation exists in the topology and exit reconciliation if so
	if exists, err := r.relationExists(relation, client); err != nil {
		return reconcile.Result{}, err
	} else if exists {
		return reconcile.Result{}, nil
	}

	// If the relation does not exist, create it
	if err := r.createRelation(relation, client); err != nil {
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, nil
}

func (r *Reconciler) reconcileDelete(relation *v1beta1.Relation) (reconcile.Result, error) {
	// If the relation has already been finalized, exit reconciliation
	if !k8s.HasFinalizer(relation, topoFinalizer) {
		return reconcile.Result{}, nil
	}

	// Connect to the topology service
	conn, err := grpc.ConnectService(r.client, relation.Namespace, topoService)
	if err != nil {
		return reconcile.Result{}, err
	}
	defer conn.Close()

	client := topo.NewTopoClient(conn)

	// Delete the relation from the topology
	if err := r.deleteRelation(relation, client); err != nil {
		return reconcile.Result{}, err
	}

	// Once the relation has been deleted, remove the topology finalizer
	k8s.RemoveFinalizer(relation, topoFinalizer)
	if err := r.client.Update(context.TODO(), relation); err != nil {
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, nil
}

func (r *Reconciler) relationExists(relation *v1beta1.Relation, client topo.TopoClient) (bool, error) {
	request := &topo.GetRequest{
		ID: topo.ID(relation.Name),
	}
	_, err := client.Get(context.TODO(), request)
	if err == nil {
		return true, nil
	}

	stat, ok := status.FromError(err)
	if !ok {
		return false, err
	}

	err = errors.FromStatus(stat)
	if !errors.IsNotFound(err) {
		return false, err
	}
	return false, nil
}

func (r *Reconciler) createRelation(relation *v1beta1.Relation, client topo.TopoClient) error {
	request := &topo.CreateRequest{
		Object: &topo.Object{
			ID:   topo.ID(relation.Name),
			Type: topo.Object_RELATION,
			Obj: &topo.Object_Relation{
				Relation: &topo.Relation{
					KindID:      topo.ID(relation.Spec.Kind.Name),
					SrcEntityID: topo.ID(relation.Spec.Source.Name),
					TgtEntityID: topo.ID(relation.Spec.Target.Name),
				},
			},
			Attributes: relation.Spec.Attributes,
		},
	}

	_, err := client.Create(context.TODO(), request)
	if err == nil {
		return nil
	}

	stat, ok := status.FromError(err)
	if !ok {
		return err
	}

	err = errors.FromStatus(stat)
	if !errors.IsAlreadyExists(err) {
		return err
	}
	return nil
}

func (r *Reconciler) deleteRelation(relation *v1beta1.Relation, client topo.TopoClient) error {
	request := &topo.DeleteRequest{
		ID: topo.ID(relation.Name),
	}

	_, err := client.Delete(context.TODO(), request)
	if err == nil {
		return nil
	}

	stat, ok := status.FromError(err)
	if !ok {
		return err
	}

	err = errors.FromStatus(stat)
	if !errors.IsNotFound(err) {
		return err
	}
	return nil
}

type kindMapper struct {
	client client.Client
}

func (m *kindMapper) Map(object handler.MapObject) []reconcile.Request {
	kind := object.Object.(*v1beta1.Kind)
	relations := &v1beta1.RelationList{}
	relationFields := map[string]string{
		"spec.kind.name": kind.Name,
	}
	relationOpts := &client.ListOptions{
		Namespace:     kind.Namespace,
		FieldSelector: fields.SelectorFromSet(relationFields),
	}
	err := m.client.List(context.TODO(), relations, relationOpts)
	if err != nil {
		return []reconcile.Request{}
	}

	requests := make([]reconcile.Request, len(relations.Items))
	for i, relation := range relations.Items {
		requests[i] = reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: relation.Namespace,
				Name:      relation.Name,
			},
		}
	}
	return requests
}
