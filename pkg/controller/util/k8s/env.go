package k8s

import "os"

type Scope string

const (
	ClusterScope   Scope = "cluster"
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

func GetName(def string) string {
	name := os.Getenv(nameEnv)
	if name != "" {
		return name
	}
	return def
}

func GetNamespace() string {
	namespace := os.Getenv(namespaceEnv)
	if namespace != "" {
		return namespace
	}
	return defaultNamespace
}

func GetScope() Scope {
	scope := os.Getenv(scopeEnv)
	if scope != "" {
		return Scope(scope)
	}
	return defaultScope
}
