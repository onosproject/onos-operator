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

package model

import (
	"context"
	"fmt"
	"github.com/onosproject/onos-config/api/admin"
	"github.com/onosproject/onos-lib-go/pkg/logging"
	"github.com/onosproject/onos-operator/pkg/apis/config/v1beta1"
	"google.golang.org/grpc"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

var log = logging.GetLogger("controller", "config", "model")

// Add creates a new Database controller and adds it to the Manager. The Manager will set fields on the
// controller and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	r := &Reconciler{
		client: mgr.GetClient(),
		scheme: mgr.GetScheme(),
		config: mgr.GetConfig(),
	}

	// Create a new controller
	c, err := controller.New("config-model-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource Model
	err = c.Watch(&source.Kind{Type: &v1beta1.Model{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &Reconciler{}

// Reconciler reconciles a Model object
type Reconciler struct {
	client client.Client
	scheme *runtime.Scheme
	config *rest.Config
}

// Reconcile reads that state of the cluster for a Model object and makes changes based on the state read
// and what is in the Model.Spec
func (r *Reconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	log.Infof("Reconciling Model %s.%s", request.Namespace, request.Name)

	// Fetch the Model instance
	model := &v1beta1.Model{}
	err := r.client.Get(context.TODO(), request.NamespacedName, model)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	if model.Status.Phase == nil {
		phase := v1beta1.ModelPhaseGenerating
		model.Status.Phase = &phase
		if err := r.client.Status().Update(context.TODO(), model); err != nil {
			return reconcile.Result{}, err
		}
		return reconcile.Result{}, nil
	}

	phase := *model.Status.Phase
	switch phase {
	case v1beta1.ModelPhaseGenerating:
		return r.generateModel(model)
	case v1beta1.ModelPhaseGenerated:
		phase := v1beta1.ModelPhaseInstalling
		model.Status.Phase = &phase
		if err := r.client.Status().Update(context.TODO(), model); err != nil {
			return reconcile.Result{}, err
		}
		return reconcile.Result{}, nil
	case v1beta1.ModelPhaseInstalling:
		return r.installModel(model)
	case v1beta1.ModelPhaseInstalled:
	}
	return reconcile.Result{}, nil
}

func (r *Reconciler) generateModel(model *v1beta1.Model) (reconcile.Result, error) {
	bytes, err := generatePlugin(model)
	if err != nil {
		log.Error(err)
		return reconcile.Result{}, nil
	}

	plugin := getPluginName(model)
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: model.Namespace,
			Name:      model.Name,
		},
		BinaryData: map[string][]byte{
			plugin: bytes,
		},
	}
	if err := r.client.Create(context.TODO(), cm); err != nil {
		return reconcile.Result{}, err
	}

	phase := v1beta1.ModelPhaseGenerated
	model.Status.Phase = &phase
	if err := r.client.Status().Update(context.TODO(), model); err != nil {
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, nil
}

func (r *Reconciler) installModel(model *v1beta1.Model) (reconcile.Result, error) {
	cm := &corev1.ConfigMap{}
	name := types.NamespacedName{
		Namespace: model.Namespace,
		Name:      model.Name,
	}
	if err := r.client.Get(context.TODO(), name, cm); err != nil {
		return reconcile.Result{}, err
	}

	plugin := getPluginName(model)
	bytes := cm.BinaryData[plugin]

	services := &corev1.ServiceList{}
	if err := r.client.List(context.TODO(), services, client.HasLabels{"app=onos", "type=config"}); err != nil {
		return reconcile.Result{}, err
	} else if len(services.Items) == 0 {
		return reconcile.Result{}, nil
	}

	service := services.Items[0]

	conn, err := grpc.Dial(fmt.Sprintf("%s:%d", service.Name, service.Spec.Ports[0].Port), grpc.WithInsecure())
	if err != nil {
		return reconcile.Result{}, err
	}

	client := admin.NewConfigAdminServiceClient(conn)
	stream, err := client.UploadRegisterModel(context.TODO())
	if err != nil {
		return reconcile.Result{}, err
	}

	for i := 0; i < len(bytes); i += 1040 {
		var chunk []byte
		if len(bytes) < i+1024 {
			chunk = bytes[i:]
		} else {
			chunk = bytes[i : i+1024]
		}
		err := stream.Send(&admin.Chunk{
			SoFile:  fmt.Sprintf("%s.so", model.Name),
			Content: chunk,
		})
		if err != nil {
			return reconcile.Result{}, err
		}
	}

	_, err = stream.CloseAndRecv()
	if err != nil {
		return reconcile.Result{}, err
	}

	phase := v1beta1.ModelPhaseInstalled
	model.Status.Phase = &phase
	if err := r.client.Status().Update(context.TODO(), model); err != nil {
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, nil
}
