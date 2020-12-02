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

package topo

import (
	"github.com/onosproject/onos-operator/pkg/apis/topo/v1beta1"
	"github.com/onosproject/onos-operator/pkg/controller/topo/entity"
	"github.com/onosproject/onos-operator/pkg/controller/topo/kind"
	"github.com/onosproject/onos-operator/pkg/controller/topo/relation"
	"github.com/onosproject/onos-operator/pkg/controller/topo/service"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// AddControllers adds the topology controllers to the given manager
func AddControllers(mgr manager.Manager) error {
	if err := entity.Add(mgr); err != nil {
		return err
	}
	if err := kind.Add(mgr); err != nil {
		return err
	}
	if err := relation.Add(mgr); err != nil {
		return err
	}
	if err := service.Add(mgr); err != nil {
		return err
	}

	if err := mgr.GetFieldIndexer().IndexField(&v1beta1.Entity{}, "spec.kind.name", func(rawObj runtime.Object) []string {
		entity := rawObj.(*v1beta1.Entity)
		return []string{entity.Spec.Kind.Name}
	}); err != nil {
		return err
	}

	if err := mgr.GetFieldIndexer().IndexField(&v1beta1.Relation{}, "spec.kind.name", func(rawObj runtime.Object) []string {
		relation := rawObj.(*v1beta1.Relation)
		return []string{relation.Spec.Kind.Name}
	}); err != nil {
		return err
	}
	return nil
}
