package config

import (
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// RegisterWebhooks registes admission webhooks on the given manager
func RegisterWebhooks(mgr manager.Manager) error {
	mgr.GetWebhookServer().Register("/compiler", &webhook.Admission{
		Handler: &CompilerInjector{
			client: mgr.GetClient(),
		},
	})
	mgr.GetWebhookServer().Register("/registry", &webhook.Admission{
		Handler: &RegistryInjector{
			client: mgr.GetClient(),
		},
	})
	return nil
}
