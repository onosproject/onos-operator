// SPDX-FileCopyrightText: 2020-present Open Networking Foundation <info@opennetworking.org>
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"github.com/onosproject/onos-lib-go/pkg/logging"
	appapi "github.com/onosproject/onos-operator/pkg/apis/app"
	appctrl "github.com/onosproject/onos-operator/pkg/controller/app/sidecar"
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

var log = logging.GetLogger("onos", "app")

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

	log.Info("Registering components")

	// Setup Scheme for all resources
	if err := appapi.AddToScheme(mgr.GetScheme()); err != nil {
		log.Error(err)
		os.Exit(1)
	}

	// Setup all Controllers
	if err := appctrl.AddProxyController(mgr); err != nil {
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
