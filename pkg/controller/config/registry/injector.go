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
	"fmt"
	configv1beta1 "github.com/onosproject/onos-operator/pkg/apis/config/v1beta1"
	"github.com/rogpeppe/go-internal/module"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
)

const (
	// RegistryInjectAnnotation is an annotation indicating the model to inject the registry into a pod
	RegistryInjectAnnotation = "registry.config.onosproject.org/inject"
	// RegistryInjectStatusAnnotation is an annotation indicating the status of registry injection
	RegistryInjectStatusAnnotation = "registry.config.onosproject.org/inject-status"
	// RegistryInjectStatusInjected is an annotation value indicating the registry has been injected
	RegistryInjectStatusInjected = "injected"
	// CompilerVersionAnnotation is an annotation indicating the model API version
	CompilerVersionAnnotation = "compiler.config.onosproject.org/version"
	// CompilerTargetAnnotation is an annotation indicating the Go module for which to compile a model
	CompilerTargetAnnotation = "compiler.config.onosproject.org/target"
)

const (
	buildPath          = "/etc/onos/build"
	moduleVolumeName   = "plugin-module"
	registryVolumeName = "model-registry"
	pluginsVolumeName  = "plugin-cache"
	goCacheVolumeName  = "mod-cache"
	defaultGoModTarget = "github.com/onosproject/onos-config"
	registryPath       = "/etc/onos/registry"
	modulePath         = "/etc/onos/mod"
	pluginsPath        = "/etc/onos/plugins"
	goCachePath        = "/go/pkg/mod/cache"
)

func newInjector(client client.Client, namespace string) *Injector {
	return &Injector{
		client:    client,
		namespace: namespace,
	}
}

// Injector is a mutating webhook for injecting the registry container into pods
type Injector struct {
	client    client.Client
	namespace string
}

func (i *Injector) inject(ctx context.Context, pod *corev1.Pod) (bool, error) {
	// Determine whether registry injection is enabled for this pod
	injectRegistry, ok := pod.Annotations[RegistryInjectAnnotation]
	if !ok || injectRegistry == "" {
		log.Debugf("Skipping registry injection for Pod '%s/%s': '%s' annotation not found", pod.Name, pod.Namespace, RegistryInjectAnnotation)
		return false, nil
	}

	// Skip registry injection if the registry has already been injected
	injectStatus, ok := pod.Annotations[RegistryInjectStatusAnnotation]
	if ok && injectStatus == RegistryInjectStatusInjected {
		log.Debugf("Skipping registry injection for Pod '%s/%s': '%s' is '%s'", pod.Name, pod.Namespace, RegistryInjectStatusAnnotation, injectStatus)
		return false, nil
	}

	if err := i.injectRegistry(ctx, pod); err != nil {
		return true, err
	}

	// Set the registry injection status to injected
	pod.Annotations[RegistryInjectStatusAnnotation] = RegistryInjectStatusInjected
	return true, nil
}

