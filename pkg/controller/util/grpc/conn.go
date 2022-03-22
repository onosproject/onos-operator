// SPDX-FileCopyrightText: 2020-present Open Networking Foundation <info@opennetworking.org>
//
// SPDX-License-Identifier: Apache-2.0

package grpc

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"github.com/onosproject/onos-lib-go/pkg/certs"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	corev1 "k8s.io/api/core/v1"
	"os"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ConnectAddress connects to a gRPC endpoint
func ConnectAddress(address string) (*grpc.ClientConn, error) {
	cert, err := tls.X509KeyPair([]byte(certs.DefaultClientCrt), []byte(certs.DefaultClientKey))
	if err != nil {
		return nil, err
	}
	tlsConfig := &tls.Config{
		Certificates:       []tls.Certificate{cert},
		InsecureSkipVerify: true,
	}
	return grpc.Dial(address, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
}

// ConnectService connects to a gRPC service by name
func ConnectService(c client.Client, namespace, name string) (*grpc.ClientConn, error) {
	// Locate the onos-config service
	services := &corev1.ServiceList{}
	if err := c.List(context.TODO(), services, client.InNamespace(namespace), client.MatchingLabels{"app": name}); err != nil {
		return nil, err
	} else if len(services.Items) == 0 {
		return nil, errors.New("service not found")
	}

	// Find the first matching ClusterIP service
	var service *corev1.Service
	var foundService corev1.Service
	for _, s := range services.Items {
		if s.Spec.Type == corev1.ServiceTypeClusterIP && s.Spec.ClusterIP != corev1.ClusterIPNone {
			foundService = s
			service = &foundService
			break
		}
	}

	// If no ClusterIP service was found, return an error
	if service == nil {
		return nil, errors.New("service not found")
	}

	clusterDomain := os.Getenv("CLUSTER_DOMAIN")
	if clusterDomain == "" {
		clusterDomain = "cluster.local"
	}
	return ConnectAddress(fmt.Sprintf("%s.%s.svc.%s:%d", service.Name, service.Namespace, clusterDomain, service.Spec.Ports[0].Port))
}
