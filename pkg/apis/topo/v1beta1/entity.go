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

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// EntitySpec is the k8s spec for a Entity resource
type EntitySpec struct {
	Kind    metav1.ObjectMeta               `json:"kind,omitempty"`
	URI     string                          `json:"uri,omitempty"`
	Aspects map[string]runtime.RawExtension `json:"aspects,omitempty"`
}

// EntityStatus defines the observed state of Entity
type EntityStatus struct{}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Entity is the Schema for the Entity API
// +k8s:openapi-gen=true
type Entity struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              EntitySpec   `json:"spec,omitempty"`
	Status            EntityStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// EntityList contains a list of Database
type EntityList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Entity `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Entity{}, &EntityList{})
}
