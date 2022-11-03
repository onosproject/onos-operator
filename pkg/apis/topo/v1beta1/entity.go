// SPDX-FileCopyrightText: 2020-present Open Networking Foundation <info@opennetworking.org>
//
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// EntitySpec is the k8s spec for a Entity resource
type EntitySpec struct {
	URI         string                          `json:"uri,omitempty"`
	Kind        metav1.ObjectMeta               `json:"kind,omitempty"`
	Aspects     map[string]runtime.RawExtension `json:"aspects,omitempty"`
	ServiceName string                          `json:"serviceName,omitempty"`
}

// EntityState defines the states of an entity
type EntityState string

const (
	// StatePending when entity is waiting for topo cluster
	StatePending EntityState = "Pending"
	// StateAdding when adding entity to topo
	StateAdding EntityState = "Adding"
	// StateAdded when entity is added to topo
	StateAdded EntityState = "Added"
	// StateRemoving when removing entity from topo
	StateRemoving EntityState = "Removing"
	// StateRemoved when entity is removed from topo OR topo service is not found (deleted)
	StateRemoved EntityState = "Removed"
)

// EntityStatus defines the observed state of Entity
type EntityStatus struct {
	State EntityState `json:"state"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Entity is the Schema for the Entity API
// +k8s:openapi-gen=true
type Entity struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              EntitySpec   `json:"spec,omitempty"`
	Status            EntityStatus `json:"status"`
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
