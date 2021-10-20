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

package entity

import (
	"context"
	prototypes "github.com/gogo/protobuf/types"
	"github.com/onosproject/onos-api/go/onos/topo"
	"github.com/onosproject/onos-lib-go/pkg/errors"
	"github.com/onosproject/onos-lib-go/pkg/logging"
	"github.com/onosproject/onos-operator/pkg/apis/topo/v1beta1"
	"github.com/onosproject/onos-operator/pkg/controller/util/grpc"
	"github.com/onosproject/onos-operator/pkg/controller/util/k8s"
	"google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logging.GetLogger("controller", "topo", "entity")

const topoService = "onos-topo"
const topoFinalizer = "topo"

// Add creates a new Entity controller and adds it to the Manager. The Manager will set fields on the
// controller and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	r := &Reconciler{
		client: mgr.GetClient(),
		scheme: mgr.GetScheme(),
		config: mgr.GetConfig(),
	}

	// Create a new controller
	c, err := controller.New("topo-entity-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource Entity
	err = c.Watch(&source.Kind{Type: &v1beta1.Entity{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}
	return nil
}

var _ reconcile.Reconciler = &Reconciler{}

// Reconciler reconciles a Entity object
type Reconciler struct {
	client client.Client
	scheme *runtime.Scheme
	config *rest.Config
}

// Reconcile reads that state of the cluster for a Entity object and makes changes based on the state read
// and what is in the Entity.Spec
func (r *Reconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	log.Infof("Reconciling Entity %s/%s", request.Namespace, request.Name)

	// Fetch the Entity instance
	entity := &v1beta1.Entity{}
	err := r.client.Get(context.TODO(), request.NamespacedName, entity)
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

	if entity.DeletionTimestamp == nil {
		return r.reconcileCreate(entity)
	}
	return r.reconcileDelete(entity)
}

func (r *Reconciler) reconcileCreate(entity *v1beta1.Entity) (reconcile.Result, error) {
	// Add the finalizer to the entity if necessary
	if !k8s.HasFinalizer(entity, topoFinalizer) {
		k8s.AddFinalizer(entity, topoFinalizer)
		err := r.client.Update(context.TODO(), entity)
		if err != nil {
			return reconcile.Result{}, err
		}
	}

	// Connect to the topology service
	conn, err := grpc.ConnectService(r.client, entity.Namespace, topoService)
	if err != nil {
		return reconcile.Result{}, err
	}
	defer conn.Close()

	client := topo.NewTopoClient(conn)

	// Check if the entity exists in the topology and return it for update if so
	if object, err := r.entityExists(entity, client); err != nil {
		return reconcile.Result{}, err
	} else if object != nil {
		if err := r.updateEntity(entity, object, client); err != nil {
			return reconcile.Result{}, err
		}
		return reconcile.Result{}, nil
	}

	// If the entity does not exist, create it
	if err := r.createEntity(entity, client); err != nil {
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, nil
}

func (r *Reconciler) reconcileDelete(entity *v1beta1.Entity) (reconcile.Result, error) {
	// If the entity has already been finalized, exit reconciliation
	if !k8s.HasFinalizer(entity, topoFinalizer) {
		return reconcile.Result{}, nil
	}

	ns := &corev1.Namespace{}
	nsName := types.NamespacedName{
		Name: entity.Namespace,
	}
	err := r.client.Get(context.TODO(), nsName, ns)
	if err != nil && !k8serrors.IsNotFound(err) {
		return reconcile.Result{}, err
	}

	if err == nil && ns.DeletionTimestamp == nil {
		// Connect to the topology service
		conn, err := grpc.ConnectService(r.client, entity.Namespace, topoService)
		if err != nil {
			return reconcile.Result{}, err
		}
		defer conn.Close()

		client := topo.NewTopoClient(conn)

		// Delete the entity from the topology
		if err := r.deleteEntity(entity, client); err != nil {
			return reconcile.Result{}, err
		}
	}

	// Once the entity has been deleted, remove the topology finalizer
	k8s.RemoveFinalizer(entity, topoFinalizer)
	if err := r.client.Update(context.TODO(), entity); err != nil {
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, nil
}

func (r *Reconciler) entityExists(entity *v1beta1.Entity, client topo.TopoClient) (*topo.Object, error) {
	request := &topo.GetRequest{
		ID: topo.ID(entity.Spec.URI),
	}
	resp, err := client.Get(context.TODO(), request)
	if err == nil {
		return resp.Object, nil
	}

	stat, ok := status.FromError(err)
	if !ok {
		return nil, err
	}

	err = errors.FromStatus(stat)
	if !errors.IsNotFound(err) {
		return nil, err
	}
	return nil, nil
}

func (r *Reconciler) createEntity(entity *v1beta1.Entity, client topo.TopoClient) error {
	object := &topo.Object{
		ID:   topo.ID(entity.Spec.URI),
		Type: topo.Object_ENTITY,
		Obj: &topo.Object_Entity{
			Entity: &topo.Entity{
				KindID: topo.ID(entity.Spec.Kind.Name),
			},
		},
		Aspects: make(map[string]*prototypes.Any),
	}
	for key, value := range entity.Spec.Aspects {
		err := object.SetAspectBytes(key, value.Raw)
		if err != nil {
			return err
		}
	}
	log.Infof("Creating entity %+v", object)

	request := &topo.CreateRequest{
		Object: object,
	}
	_, err := client.Create(context.TODO(), request)
	if err == nil {
		log.Infof("Entity created: %+v", object)
		return nil
	}

	stat, ok := status.FromError(err)
	if !ok {
		log.Infof("Unable to create entity: %+v", object)
		return err
	}

	err = errors.FromStatus(stat)
	if !errors.IsAlreadyExists(err) {
		log.Warnf("Unable to create entity %s: status=%+v'; err=%+v", object.ID, stat, err)
		return err
	}
	return nil
}

func (r *Reconciler) updateEntity(entity *v1beta1.Entity, object *topo.Object, client topo.TopoClient) error {
	for key, value := range entity.Spec.Aspects {
		err := object.SetAspectBytes(key, value.Raw)
		if err != nil {
			return err
		}
	}
	log.Infof("Updating entity %+v", object)

	request := &topo.UpdateRequest{
		Object: object,
	}
	_, err := client.Update(context.TODO(), request)
	if err == nil {
		log.Infof("Entity updated: %+v", object)
		return nil
	}

	stat, ok := status.FromError(err)
	if !ok {
		log.Warnf("Unable to update entity %s: %+v", object.ID, err)
		return err
	}

	err = errors.FromStatus(stat)
	if !errors.IsAlreadyExists(err) {
		log.Warnf("Unable to update entity %s: status=%+v'; err=%+v", object.ID, stat, err)
		return err
	}
	return nil
}

func (r *Reconciler) deleteEntity(entity *v1beta1.Entity, client topo.TopoClient) error {
	request := &topo.DeleteRequest{
		ID: topo.ID(entity.Spec.URI),
	}
	log.Infof("Deleting entity %s", request.ID)

	_, err := client.Delete(context.TODO(), request)
	if err == nil {
		log.Infof("Entity deleted: %s", request.ID)
		return nil
	}

	stat, ok := status.FromError(err)
	if !ok {
		log.Warnf("Unable to delete entity %s: %+v", request.ID, err)
		return err
	}

	err = errors.FromStatus(stat)
	if !errors.IsNotFound(err) {
		log.Warnf("Unable to delete entity %s: status=%+v'; err=%+v", request.ID, stat, err)
		return err
	}
	return nil
}
