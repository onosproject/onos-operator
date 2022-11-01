// SPDX-FileCopyrightText: 2020-present Open Networking Foundation <info@opennetworking.org>
//
// SPDX-License-Identifier: Apache-2.0

package kind

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

var log = logging.GetLogger("controller", "topo", "kind")

const topoService = "onos-topo"
const topoFinalizer = "topo"

// Add creates a new Kind controller and adds it to the Manager. The Manager will set fields on the
// controller and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	r := &Reconciler{
		client: mgr.GetClient(),
		scheme: mgr.GetScheme(),
		config: mgr.GetConfig(),
	}

	// Create a new controller
	c, err := controller.New("topo-kind-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource Kind
	err = c.Watch(&source.Kind{Type: &v1beta1.Kind{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}
	return nil
}

var _ reconcile.Reconciler = &Reconciler{}

// Reconciler reconciles a Kind object
type Reconciler struct {
	client client.Client
	scheme *runtime.Scheme
	config *rest.Config
}

// Reconcile reads that state of the cluster for a Kind object and makes changes based on the state read
// and what is in the Kind.Spec
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log.Infof("Reconciling Kind %s/%s", request.Namespace, request.Name)

	// Fetch the Kind instance
	kind := &v1beta1.Kind{}
	err := r.client.Get(ctx, request.NamespacedName, kind)
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

	if kind.DeletionTimestamp == nil {
		return r.reconcileCreate(ctx, kind)
	}
	return r.reconcileDelete(ctx, kind)
}

func (r *Reconciler) reconcileCreate(ctx context.Context, kind *v1beta1.Kind) (reconcile.Result, error) {
	// Add the finalizer to the kind if necessary
	if !k8s.HasFinalizer(kind, topoFinalizer) {
		k8s.AddFinalizer(kind, topoFinalizer)
		err := r.client.Update(ctx, kind)
		if err != nil {
			return reconcile.Result{}, err
		}
	}

	// Connect to the topology service
	conn, err := grpc.ConnectService(r.client, kind.Namespace, topoService)
	if err != nil {
		return reconcile.Result{}, err
	}
	defer conn.Close()

	client := topo.NewTopoClient(conn)

	// Check if the kind exists in the topology and return it for update if so
	if object, err := r.kindExists(ctx, kind, client); err != nil {
		return reconcile.Result{}, err
	} else if object != nil {
		if err := r.updateKind(ctx, kind, object, client); err != nil {
			return reconcile.Result{}, err
		}
		return reconcile.Result{}, nil
	}

	// If the kind does not exist, create it
	if err := r.createKind(ctx, kind, client); err != nil {
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, nil
}

func (r *Reconciler) reconcileDelete(ctx context.Context, kind *v1beta1.Kind) (reconcile.Result, error) {
	// If the kind has already been finalized, exit reconciliation
	if !k8s.HasFinalizer(kind, topoFinalizer) {
		return reconcile.Result{}, nil
	}

	ns := &corev1.Namespace{}
	nsName := types.NamespacedName{
		Name: kind.Namespace,
	}
	err := r.client.Get(ctx, nsName, ns)
	if err != nil && !k8serrors.IsNotFound(err) {
		return reconcile.Result{}, err
	}

	if err == nil && ns.DeletionTimestamp == nil {
		// Connect to the topology service
		conn, err := grpc.ConnectService(r.client, kind.Namespace, topoService)
		if err != nil {
			return reconcile.Result{}, err
		}
		defer conn.Close()

		client := topo.NewTopoClient(conn)

		// Delete the kind from the topology
		if err := r.deleteKind(ctx, kind, client); err != nil {
			return reconcile.Result{}, err
		}
	}

	// Once the kind has been deleted, remove the topology finalizer
	k8s.RemoveFinalizer(kind, topoFinalizer)
	if err := r.client.Update(context.TODO(), kind); err != nil {
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, nil
}

func (r *Reconciler) kindExists(ctx context.Context, kind *v1beta1.Kind, client topo.TopoClient) (*topo.Object, error) {
	request := &topo.GetRequest{
		ID: topo.ID(kind.Name),
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

func (r *Reconciler) createKind(ctx context.Context, kind *v1beta1.Kind, client topo.TopoClient) error {
	object := &topo.Object{
		ID:   topo.ID(kind.Name),
		Type: topo.Object_KIND,
		Obj: &topo.Object_Kind{
			Kind: &topo.Kind{
				Name: kind.Name,
			},
		},
		Aspects: make(map[string]*prototypes.Any),
	}
	for key, value := range kind.Spec.Aspects {
		err := object.SetAspectBytes(key, value.Raw)
		if err != nil {
			return err
		}
	}
	log.Infof("Creating kind %+v", object)

	request := &topo.CreateRequest{
		Object: object,
	}
	_, err := client.Create(ctx, request)
	if err == nil {
		log.Infof("Kind created: %+v", object)
		return nil
	}

	stat, ok := status.FromError(err)
	if !ok {
		log.Warnf("Unable to create kind %s: %+v", object.ID, err)
		return err
	}

	err = errors.FromStatus(stat)
	if !errors.IsAlreadyExists(err) {
		log.Warnf("Unable to create kind %s: status=%+v'; err=%+v", object.ID, stat, err)
		return err
	}
	return nil
}

func (r *Reconciler) updateKind(ctx context.Context, kind *v1beta1.Kind, object *topo.Object, client topo.TopoClient) error {
	for key, value := range kind.Spec.Aspects {
		err := object.SetAspectBytes(key, value.Raw)
		if err != nil {
			return err
		}
	}
	log.Infof("Updating kind %+v", object)

	request := &topo.UpdateRequest{
		Object: object,
	}
	_, err := client.Update(ctx, request)
	if err == nil {
		log.Infof("Kind updated: %+v", object)
		return nil
	}

	stat, ok := status.FromError(err)
	if !ok {
		log.Warnf("Unable to update kind %s: %+v", object.ID, err)
		return err
	}

	err = errors.FromStatus(stat)
	if !errors.IsAlreadyExists(err) {
		log.Warnf("Unable to update kind %s: status=%+v'; err=%+v", object.ID, stat, err)
		return err
	}
	return nil
}

func (r *Reconciler) deleteKind(ctx context.Context, kind *v1beta1.Kind, client topo.TopoClient) error {
	request := &topo.DeleteRequest{
		ID: topo.ID(kind.Name),
	}
	log.Infof("Deleting kind %s", request.ID)

	_, err := client.Delete(ctx, request)
	if err == nil {
		log.Infof("Kind deleted: %s", request.ID)
		return nil
	}

	stat, ok := status.FromError(err)
	if !ok {
		log.Warnf("Unable to delete kind %s: %+v", request.ID, err)
		return err
	}

	err = errors.FromStatus(stat)
	if !errors.IsNotFound(err) {
		log.Warnf("Unable to delete kind %s: status=%+v'; err=%+v", request.ID, stat, err)
		return err
	}
	return nil
}
