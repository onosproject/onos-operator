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

package registry

import (
	"context"
	"encoding/json"
	"fmt"
	configv1beta1 "github.com/onosproject/onos-operator/pkg/apis/config/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	"strconv"
	"strings"
)

const (
	// RegistryInjectAnnotation is an annotation indicating the model to inject the registry into a pod
	RegistryInjectAnnotation = "registry.config.onosproject.org/inject"
	// RegistryNamespaceAnnotation is an annotation indicating the registry namespace
	RegistryNamespaceAnnotation = "registry.config.onosproject.org/namespace"
	// RegistryNamespaceAnnotation is an annotation indicating the registry name
	RegistryNameAnnotation = "registry.config.onosproject.org/name"
	// RegistryInjectStatusAnnotation is an annotation indicating the status of registry injection
	RegistryInjectStatusAnnotation = "registry.config.onosproject.org/inject-status"
	// RegistryInjectStatusInjeceted is an annotation value indicating the registry has been injected
	RegistryInjectStatusInjected = "injected"
	// RegistryPathAnnotation is an annotation indicating the path at which to mount the registry
	RegistryPathAnnotation = "registry.config.onosproject.org/path"
	// ModelAPIVersionAnnotation is an annotation indicating the model API version
	ModelAPIVersionAnnotation = "plugin.config.onosproject.org/api-version"
	// GolangBuildVersionAnnotation is an annotation indicating the onosproject/go-build version for which to compile a model
	GolangBuildVersionAnnotation = "plugin.config.onosproject.org/golang-build-version"
	// GoModTargetAnnotation is an annotation indicating the Go module for which to compile a model
	GoModTargetAnnotation = "plugin.config.onosproject.org/go-mod-target"
	// GoModReplaceAnnotation is an annotation indicating a replacement for the target Go module
	GoModReplaceAnnotation = "plugin.config.onosproject.org/go-mod-replace"
)

const (
	modelPath           = "/etc/onos/models"
	buildPath           = "/build"
	defaultGoModTarget  = "github.com/onosproject/onos-config"
	defaultRegistryPath = "/etc/onos/plugins"
)

// RegistryInjector is a mutating webhook for injecting the registry container into pods
type RegistryInjector struct {
	client  client.Client
	decoder *admission.Decoder
}

// InjectDecoder :
func (i *RegistryInjector) InjectDecoder(decoder *admission.Decoder) error {
	i.decoder = decoder
	return nil
}

