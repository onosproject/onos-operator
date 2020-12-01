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
	Type       string      `json:"type,omitempty"`
	Version    string      `json:"version,omitempty"`
	YangModels []YangModel `json:"yangModels,omitempty"`
}

// YangModel defines a Yang model
type YangModel struct {
	Name         string `json:"name,omitempty"`
	Organization string `json:"organization,omitempty"`
	Version      string `json:"version,omitempty"`
	Data         string `json:"data,omitempty"`
}

// ModelStatus defines the observed state of Model
type ModelStatus struct {
	Phase *ModelPhase `json:"phase,omitempty"`
}

// ModelPhase is the phase of a model
type ModelPhase string

const (
	ModelPhaseGenerating ModelPhase = "Generating"
	ModelPhaseGenerated  ModelPhase = "Generated"
	ModelPhaseInstalling ModelPhase = "Installing"
	ModelPhaseInstalled  ModelPhase = "Installed"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Model is the Schema for the Model API
// +k8s:openapi-gen=true
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
