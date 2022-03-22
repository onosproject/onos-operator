// SPDX-FileCopyrightText: 2020-present Open Networking Foundation <info@opennetworking.org>
//
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetStateMode indicates the mode for reading state from a device
type GetStateMode string

const (
	// GetStateNone - device type does not support Operational State at all
	GetStateNone GetStateMode = "GetStateNone"
	// GetStateOpState - device returns all its op state attributes by querying
	// GetRequest_STATE and GetRequest_OPERATIONAL
	GetStateOpState GetStateMode = "GetStateOpState"
	// GetStateExplicitRoPaths - device returns all its op state attributes by querying
	// exactly what the ReadOnly paths from YANG - wildcards are handled by device
	GetStateExplicitRoPaths GetStateMode = "GetStateExplicitRoPaths"
	// GetStateExplicitRoPathsExpandWildcards - where there are wildcards in the
	// ReadOnly paths 2 calls have to be made - 1) to expand the wildcards in to
	// real paths (since the device doesn't do it) and 2) to query those expanded
	// wildcard paths - this is the Stratum 1.0.0 method
	GetStateExplicitRoPathsExpandWildcards GetStateMode = "GetStateExplicitRoPathsExpandWildcards"
)

// ModelSpec is the k8s spec for a Model resource
type ModelSpec struct {
	Plugin  *Plugin           `json:"plugin,omitempty"`
	Modules []Module          `json:"modules,omitempty"`
	Files   map[string]string `json:"files,omitempty"`
}

// Plugin is the spec for a Model plugin
type Plugin struct {
	Type         string       `json:"type,omitempty"`
	Version      string       `json:"version,omitempty"`
	GetStateMode GetStateMode `json:"getStateMode,omitempty"`
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
