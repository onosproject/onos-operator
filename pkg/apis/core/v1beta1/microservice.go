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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MicroserviceSpec is the k8s spec for a Service resource
type MicroserviceSpec struct {
	Replicas *int32                 `json:"replicas,omitempty" protobuf:"varint,1,opt,name=replicas"`
	Selector *metav1.LabelSelector  `json:"selector"`
	Template corev1.PodTemplateSpec `json:"template" protobuf:"bytes,3,opt,name=template"`
	Model    metav1.ObjectMeta      `json:"model,omitempty"`
	Config   map[string]string      `json:"config,omitempty"`
}

// MicroserviceStatus defines the observed state of Service
type MicroserviceStatus struct{}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Service is the Schema for the Service API
// +k8s:openapi-gen=true
type Microservice struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              MicroserviceSpec   `json:"spec,omitempty"`
	Status            MicroserviceStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MicroserviceList contains a list of Database
type MicroserviceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Microservice `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Microservice{}, &MicroserviceList{})
}
