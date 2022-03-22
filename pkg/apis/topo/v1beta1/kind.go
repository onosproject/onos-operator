// SPDX-FileCopyrightText: 2020-present Open Networking Foundation <info@opennetworking.org>
//
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// KindSpec is the k8s spec for a Kind resource
type KindSpec struct {
	Aspects map[string]runtime.RawExtension `json:"aspects,omitempty"`
}

// KindStatus defines the observed state of Kind
type KindStatus struct{}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Kind is the Schema for the Kind API
// +k8s:openapi-gen=true
type Kind struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              KindSpec   `json:"spec,omitempty"`
	Status            KindStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// KindList contains a list of Database
type KindList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Kind `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Kind{}, &KindList{})
}