// Handle :
func (i *RegistryInjector) Handle(ctx context.Context, request admission.Request) admission.Response {
	log.Infof("Received admission request for Pod '%s/%s'", request.Name, request.Namespace)

	// Decode the pod
	pod := &corev1.Pod{}
	if err := i.decoder.Decode(request, pod); err != nil {
		log.Errorf("Failed to inject registry into Pod '%s/%s': %s", pod.Name, pod.Namespace, err)
		return admission.Errored(http.StatusBadRequest, err)
	}

	// Determine whether registry injection is enabled for this pod
	injectRegistry, ok := pod.Annotations[RegistryInjectAnnotation]
	if !ok {
		log.Debugf("Skipping registry injection for Pod '%s/%s': '%s' annotation not found", pod.Name, pod.Namespace, RegistryInjectAnnotation)
		return admission.Allowed(fmt.Sprintf("'%s' annotation not found", RegistryInjectAnnotation))
	}
	if inject, err := strconv.ParseBool(injectRegistry); err != nil {
		log.Debugf("Skipping registry injection for Pod '%s/%s': '%s' annotation could not be parsed (%v)", pod.Name, pod.Namespace, RegistryInjectAnnotation, err)
		return admission.Allowed(fmt.Sprintf("'%s' annotation could not be parsed", RegistryInjectAnnotation))
	} else if !inject {
		log.Debugf("Skipping registry injection for Pod '%s/%s': '%s' is false", pod.Name, pod.Namespace, RegistryInjectAnnotation)
		return admission.Allowed(fmt.Sprintf("'%s' annotation is false", RegistryInjectAnnotation))
	}

	// Skip registry injection if the registry has already been injected
	injectStatus, ok := pod.Annotations[RegistryInjectStatusAnnotation]
	if ok && injectStatus == RegistryInjectStatusInjected {
		log.Debugf("Skipping registry injection for Pod '%s/%s': '%s' is '%s'", pod.Name, pod.Namespace, RegistryInjectStatusAnnotation, injectStatus)
		return admission.Allowed(fmt.Sprintf("'%s' annotation is '%s'", RegistryInjectStatusAnnotation, injectStatus))
	}

	// Get the model API version
	modelAPIVersion, ok := pod.Annotations[ModelAPIVersionAnnotation]
	if !ok {
		log.Errorf("Failed to inject registry into Pod '%s/%s': '%s' annotation not found", pod.Name, pod.Namespace, ModelAPIVersionAnnotation)
		return admission.Denied(fmt.Sprintf("'%s' annotation not found", ModelAPIVersionAnnotation))
	}

	golangBuildVersion := pod.Annotations[GolangBuildVersionAnnotation]
	goModTarget := pod.Annotations[GoModTargetAnnotation]
	if goModTarget == "" {
		goModTarget = defaultGoModTarget
	}
	goModReplace := pod.Annotations[GoModReplaceAnnotation]
	registryPath, ok := pod.Annotations[RegistryPathAnnotation]
	if !ok {
		registryPath = defaultRegistryPath
	}

	// Get the registry namespace and name
	registryNamespace, ok := pod.Annotations[RegistryNamespaceAnnotation]
	if !ok || registryNamespace == "" {
		registryNamespace = request.Namespace
	}
	registryName, ok := pod.Annotations[RegistryNameAnnotation]
	if !ok || registryName == "" {
		log.Errorf("Failed to inject registry into Pod '%s/%s': '%s' annotation not found", pod.Name, pod.Namespace, RegistryNameAnnotation)
		return admission.Denied(fmt.Sprintf("'%s' annotation not found", RegistryNameAnnotation))
	}

	log.Infof("Injecting registry '%s/%s' into Pod '%s/%s'", registryName, registryNamespace, pod.Name, pod.Namespace)

	// Load the registry to inject
	registry := &configv1beta1.ModelRegistry{}
	registryNamespacedName := types.NamespacedName{
		Namespace: registryNamespace,
		Name:      registryName,
	}
	if err := i.client.Get(ctx, registryNamespacedName, registry); err != nil {
		log.Errorf("Failed to inject registry into Pod '%s/%s': %s", pod.Name, pod.Namespace, err)
		return admission.Denied(err.Error())
	}

	// Add a registry volume to the pod
	if !hasVolume(pod, registry.Spec.Volume.Name) {
		pod.Spec.Volumes = append(pod.Spec.Volumes, registry.Spec.Volume)
	}

	// Load existing models via init containers
	var models []configv1beta1.Model
	modelList := &configv1beta1.ModelList{}
	modelListOpts := &client.ListOptions{
		Namespace: request.Namespace,
	}
	if err := i.client.List(context.Background(), modelList, modelListOpts); err != nil {
		log.Errorf("Failed to inject models into Pod '%s/%s': %s", pod.Name, pod.Namespace, err)
		return admission.Errored(http.StatusInternalServerError, err)
	}

	for _, model := range modelList.Items {
		if model.Spec.Plugin != nil {
			models = append(models, model)
		}
	}

	for _, model := range models {
		log.Infof("Injecting model '%s' into Pod '%s/%s'", model.Name, pod.Name, pod.Namespace)

		// Load the model files
		files, err := i.getModelFiles(model)
		if err != nil {
			log.Errorf("Failed to inject model '%s' into Pod '%s/%s': %s", model.Name, pod.Name, pod.Namespace, err)
			if errors.IsNotFound(err) {
				log.Warnf("Failed to inject model '%s/%s' into Pod '%s/%s': %s", model.Name, model.Namespace, pod.Name, pod.Namespace, err)
				return admission.Denied(fmt.Sprintf("Model '%s' not initialized", model.Name))
			}
			log.Errorf("Failed to inject model '%s/%s' into Pod '%s/%s': %s", model.Name, model.Namespace, pod.Name, pod.Namespace, err)
			return admission.Errored(http.StatusInternalServerError, err)
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

		// Add the model volume to the pod
		pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{
			Name: model.Name,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: model.Name,
					},
				},
			},
		})

		args := []string{
			"--name",
			model.Spec.Plugin.Type,
			"--version",
			model.Spec.Plugin.Version,
			"--build-path",
			buildPath,
			"--output-path",
			registryPath,
		}

		if goModTarget != "" {
			args = append(args, "--target", goModTarget)
		}

		if goModReplace != "" {
			args = append(args, "--replace", goModReplace)
		}

		// Add module arguments
		for module, file := range files {
			args = append(args, "--module", fmt.Sprintf("%s=%s/%s", module, modelPath, file))
		}

		var tags []string
		if modelAPIVersion != "" {
			tags = append(tags, modelAPIVersion)
		}
		if golangBuildVersion != "" {
			tags = append(tags, fmt.Sprintf("golang-build-%s", golangBuildVersion))
		}
		image := fmt.Sprintf("onosproject/config-model-compiler:%s", strings.Join(tags, "-"))

		// Add the compiler init container
		container := corev1.Container{
			Name:  fmt.Sprintf("%s-%s-compiler", strings.ToLower(model.Spec.Plugin.Type), strings.ReplaceAll(model.Spec.Plugin.Version, ".", "-")),
			Image: image,
			Args:  args,
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      model.Name,
					MountPath: modelPath,
				},
				{
					Name:      registry.Spec.Volume.Name,
					MountPath: registryPath,
				},
			},
		}

		// If the model is present, inject the init container into the pod
		pod.Spec.InitContainers = append(pod.Spec.InitContainers, container)
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
	if modelAPIVersion != "" {
		tags = append(tags, modelAPIVersion)
	}
	if golangBuildVersion != "" {
		tags = append(tags, fmt.Sprintf("golang-build-%s", golangBuildVersion))
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

	// Set the registry injection status to injected
	pod.Annotations[RegistryInjectStatusAnnotation] = RegistryInjectStatusInjected

	// Marshal the pod and return a patch response
	marshaledPod, err := json.Marshal(pod)
	if err != nil {
		log.Errorf("Failed to inject registry '%s/%s' into Pod '%s/%s': %s", registryName, registryNamespace, pod.Name, pod.Namespace, err)
		return admission.Errored(http.StatusInternalServerError, err)
	}

	log.Infof("Completed injecting registry '%s/%s' into Pod '%s/%s'", registryName, registryNamespace, pod.Name, pod.Namespace)
	return admission.PatchResponseFromRaw(request.Object.Raw, marshaledPod)
}

func (i *RegistryInjector) getModelFiles(model configv1beta1.Model) (map[string]string, error) {
	files := make(map[string]string)
	for _, module := range model.Spec.Modules {
		name := fmt.Sprintf("%s@%s", module.Name, module.Version)
		file := fmt.Sprintf("%s-%s.yang", module.Name, module.Version)
		files[name] = file
	}

	for _, dep := range model.Spec.Dependencies {
		ns := dep.Namespace
		if ns == "" {
			ns = model.Namespace
		}
		modelDep := configv1beta1.Model{}
		modelDepName := types.NamespacedName{
			Name:      dep.Name,
			Namespace: ns,
		}
		if err := i.client.Get(context.Background(), modelDepName, &modelDep); err != nil {
			return nil, err
		}

		refFiles, err := i.getModelFiles(modelDep)
		if err != nil {
			return nil, err
		}
		for name, value := range refFiles {
			files[name] = value
		}
	}
	return files, nil
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
