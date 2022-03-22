// SPDX-FileCopyrightText: 2020-present Open Networking Foundation <info@opennetworking.org>
//
// SPDX-License-Identifier: Apache-2.0

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
