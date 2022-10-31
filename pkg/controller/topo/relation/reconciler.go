// SPDX-FileCopyrightText: 2020-present Open Networking Foundation <info@opennetworking.org>
//
// SPDX-License-Identifier: Apache-2.0

package relation

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
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log.Infof("Reconciling Relation %s/%s", request.Namespace, request.Name)

	// Fetch the Relation instance
	relation := &v1beta1.Relation{}
	err := r.client.Get(ctx, request.NamespacedName, relation)
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
		return r.reconcileCreate(ctx, relation)
	}
	return r.reconcileDelete(ctx, relation)
}

func (r *Reconciler) reconcileCreate(ctx context.Context, relation *v1beta1.Relation) (reconcile.Result, error) {
	// Add the finalizer to the relation if necessary
	if !k8s.HasFinalizer(relation, topoFinalizer) {
		k8s.AddFinalizer(relation, topoFinalizer)
		err := r.client.Update(ctx, relation)
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

	// Check if the relation exists in the topology and return it for update if so
	if object, err := r.relationExists(ctx, relation, client); err != nil {
		return reconcile.Result{}, err
	} else if object != nil {
		if err := r.updateRelation(ctx, relation, object, client); err != nil {
			return reconcile.Result{}, err
		}
		return reconcile.Result{}, nil
	}

	// If the relation does not exist, create it
	if err := r.createRelation(ctx, relation, client); err != nil {
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, nil
}

func (r *Reconciler) reconcileDelete(ctx context.Context, relation *v1beta1.Relation) (reconcile.Result, error) {
	// If the relation has already been finalized, exit reconciliation
	if !k8s.HasFinalizer(relation, topoFinalizer) {
		return reconcile.Result{}, nil
	}

	ns := &corev1.Namespace{}
	nsName := types.NamespacedName{
		Name: relation.Namespace,
	}
	err := r.client.Get(ctx, nsName, ns)
	if err != nil && !k8serrors.IsNotFound(err) {
		return reconcile.Result{}, err
	}

	if err == nil && ns.DeletionTimestamp == nil {
		// Connect to the topology service
		conn, err := grpc.ConnectService(r.client, relation.Namespace, topoService)
		if err != nil {
			return reconcile.Result{}, err
		}
		defer conn.Close()

		client := topo.NewTopoClient(conn)

		// Delete the relation from the topology
		if err := r.deleteRelation(ctx, relation, client); err != nil {
			return reconcile.Result{}, err
		}
	}

	// Once the relation has been deleted, remove the topology finalizer
	k8s.RemoveFinalizer(relation, topoFinalizer)
	if err := r.client.Update(context.TODO(), relation); err != nil {
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, nil
}

func (r *Reconciler) relationExists(ctx context.Context, relation *v1beta1.Relation, client topo.TopoClient) (*topo.Object, error) {
	request := &topo.GetRequest{
		ID: topo.ID(relation.Spec.URI),
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

func (r *Reconciler) createRelation(ctx context.Context, relation *v1beta1.Relation, client topo.TopoClient) error {
	object := &topo.Object{
		ID:   topo.ID(relation.Spec.URI),
		Type: topo.Object_RELATION,
		Obj: &topo.Object_Relation{
			Relation: &topo.Relation{
				KindID:      topo.ID(relation.Spec.Kind.Name),
				SrcEntityID: topo.ID(relation.Spec.Source.URI),
				TgtEntityID: topo.ID(relation.Spec.Target.URI),
			},
		},
		Aspects: make(map[string]*prototypes.Any),
	}
	for key, value := range relation.Spec.Aspects {
		err := object.SetAspectBytes(key, value.Raw)
		if err != nil {
			return err
		}
	}
	log.Infof("Creating relation %+v", object)

	request := &topo.CreateRequest{
		Object: object,
	}
	_, err := client.Create(ctx, request)
	if err == nil {
		log.Infof("Relation created: %+v", object)
		return nil
	}

	stat, ok := status.FromError(err)
	if !ok {
		log.Infof("Unable to create relation: %+v", object)
		return err
	}

	err = errors.FromStatus(stat)
	if !errors.IsAlreadyExists(err) {
		log.Warnf("Unable to create relation %s: status=%+v'; err=%+v", object.ID, stat, err)
		return err
	}
	return nil
}

func (r *Reconciler) updateRelation(ctx context.Context, relation *v1beta1.Relation, object *topo.Object, client topo.TopoClient) error {
	for key, value := range relation.Spec.Aspects {
		err := object.SetAspectBytes(key, value.Raw)
		if err != nil {
			return err
		}
	}
	log.Infof("Updating relation %+v", object)

	request := &topo.CreateRequest{
		Object: object,
	}
	_, err := client.Create(ctx, request)
	if err == nil {
		log.Infof("Relation updated: %+v", object)
		return nil
	}

	stat, ok := status.FromError(err)
	if !ok {
		log.Warnf("Unable to update relation %s: %+v", object.ID, err)
		return err
	}

	err = errors.FromStatus(stat)
	if !errors.IsAlreadyExists(err) {
		log.Warnf("Unable to update relation %s: status=%+v'; err=%+v", object.ID, stat, err)
		return err
	}
	return nil
}

func (r *Reconciler) deleteRelation(ctx context.Context, relation *v1beta1.Relation, client topo.TopoClient) error {
	request := &topo.DeleteRequest{
		ID: topo.ID(relation.Spec.URI),
	}
	log.Infof("Deleting relation %s", request.ID)

	_, err := client.Delete(ctx, request)
	if err == nil {
		log.Infof("Relation deleted: %s", request.ID)
		return nil
	}

	stat, ok := status.FromError(err)
	if !ok {
		log.Warnf("Unable to delete relation %s: %+v", request.ID, err)
		return err
	}

	err = errors.FromStatus(stat)
	if !errors.IsNotFound(err) {
		log.Warnf("Unable to delete relation %s: status=%+v'; err=%+v", request.ID, stat, err)
		return err
	}
	return nil
}
