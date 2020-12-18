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

package config

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/onosproject/onos-operator/pkg/apis/config/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	"strings"
)

const (
	// RegistryInjectAnnotation is an annotation indicating the model to inject the registry into a pod
	RegistryInjectAnnotation = "registry.config.onosproject.org/inject"
	// RegistryVersionAnnotation is an annotation indicating the path at which to mount the registry
	RegistryPathAnnotation = "registry.config.onosproject.org/path"
)

const (
	defaultRegistryPath = "/etc/onos/plugins"
)

// RegistryInjector is a mutating webhook for injecting the registry container into pods
type RegistryInjector struct {
	client  client.Client
	decoder *admission.Decoder
}

func (i *RegistryInjector) InjectDecoder(decoder *admission.Decoder) error {
	i.decoder = decoder
	return nil
}

func (i *RegistryInjector) Handle(ctx context.Context, request admission.Request) admission.Response {
	log.Infof("Received admission request for Pod '%s/%s'", request.Name, request.Namespace)

	// Decode the pod
	pod := &corev1.Pod{}
	if err := i.decoder.Decode(request, pod); err != nil {
		log.Errorf("Failed to inject registry into Pod '%s/%s': %s", pod.Name, pod.Namespace, err)
		return admission.Errored(http.StatusBadRequest, err)
	}

	// If the pod is annotated with the RegistryInjectAnnotation, inject the module registry
	registryInject, ok := pod.Annotations[RegistryInjectAnnotation]
	if !ok {
		return admission.Allowed(fmt.Sprintf("'%s' annotation not found", RegistryInjectAnnotation))
	}
	log.Infof("Injecting registry '%s' sidecar into Pod '%s/%s'", registryInject, pod.Name, pod.Namespace)

	// If the pod is annotated with ModelInjectAnnotation, ensure RegistryLanguageAnnotation
	// and RegistryVersionAnnotation are present as well
	compilerLanguage, ok := pod.Annotations[CompilerLanguageAnnotation]
	if !ok {
		log.Errorf("Failed to inject registry into Pod '%s/%s': '%s' annotation not found", pod.Name, pod.Namespace, CompilerLanguageAnnotation)
		return admission.Denied(fmt.Sprintf("'%s' annotation not found", CompilerLanguageAnnotation))
	}
	compilerVersion, ok := pod.Annotations[CompilerVersionAnnotation]
	if !ok {
		log.Errorf("Failed to inject registry into Pod '%s/%s': '%s' annotation not found", pod.Name, pod.Namespace, CompilerVersionAnnotation)
		return admission.Denied(fmt.Sprintf("'%s' annotation not found", CompilerVersionAnnotation))
	}
	goBuildVersion := pod.Annotations[CompilerGolangBuildVersionAnnotation]
	goModTarget := pod.Annotations[CompilerGoModTargetAnnotation]
	goModReplace := pod.Annotations[CompilerGoModReplaceAnnotation]
	registryPath, ok := pod.Annotations[RegistryPathAnnotation]
	if !ok {
		registryPath = defaultRegistryPath
	}

	// Load the registry to inject
	registry := &v1beta1.ModelRegistry{}
	registryName := types.NamespacedName{
		Namespace: request.Namespace,
		Name:      registryInject,
	}
	if err := i.client.Get(ctx, registryName, registry); err != nil {
		log.Errorf("Failed to inject registry into Pod '%s/%s': %s", pod.Name, pod.Namespace, err)
		return admission.Denied(err.Error())
	}

	// Add a registry volume to the pod
	if !hasVolume(pod, registry.Spec.Volume.Name) {
		pod.Spec.Volumes = append(pod.Spec.Volumes, registry.Spec.Volume)
	}

	// Mount the registry volume to existing containers
	for i, container := range pod.Spec.Containers {
		if !hasVolumeMount(container, registry.Spec.Volume.Name) {
			container.Env = append(container.Env, corev1.EnvVar{
				Name:  "CONFIG_MODEL_REGISTRY",
				Value: registryPath,
			})
			container.Env = append(container.Env, corev1.EnvVar{
				Name:  "CONFIG_MODULE_TARGET",
				Value: goModTarget,
			})
			container.Env = append(container.Env, corev1.EnvVar{
				Name:  "CONFIG_MODULE_REPLACE",
				Value: goModReplace,
			})
			container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
				Name:      registry.Spec.Volume.Name,
				MountPath: registryPath,
			})
			pod.Spec.Containers[i] = container
		}
	}

	args := []string{
		"--build-path",
		buildPath,
		"--registry-path",
		registryPath,
	}

	if goModTarget != "" {
		args = append(args, "--target", goModTarget)
	}

	if goModReplace != "" {
		args = append(args, "--replace", goModReplace)
	}

	var tags []string
	if compilerLanguage != "" {
		tags = append(tags, compilerLanguage)
	}
	if compilerVersion != "" {
		tags = append(tags, compilerVersion)
	}
	if goBuildVersion != "" {
		tags = append(tags, fmt.Sprintf("build%s", goBuildVersion))
	}
	image := fmt.Sprintf("onosproject/config-model-registry:%s", strings.Join(tags, "-"))

	// Add the registry init container
	container := corev1.Container{
		Name:  "model-registry",
		Image: image,
		Args:  args,
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      registry.Spec.Volume.Name,
				MountPath: registryPath,
			},
		},
	}

	// If the model is present, inject the init container into the pod
	pod.Spec.Containers = append(pod.Spec.Containers, container)

	// Marshal the pod and return a patch response
	marshaledPod, err := json.Marshal(pod)
	if err != nil {
		log.Errorf("Failed to inject registry into Pod '%s/%s': %s", pod.Name, pod.Namespace, err)
		return admission.Errored(http.StatusInternalServerError, err)
	}
	return admission.PatchResponseFromRaw(request.Object.Raw, marshaledPod)
}

func hasVolume(pod *corev1.Pod, name string) bool {
	for _, volume := range pod.Spec.Volumes {
		if volume.Name == name {
			return true
		}
	}
	return false
}

func hasVolumeMount(container corev1.Container, name string) bool {
	for _, mount := range container.VolumeMounts {
		if mount.Name == name {
			return true
		}
	}
	return false
}

var _ admission.Handler = &RegistryInjector{}
var _ admission.DecoderInjector = &RegistryInjector{}
