// SPDX-FileCopyrightText: 2020-present Open Networking Foundation <info@opennetworking.org>
//
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// RelationEndpoint represents the source or target or a Relation resource
type RelationEndpoint struct {
	metav1.ObjectMeta `json:"metadata,omitempty"`
	URI               string `json:"uri,omitempty"`
}

// RelationSpec is the k8s spec for a Relation resource
type RelationSpec struct {
	URI     string                          `json:"uri,omitempty"`
	Kind    metav1.ObjectMeta               `json:"kind,omitempty"`
	Source  RelationEndpoint                `json:"source,omitempty"`
	Target  RelationEndpoint                `json:"target,omitempty"`
	Aspects map[string]runtime.RawExtension `json:"aspects,omitempty"`
}

// RelationStatus defines the observed state of Relation
type RelationStatus struct{}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Relation is the Schema for the Relation API
// +k8s:openapi-gen=true
type Relation struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              RelationSpec   `json:"spec,omitempty"`
	Status            RelationStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RelationList contains a list of Database
type RelationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Relation `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Relation{}, &RelationList{})
}
