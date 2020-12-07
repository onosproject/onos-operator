package config

import (
	"github.com/onosproject/onos-lib-go/pkg/logging"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

var log = logging.GetLogger("admission", "config")

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
