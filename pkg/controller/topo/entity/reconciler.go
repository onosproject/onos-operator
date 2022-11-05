// SPDX-FileCopyrightText: 2020-present Open Networking Foundation <info@opennetworking.org>
//
// SPDX-License-Identifier: Apache-2.0

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

const topoFinalizer = "topo.onosproject.org/entity"

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

	err = c.Watch(&source.Kind{Type: &corev1.Service{}}, handler.EnqueueRequestsFromMapFunc(func(object client.Object) []reconcile.Request {
		topoEntityList := &v1beta1.EntityList{}
		if err := mgr.GetClient().List(context.Background(), topoEntityList, &client.ListOptions{Namespace: object.GetNamespace()}); err != nil {
			log.Error(err)
			return nil
		}
		var requests []reconcile.Request
		for _, entity := range topoEntityList.Items {
			if object.GetName() == entity.Spec.ServiceName {
				requests = append(requests, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Namespace: entity.Namespace,
						Name:      entity.Name,
					},
				})
			}
		}
		return requests
	}))
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
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log.Infof("Reconciling Entity request in namespace %s, %s", request.Name, request.Namespace)
	// Fetch the Entity instance
	entity := &v1beta1.Entity{}
	err := r.client.Get(ctx, request.NamespacedName, entity)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		log.Warnf("Failed to reconcile entity %s in namespace %s, %s", request.Name, request.Namespace, err)
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	if entity.DeletionTimestamp == nil {
		return r.reconcileCreate(ctx, entity)
	}
	return r.reconcileDelete(ctx, entity)
}

func (r *Reconciler) reconcileCreate(ctx context.Context, entity *v1beta1.Entity) (reconcile.Result, error) {
	// Get topo service name
	topoServiceName := entity.Spec.ServiceName

	// Check if topo service is available
	topoNamespacedName := types.NamespacedName{Namespace: entity.Namespace, Name: topoServiceName}
	topoServiceObject := &corev1.Service{}
	if err := r.client.Get(ctx, topoNamespacedName, topoServiceObject); err != nil && k8serrors.IsNotFound(err) {
		// Set the state to StatePending if topo service is not found (deleted).
		entity.Status = v1beta1.EntityStatus{State: v1beta1.StatePending}
		if err := r.client.Status().Update(ctx, entity); err != nil {
			log.Warnf("Failed to update state of entity %s, %s, %s", entity.Name, entity.Namespace, err)
			return reconcile.Result{}, err
		}
		log.Warnf("Failed to find topo service %s in namespace %s, %s", topoServiceName, entity.Namespace, err)
		return reconcile.Result{}, err
	}

	switch entity.Status.State {
	case v1beta1.StatePending:
		entity.Status = v1beta1.EntityStatus{State: v1beta1.StateAdding}
		err := r.client.Status().Update(ctx, entity)
		if err != nil {
			log.Warnf("Failed to reconcile updating state of entity %s, %s, %s", entity.Name, entity.Namespace, err)
			return reconcile.Result{}, err
		}
		return reconcile.Result{}, nil
	case v1beta1.StateAdding:
		// Add the finalizer to the entity if necessary
		if !k8s.HasFinalizer(entity, topoFinalizer) {
			k8s.AddFinalizer(entity, topoFinalizer)
			err := r.client.Update(ctx, entity)
			if err != nil {
				log.Warnf("Failed to reconcile adding finalizer to entity %s, %s, %s", entity.Name, entity.Namespace, err)
				return reconcile.Result{}, err
			}
			return reconcile.Result{}, nil
		}
		// Connect to the topology service
		conn, err := grpc.ConnectService(r.client, entity.Namespace, topoServiceName)
		if err != nil {
			log.Warnf("Failed to reconcile creating entity %s, %s, %s", entity.Name, entity.Namespace, err)
			return reconcile.Result{}, err
		}
		defer conn.Close()
		client := topo.NewTopoClient(conn)
		// Check if the entity exists in the topology and return it for update if so
		if object, err := r.entityExists(ctx, entity, client); err != nil {
			return reconcile.Result{}, err
		} else if object != nil {
			if err := r.updateEntity(ctx, entity, object, client); err != nil && !errors.IsNotFound(err) && !errors.IsConflict(err) {
				log.Warnf("Failed to reconcile creating entity %s, %s, %s", entity.Name, entity.Namespace, err)
				return reconcile.Result{}, err
			}
		}
		if err := r.createEntity(ctx, entity, client); err != nil && !errors.IsAlreadyExists(err) {
			log.Warnf("Failed to reconcile creating entity %s, %s, %s", entity.Name, entity.Namespace, err)
			return reconcile.Result{}, err
		}
		entity.Status = v1beta1.EntityStatus{State: v1beta1.StateAdded}
		err = r.client.Status().Update(ctx, entity)
		if err != nil {
			log.Warnf("Failed to reconcile updating state of entity %s, %s, %s", entity.Name, entity.Namespace, err)
			return reconcile.Result{}, err
		}
		return reconcile.Result{}, nil
	case v1beta1.StateAdded:
		log.Debugf("Entity %s is already added to topo store.", entity.Name)
		return reconcile.Result{}, nil
	}
	return reconcile.Result{}, nil
}

