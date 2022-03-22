// SPDX-FileCopyrightText: 2020-present Open Networking Foundation <info@opennetworking.org>
//
// SPDX-License-Identifier: Apache-2.0

package k8s

import "os"

// Scope :
type Scope string

const (
	// ClusterScope :
	ClusterScope Scope = "cluster"

	// NamespaceScope :
	NamespaceScope Scope = "namespace"
)

const (
	nameEnv      = "CONTROLLER_NAME"
	namespaceEnv = "CONTROLLER_NAMESPACE"
	scopeEnv     = "CONTROLLER_SCOPE"
)

const (
	defaultNamespace = "kube-system"
	defaultScope     = ClusterScope
)

// GetName :
func GetName(def string) string {
	name := os.Getenv(nameEnv)
	if name != "" {
		return name
	}
	return def
}

// GetNamespace :
func GetNamespace() string {
	namespace := os.Getenv(namespaceEnv)
	if namespace != "" {
		return namespace
	}
	return defaultNamespace
}

// GetScope :
func GetScope() Scope {
	scope := os.Getenv(scopeEnv)
	if scope != "" {
		return Scope(scope)
	}
	return defaultScope
}
