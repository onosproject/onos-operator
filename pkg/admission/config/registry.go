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
	corev1 "k8s.io/api/core/v1"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	"strings"
)

const (
	// InjectRegistryAnnotation is an annotation indicating the model to inject the registry into a pod
	InjectRegistryAnnotation = "config.onosproject.org/inject-registry"
	// RegistryVersionAnnotation is an annotation indicating the path at which to mount the registry
	RegistryPathAnnotation = "config.onosproject.org/registry-path"
)

const (
	registryVolume = "registry"
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

	// If the pod is annotated with the InjectRegistryAnnotation, inject the module registry
	modelInject, ok := pod.Annotations[InjectRegistryAnnotation]
	if !ok || modelInject != "true" {
		return admission.Allowed(fmt.Sprintf("'%s' annotation not found", InjectRegistryAnnotation))
	}
	log.Infof("Injecting registry sidecar into Pod '%s/%s'", pod.Name, pod.Namespace)

	// If the pod is annotated with InjectModelAnnotation, ensure RegistryLanguageAnnotation
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

	// Add a registry volume to the pod
	if !hasRegistryVolume(pod) {
		pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{
			Name: registryVolume,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		})
	}

	// Mount the registry volume to existing containers
	for i, container := range pod.Spec.Containers {
		if !hasRegistryVolumeMount(container) {
			container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
				Name:      registryVolume,
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
		Name:  fmt.Sprintf("model-registry"),
		Image: image,
		Args:  args,
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      registryVolume,
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

func hasRegistryVolume(pod *corev1.Pod) bool {
	for _, volume := range pod.Spec.Volumes {
		if volume.Name == registryVolume {
			return true
		}
	}
	return false
}

func hasRegistryVolumeMount(container corev1.Container) bool {
	for _, mount := range container.VolumeMounts {
		if mount.Name == registryVolume {
			return true
		}
	}
	return false
}

var _ admission.Handler = &RegistryInjector{}
var _ admission.DecoderInjector = &RegistryInjector{}