func (r *Reconciler) reconcileDelete(ctx context.Context, entity *v1beta1.Entity) (reconcile.Result, error) {
	// If the entity has already been finalized, exit reconciliation
	if !k8s.HasFinalizer(entity, topoFinalizer) {
		return reconcile.Result{}, nil
	}

	// Get topo service name
	topoServiceName := entity.Spec.ServiceName

	// Check if topo service is available
	objectKey := types.NamespacedName{Namespace: entity.Namespace, Name: topoServiceName}
	topoServiceObject := &corev1.Service{}
	if err := r.client.Get(ctx, objectKey, topoServiceObject); err != nil && k8serrors.IsNotFound(err) {
		// Remove the finalizer if topo service is not found (deleted).
		k8s.RemoveFinalizer(entity, topoFinalizer)
		if err := r.client.Update(ctx, entity); err != nil {
			log.Warnf("Failed to reconcile updating entity %s, %s, %s", entity.Name, entity.Namespace, err)
			return reconcile.Result{}, err
		}
		log.Warnf("Failed to find topo service %s in namespace %s, %s; removed entities' finalizer", topoServiceName, entity.Namespace, err)
		return reconcile.Result{}, err
	}

	switch entity.Status.State {
	case v1beta1.StateAdding, v1beta1.StateAdded:
		entity.Status = v1beta1.EntityStatus{State: v1beta1.StateRemoving}
		err := r.client.Status().Update(ctx, entity)
		if err != nil {
			log.Warnf("Failed to reconcile updating state of entity %s, %s, %s", entity.Name, entity.Namespace, err)
			return reconcile.Result{}, err
		}
		return reconcile.Result{}, nil
	case v1beta1.StateRemoving:
		// Connect to the topology service
		conn, err := grpc.ConnectService(r.client, entity.Namespace, topoServiceName)
		if err != nil {
			log.Warnf("Failed to reconcile deleting entity %s, %s, %s", entity.Name, entity.Namespace, err)
			return reconcile.Result{}, err
		}
		defer conn.Close()
		client := topo.NewTopoClient(conn)
		// Delete the entity from the topology
		if err := r.deleteEntity(ctx, entity, client); err != nil && !errors.IsNotFound(err) {
			log.Warnf("Failed to reconcile deleting entity %s, %s, %s", entity.Name, entity.Namespace, err)
			return reconcile.Result{}, err
		}
		entity.Status = v1beta1.EntityStatus{State: v1beta1.StateRemoved}
		err = r.client.Status().Update(ctx, entity)
		if err != nil {
			log.Warnf("Failed to reconcile updating state of entity %s, %s, %s", entity.Name, entity.Namespace, err)
			return reconcile.Result{}, err
		}
		return reconcile.Result{}, nil
	case v1beta1.StateRemoved:
		log.Debugf("Entity %s is already removed or never been added to the topo store.", entity.Name)
		k8s.RemoveFinalizer(entity, topoFinalizer)
		if err := r.client.Update(ctx, entity); err != nil {
			log.Warnf("Failed to reconcile removing finalizer of entity %s, %s, %s", entity.Name, entity.Namespace, err)
			return reconcile.Result{}, err
		}
		return reconcile.Result{}, nil
	}

	return reconcile.Result{}, nil
}

func (r *Reconciler) entityExists(ctx context.Context, entity *v1beta1.Entity, client topo.TopoClient) (*topo.Object, error) {
	request := &topo.GetRequest{
		ID: topo.ID(entity.Spec.URI),
	}
	resp, err := client.Get(ctx, request)
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

func (r *Reconciler) createEntity(ctx context.Context, entity *v1beta1.Entity, client topo.TopoClient) error {
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
	_, err := client.Create(ctx, request)
	if err == nil {
		log.Infof("Entity created: %+v", object)
		return nil
	}

	stat, ok := status.FromError(err)
	if !ok {
		log.Warnf("Unable to create entity: %+v", object, err)
		return err
	}

	err = errors.FromStatus(stat)
	if !errors.IsAlreadyExists(err) {
		log.Warnf("Unable to create entity %s: status=%+v'; err=%+v", object.ID, stat, err)
		return err
	}
	return nil
}

func (r *Reconciler) updateEntity(ctx context.Context, entity *v1beta1.Entity, object *topo.Object, client topo.TopoClient) error {
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
	_, err := client.Update(ctx, request)
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

func (r *Reconciler) deleteEntity(ctx context.Context, entity *v1beta1.Entity, client topo.TopoClient) error {
	request := &topo.DeleteRequest{
		ID: topo.ID(entity.Spec.URI),
	}
	log.Infof("Deleting entity %s", request.ID)

	_, err := client.Delete(ctx, request)
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
