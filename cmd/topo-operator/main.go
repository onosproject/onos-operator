// SPDX-FileCopyrightText: 2020-present Open Networking Foundation <info@opennetworking.org>
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"github.com/onosproject/onos-lib-go/pkg/logging"
	topoapi "github.com/onosproject/onos-operator/pkg/apis/topo"
	topoctrl "github.com/onosproject/onos-operator/pkg/controller/topo"
	"github.com/onosproject/onos-operator/pkg/controller/topo/entity"
	"github.com/onosproject/onos-operator/pkg/controller/topo/kind"
	"github.com/onosproject/onos-operator/pkg/controller/topo/relation"
	"github.com/onosproject/onos-operator/pkg/controller/topo/service"
	"github.com/onosproject/onos-operator/pkg/controller/util/k8s"
	"github.com/onosproject/onos-operator/pkg/controller/util/leader"
	"github.com/onosproject/onos-operator/pkg/controller/util/ready"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"os"
	"runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
)

var log = logging.GetLogger("topo-operator")

func printVersion() {
	log.Info(fmt.Sprintf("Go Version: %s", runtime.Version()))
	log.Info(fmt.Sprintf("Go OS/Arch: %s/%s", runtime.GOOS, runtime.GOARCH))
}

func main() {
	printVersion()

	// Get a config to talk to the apiserver
	cfg, err := config.GetConfig()
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}

	// Become the leader before proceeding
	_ = leader.Become(context.TODO())

	r := ready.NewFileReady()
	err = r.Set()
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}
	defer func() {
		_ = r.Unset()
	}()

	opts := manager.Options{}
	scope := k8s.GetScope()
	if scope == k8s.NamespaceScope {
		opts.Namespace = k8s.GetNamespace()
	}

	// Create a new Cmd to provide shared dependencies and start components
	mgr, err := manager.New(cfg, opts)
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}
	mgr.GetClient()

	log.Info("Registering components")

	// Setup Scheme for all resources
	if err := topoapi.AddToScheme(mgr.GetScheme()); err != nil {
		log.Error(err)
		os.Exit(1)
	}

	// Add controllers to the manager
	if err := topoctrl.AddControllers(context.Background(), mgr); err != nil {
		log.Error(err)
		os.Exit(1)
	}

	// Setup all Controllers
	if err := entity.Add(mgr); err != nil {
		log.Error(err)
		os.Exit(1)
	}
	if err := kind.Add(mgr); err != nil {
		log.Error(err)
		os.Exit(1)
	}
	if err := relation.Add(mgr); err != nil {
		log.Error(err)
		os.Exit(1)
	}
	if err := service.Add(mgr); err != nil {
		log.Error(err)
		os.Exit(1)
	}

	log.Info("Starting the operator")

	// Start the Cmd
	if err := mgr.Start(signals.SetupSignalHandler()); err != nil {
		log.Error(err, "operator exited non-zero")
		os.Exit(1)
	}
}
