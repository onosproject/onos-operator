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
)

// ModelSpec is the k8s spec for a Model resource
type ModelSpec struct {
	Plugin  *Plugin           `json:"plugin,omitempty"`
	Modules []Module          `json:"modules,omitempty"`
	Files   map[string]string `json:"files,omitempty"`
}

// Plugin is the spec for a Model plugin
type Plugin struct {
	Type    string `json:"type,omitempty"`
	Version string `json:"version,omitempty"`
}

// Module defines a module
type Module struct {
	Name         string `json:"name,omitempty"`
	Organization string `json:"organization,omitempty"`
	Revision     string `json:"revision,omitempty"`
	File         string `json:"file,omitempty"`
}

// ModelStatus defines the observed state of Model
type ModelStatus struct {
	RegistryStatuses []RegistryStatus `json:"registryStatuses,omitempty"`
}

// RegistryStatus defines the state of a model in a registry
type RegistryStatus struct {
	PodName string     `json:"podName,omitempty"`
	Phase   ModelPhase `json:"phase,omitempty"`
}

// ModelPhase is the phase of a model
type ModelPhase string

const (
	// ModelPending pending
	ModelPending ModelPhase = "Pending"

	// ModelInstalling installing
	ModelInstalling ModelPhase = "Installing"

	// ModelInstalled installed
	ModelInstalled ModelPhase = "Installed"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:openapi-gen=true

// Model is the Schema for the Model API
type Model struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              ModelSpec   `json:"spec,omitempty"`
	Status            ModelStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ModelList contains a list of Database
type ModelList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Model `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Model{}, &ModelList{})
}
