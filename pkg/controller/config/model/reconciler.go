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
	"github.com/onosproject/onos-lib-go/pkg/logging"
	"github.com/onosproject/onos-operator/pkg/apis/config/v1beta1"
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
	log.Infof("Reconciling Model %s/%s", request.Namespace, request.Name)

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

	// Create a ConfigMap to store the modules
	cm := &corev1.ConfigMap{}
	cmName := types.NamespacedName{
		Name:      model.Name,
		Namespace: model.Namespace,
	}
	if err := r.client.Get(context.Background(), cmName, cm); err != nil {
		if !errors.IsNotFound(err) {
			return reconcile.Result{}, err
		}

		data := make(map[string]string)
		for _, module := range model.Spec.Modules {
			name := fmt.Sprintf("%s@%s", module.Name, module.Version)
			data[name] = module.Data
		}

		cm = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      model.Name,
				Namespace: model.Namespace,
			},
			Data: data,
		}
		if err := r.client.Create(context.Background(), cm); err != nil {
			return reconcile.Result{}, err
		}
	}
	return reconcile.Result{}, nil
}
