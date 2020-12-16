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
	configv1beta1 "github.com/onosproject/onos-operator/pkg/apis/config/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	"strings"
)

const (
	// InjectModelAnnotation is an annotation indicating the model to inject a model into a pod
	InjectModelAnnotation = "config.onosproject.org/inject-model"
	// CompilerLanguageAnnotation is an annotation indicating which compiler language to use to compile a model
	CompilerLanguageAnnotation = "compiler.config.onosproject.org/language"
	// CompilerVersionAnnotation is an annotation indicating which compiler version to use to compile a model
	CompilerVersionAnnotation = "compiler.config.onosproject.org/version"
	// CompilerGoVersionAnnotation is an annotation indicating the Go version for which to compile a model
	CompilerGoVersionAnnotation = "compiler.config.onosproject.org/go-version"
	// CompilerGolangBuildVersionAnnotation is an annotation indicating the onosproject/go-build version for which to compile a model
	CompilerGolangBuildVersionAnnotation = "compiler.config.onosproject.org/golang-build-version"
	// CompilerGoModTargetAnnotation is an annotation indicating the Go module for which to compile a model
	CompilerGoModTargetAnnotation  = "compiler.config.onosproject.org/go-mod-target"
	// CompilerGoModReplaceAnnotation is an annotation indicating a replacement for the target Go module
	CompilerGoModReplaceAnnotation = "compiler.config.onosproject.org/go-mod-replace"
)

const (
	goLanguage = "go"
	modelPath  = "/etc/onos/models"
	buildPath  = "/build"
)

// CompilerInjector is a mutating webhook for injecting the compiler container into pods
type CompilerInjector struct {
	client  client.Client
	decoder *admission.Decoder
}

func (i *CompilerInjector) InjectDecoder(decoder *admission.Decoder) error {
	i.decoder = decoder
	return nil
}

func (i *CompilerInjector) Handle(ctx context.Context, request admission.Request) admission.Response {
	log.Infof("Received admission request for Pod '%s/%s'", request.Name, request.Namespace)

	// Decode the pod
	pod := &corev1.Pod{}
	if err := i.decoder.Decode(request, pod); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	// If the pod is annotated with InjectModelAnnotation, ensure CompilerLanguageAnnotation
	// and CompilerVersionAnnotation are present as well
	compilerLanguage, ok := pod.Annotations[CompilerLanguageAnnotation]
	if !ok {
		log.Debugf("Skipping plugin injection for Pod '%s/%s': '%s' annotation not found", pod.Name, pod.Namespace, CompilerLanguageAnnotation)
		return admission.Allowed(fmt.Sprintf("'%s' annotation not found", CompilerLanguageAnnotation))
	}
	compilerVersion, ok := pod.Annotations[CompilerVersionAnnotation]
	if !ok {
		log.Debugf("Skipping plugin injection for Pod '%s/%s': '%s' annotation not found", pod.Name, pod.Namespace, CompilerVersionAnnotation)
		return admission.Allowed(fmt.Sprintf("'%s' annotation not found", CompilerVersionAnnotation))
	}
	golangBuildVersion := pod.Annotations[CompilerGolangBuildVersionAnnotation]
	goModTarget := pod.Annotations[CompilerGoModTargetAnnotation]
	goModReplace := pod.Annotations[CompilerGoModReplaceAnnotation]
	registryPath, ok := pod.Annotations[RegistryPathAnnotation]
	if !ok {
		registryPath = defaultRegistryPath
	}

	// Load the models to be compiled
	var models []configv1beta1.Model
	if registryInject, ok := pod.Annotations[InjectRegistryAnnotation]; ok && registryInject == "true" {
		// Load existing models via init containers
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
	} else if modelInject, ok := pod.Annotations[InjectModelAnnotation]; ok && modelInject != "" {
		model := &configv1beta1.Model{}
		modelName := types.NamespacedName{
			Name:      modelInject,
			Namespace: request.Namespace,
		}
		if err := i.client.Get(ctx, modelName, model); err != nil {
			log.Errorf("Failed to inject model '%s' into Pod '%s/%s': %s", modelInject, pod.Name, pod.Namespace, err)
			if errors.IsNotFound(err) {
				return admission.Denied(fmt.Sprintf("Model '%s' not found", modelName))
			}
			return admission.Errored(http.StatusInternalServerError, err)
		}

		if model.Spec.Plugin != nil {
			models = append(models, *model)
		}
	} else {
		return admission.Allowed("model annotations not found")
	}

	// Only Go is supported
	if compilerLanguage != goLanguage {
		return admission.Denied(fmt.Sprintf("Unsupported compiler language '%s'", compilerLanguage))
	}

	for _, model := range models {
		log.Infof("Injecting model '%s' into Pod '%s/%s'", model.Name, pod.Name, pod.Namespace)

		// Load the model files
		files, err := i.getModelFiles(model)
		if err != nil {
			log.Errorf("Failed to inject model '%s' into Pod '%s/%s': %s", model.Name, pod.Name, pod.Namespace, err)
			if errors.IsNotFound(err) {
				return admission.Denied(fmt.Sprintf("Model '%s' not initialized", model.Name))
			}
			return admission.Errored(http.StatusInternalServerError, err)
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
		if compilerLanguage != "" {
			tags = append(tags, compilerLanguage)
		}
		if compilerVersion != "" {
			tags = append(tags, compilerVersion)
		}
		if golangBuildVersion != "" {
			tags = append(tags, fmt.Sprintf("build-%s", golangBuildVersion))
		}
		image := fmt.Sprintf("onosproject/config-model-compiler:%s", strings.Join(tags, "-"))

		// Add the compiler init container
		container := corev1.Container{
			Name:  fmt.Sprintf("%s-%s-compiler", model.Spec.Plugin.Type, strings.ReplaceAll(model.Spec.Plugin.Version, ".", "-")),
			Image: image,
			Args:  args,
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      model.Name,
					MountPath: modelPath,
				},
				{
					Name:      registryVolume,
					MountPath: registryPath,
				},
			},
		}

		// If the model is present, inject the init container into the pod
		pod.Spec.InitContainers = append(pod.Spec.InitContainers, container)
	}

	// Marshal the pod and return a patch response
	marshaledPod, err := json.Marshal(pod)
	if err != nil {
		log.Errorf("Failed to inject models into Pod '%s/%s': %s", pod.Name, pod.Namespace, err)
		return admission.Errored(http.StatusInternalServerError, err)
	}
	return admission.PatchResponseFromRaw(request.Object.Raw, marshaledPod)
}

func (i *CompilerInjector) getModelFiles(model configv1beta1.Model) (map[string]string, error) {
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

var _ admission.Handler = &RegistryInjector{}
var _ admission.DecoderInjector = &RegistryInjector{}