func (i *Injector) injectRegistry(ctx context.Context, pod *corev1.Pod) error {
	registryName, err := i.getRegistryName(pod)
	if err != nil {
		return err
	}
	compilerVersion, err := i.getCompilerVersion(pod)
	if err != nil {
		return err
	}
	modTarget, err := i.getModTarget(pod)
	if err != nil {
		return err
	}
	modReplace, err := i.getModReplace(pod)
	if err != nil {
		return err
	}

	log.Infof("Injecting registry '%s' into Pod '%s/%s'", registryName, pod.Name, pod.Namespace)

	// Load the registry to inject
	registry := &configv1beta1.ModelRegistry{}
	registryNamespacedName := types.NamespacedName{
		Namespace: i.namespace,
		Name:      registryName,
	}
	if err := i.client.Get(ctx, registryNamespacedName, registry); err != nil {
		return err
	}

	// Add a registry volume to the pod
	registryVolume := corev1.Volume{
		Name: registryVolumeName,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	}
	pod.Spec.Volumes = append(pod.Spec.Volumes, registryVolume)

	// Add the cache volume to the pod
	var cacheVolume corev1.Volume
	if registry.Spec.Cache.Volume != nil {
		cacheVolume = corev1.Volume{
			Name:         pluginsVolumeName,
			VolumeSource: registry.Spec.Cache.Volume.VolumeSource,
		}
	} else {
		cacheVolume = corev1.Volume{
			Name: pluginsVolumeName,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		}
	}
	pod.Spec.Volumes = append(pod.Spec.Volumes, cacheVolume)

	// Mount the registry volume to existing containers
	for j, container := range pod.Spec.Containers {
		container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
			Name:      moduleVolumeName,
			MountPath: modulePath,
		})
		container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
			Name:      registryVolumeName,
			MountPath: registryPath,
		})
		container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
			Name:      pluginsVolumeName,
			MountPath: pluginsPath,
		})
		pod.Spec.Containers[j] = container
	}

	args := []string{
		"--mod-path",
		modulePath,
		"--build-path",
		buildPath,
		"--registry-path",
		registryPath,
		"--cache-path",
		pluginsPath,
	}

	if modTarget != "" {
		args = append(args, "--mod-target", modTarget)
	}

	if modReplace != "" {
		args = append(args, "--mod-replace", modReplace)
	}

	var tags []string
	if compilerVersion != "" {
		tags = append(tags, compilerVersion)
	}
	image := fmt.Sprintf("onosproject/config-model-registry:%s", strings.Join(tags, "-"))

	// Add the registry sidecar container
	container := corev1.Container{
		Name:            "model-registry",
		Image:           image,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Args:            args,
		ReadinessProbe: &corev1.Probe{
			Handler: corev1.Handler{
				TCPSocket: &corev1.TCPSocketAction{
					Port: intstr.FromInt(5151),
				},
			},
			PeriodSeconds: 1,
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      moduleVolumeName,
				MountPath: modulePath,
			},
			{
				Name:      goCacheVolumeName,
				MountPath: goCachePath,
			},
			{
				Name:      registryVolumeName,
				MountPath: registryPath,
			},
			{
				Name:      pluginsVolumeName,
				MountPath: pluginsPath,
			},
		},
	}

	// If the model is present, inject the init container into the pod
	pod.Spec.Containers = append(pod.Spec.Containers, container)
	return nil
}

func (i *Injector) getCompilerVersion(pod *corev1.Pod) (string, error) {
	compilerVersion, ok := pod.Annotations[CompilerVersionAnnotation]
	if !ok {
		return "", fmt.Errorf("'%s' annotation not found", CompilerVersionAnnotation)
	}
	return compilerVersion, nil
}

func (i *Injector) getCompilerTarget(pod *corev1.Pod) (string, error) {
	compilerTarget := pod.Annotations[CompilerTargetAnnotation]
	return compilerTarget, nil
}

func (i *Injector) getModTarget(pod *corev1.Pod) (string, error) {
	compilerTarget, err := i.getCompilerTarget(pod)
	if err != nil {
		return "", err
	}
	if compilerTarget == "" {
		return defaultGoModTarget, nil
	}
	path, _, _ := module.SplitPathVersion(compilerTarget)
	if path != defaultGoModTarget {
		return defaultGoModTarget, nil
	}
	return compilerTarget, nil
}

func (i *Injector) getModReplace(pod *corev1.Pod) (string, error) {
	compilerTarget, err := i.getCompilerTarget(pod)
	if err != nil {
		return "", err
	}
	if compilerTarget == "" {
		return "", nil
	}
	path, _, _ := module.SplitPathVersion(compilerTarget)
	if path == defaultGoModTarget {
		return "", nil
	}
	return compilerTarget, nil
}

func (i *Injector) getRegistryName(pod *corev1.Pod) (string, error) {
	registry := pod.Annotations[RegistryInjectAnnotation]
	return registry, nil
}
