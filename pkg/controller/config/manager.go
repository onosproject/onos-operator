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

package config

import (
	"github.com/onosproject/onos-operator/pkg/controller/config/model"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// AddControllers adds the configuration controllers to the given manager
func AddControllers(mgr manager.Manager) error {
	if err := model.Add(mgr); err != nil {
		return err
	}
	return nil
}
