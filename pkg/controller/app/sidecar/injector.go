// SPDX-FileCopyrightText: 2020-present Open Networking Foundation <info@opennetworking.org>
//
// SPDX-License-Identifier: Apache-2.0

package sidecar

import (
	"context"
	"fmt"
	"github.com/onosproject/onos-lib-go/pkg/logging"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/json"
	"net/http"
	"os"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"strconv"
)

var log = logging.GetLogger("app", "sidecar")

const (
	proxyNodeEnv      = "ONOS_PROXY_NODE"
	proxyNamespaceEnv = "ONOS_PROXY_NAMESPACE"
	proxyNameEnv      = "ONOS_PROXY_NAME"
)

const (
	proxyInjectPath              = "/inject-proxy"
	proxyInjectAnnotation        = "proxy.onosproject.org/inject"
	proxyInjectStatusAnnotation  = "proxy.onosproject.org/status"
	injectedStatus               = "injected"
	proxyCPULimitAnnotation      = "proxy.onosproject.org/cpu-limit"
	proxyMemoryLimitAnnotation   = "proxy.onosproject.org/memory-limit"
	proxyCPURequestAnnotation    = "proxy.onosproject.org/cpu-request"
	proxyMemoryRequestAnnotation = "proxy.onosproject.org/memory-request"
)

const (
	defaultProxyImageEnv = "DEFAULT_PROXY_IMAGE"
	defaultProxyImage    = "onosproject/onos-proxy:latest"
)

func getDefaultProxyImage() string {
	image := os.Getenv(defaultProxyImageEnv)
	if image == "" {
		image = defaultProxyImage
	}
	return image
}

// AddProxyController adds the application proxy controller to the manager
func AddProxyController(mgr manager.Manager) error {
	mgr.GetWebhookServer().Register(proxyInjectPath, &webhook.Admission{
		Handler: &ProxyInjector{
			client: mgr.GetClient(),
			scheme: mgr.GetScheme(),
		},
	})
	return nil
}

// ProxyInjector is a mutating webhook that injects the broker container into pods
type ProxyInjector struct {
	client  client.Client
	scheme  *runtime.Scheme
	decoder *admission.Decoder
}

// InjectDecoder :
func (i *ProxyInjector) InjectDecoder(decoder *admission.Decoder) error {
	i.decoder = decoder
	return nil
}

// Handle :
func (i *ProxyInjector) Handle(_ context.Context, request admission.Request) admission.Response {
	podNamespacedName := types.NamespacedName{
		Namespace: request.Namespace,
		Name:      request.Name,
	}
	log.Infof("Received admission request for Pod '%s'", podNamespacedName)

	// Decode the pod
	pod := &corev1.Pod{}
	if err := i.decoder.Decode(request, pod); err != nil {
		log.Errorf("Could not decode Pod '%s'", podNamespacedName, err)
		return admission.Errored(http.StatusBadRequest, err)
	}

	injectBroker, ok := pod.Annotations[proxyInjectAnnotation]
	if !ok {
		log.Infof("Skipping proxy injection for Pod '%s'", podNamespacedName)
		return admission.Allowed(fmt.Sprintf("'%s' annotation not found", proxyInjectAnnotation))
	}
	if inject, err := strconv.ParseBool(injectBroker); err != nil {
		log.Errorf("Broker injection failed for Pod '%s'", podNamespacedName, err)
		return admission.Allowed(fmt.Sprintf("'%s' annotation could not be parsed", proxyInjectAnnotation))
	} else if !inject {
		log.Infof("Skipping proxy injection for Pod '%s'", podNamespacedName)
		return admission.Allowed(fmt.Sprintf("'%s' annotation is false", proxyInjectAnnotation))
	}

	injectedBroker, ok := pod.Annotations[proxyInjectStatusAnnotation]
	if ok && injectedBroker == injectedStatus {
		log.Infof("Skipping proxy injection for Pod '%s'", podNamespacedName)
		return admission.Allowed(fmt.Sprintf("'%s' annotation is '%s'", proxyInjectStatusAnnotation, injectedBroker))
	}

	cpuLimit := pod.Annotations[proxyCPULimitAnnotation]
	memoryLimit := pod.Annotations[proxyMemoryLimitAnnotation]

	limits := corev1.ResourceList{}

	if cpuLimit != "" {
		limits[corev1.ResourceCPU] = resource.MustParse(cpuLimit)
	}
	if memoryLimit != "" {
		limits[corev1.ResourceMemory] = resource.MustParse(memoryLimit)
	}

	requests := corev1.ResourceList{}

	cpuRequest := pod.Annotations[proxyCPURequestAnnotation]
	memoryRequest := pod.Annotations[proxyMemoryRequestAnnotation]

	if cpuRequest != "" {
		requests[corev1.ResourceCPU] = resource.MustParse(cpuRequest)
	}
	if memoryRequest != "" {
		requests[corev1.ResourceMemory] = resource.MustParse(memoryRequest)
	}

	pod.Spec.Containers = append(pod.Spec.Containers, corev1.Container{
		Name:            "onos-proxy",
		Image:           getDefaultProxyImage(),
		ImagePullPolicy: corev1.PullIfNotPresent,
		Resources: corev1.ResourceRequirements{
			Limits:   limits,
			Requests: requests,
		},
		Env: []corev1.EnvVar{
			{
				Name: proxyNamespaceEnv,
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: "metadata.namespace",
					},
				},
			},
			{
				Name: proxyNameEnv,
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: "metadata.name",
					},
				},
			},
			{
				Name: proxyNodeEnv,
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: "spec.nodeName",
					},
				},
			},
		},
	})
	pod.Annotations[proxyInjectStatusAnnotation] = injectedStatus

	// Marshal the pod and return a patch response
	marshaledPod, err := json.Marshal(pod)
	if err != nil {
		log.Errorf("Broker injection failed for Pod '%s'", podNamespacedName, err)
		return admission.Errored(http.StatusInternalServerError, err)
	}
	return admission.PatchResponseFromRaw(request.Object.Raw, marshaledPod)
}

var _ admission.Handler = &ProxyInjector{}
