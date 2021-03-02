package registry

import (
	"context"
	"fmt"
	configv1beta1 "github.com/onosproject/onos-operator/pkg/apis/config/v1beta1"
	"github.com/rogpeppe/go-internal/module"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
)

const (
	// RegistryInjectAnnotation is an annotation indicating the model to inject the registry into a pod
	RegistryInjectAnnotation = "registry.config.onosproject.org/inject"
	// RegistryInjectStatusAnnotation is an annotation indicating the status of registry injection
	RegistryInjectStatusAnnotation = "registry.config.onosproject.org/inject-status"
	// RegistryInjectStatusInjeceted is an annotation value indicating the registry has been injected
	RegistryInjectStatusInjected = "injected"
	// RegistryPathAnnotation is an annotation indicating the path at which to mount the registry
	RegistryPathAnnotation = "registry.config.onosproject.org/path"
	// CachePathAnnotation is an annotation indicating the path at which to mount the cache
	CachePathAnnotation = "cache.config.onosproject.org/path"
	// CompilerVersionAnnotation is an annotation indicating the model API version
	CompilerVersionAnnotation = "compiler.config.onosproject.org/version"
	// CompilerTargetAnnotation is an annotation indicating the Go module for which to compile a model
	CompilerTargetAnnotation = "compiler.config.onosproject.org/target"
)

const (
	modelPath           = "/etc/onos/models"
	buildPath           = "/build"
	registryVolumeName  = "model-registry"
	cacheVolumeName     = "plugin-cache"
	defaultGoModTarget  = "github.com/onosproject/onos-config"
	defaultRegistryPath = "/etc/onos/plugins"
	defaultCachePath    = "/etc/onos/cache"
)

func newInjector(client client.Client, namespace string) *RegistryInjector {
	return &RegistryInjector{
		client:    client,
		namespace: namespace,
	}
}

// RegistryInjector is a mutating webhook for injecting the registry container into pods
type RegistryInjector struct {
	client    client.Client
	namespace string
}

func (i *RegistryInjector) inject(ctx context.Context, pod *corev1.Pod) (bool, error) {
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
	if err := i.injectCompilers(ctx, pod); err != nil {
		return true, err
	}

	// Set the registry injection status to injected
	pod.Annotations[RegistryInjectStatusAnnotation] = RegistryInjectStatusInjected
	return true, nil
}

func (i *RegistryInjector) injectRegistry(ctx context.Context, pod *corev1.Pod) error {
	registryName, err := i.getRegistryName(pod)
	if err != nil {
		return err
	}
	registryPath, err := i.getRegistryPath(pod)
	if err != nil {
		return err
	}
	cachePath, err := i.getCachePath(pod)
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
			Name:         cacheVolumeName,
			VolumeSource: registry.Spec.Cache.Volume.VolumeSource,
		}
	} else {
		cacheVolume = corev1.Volume{
			Name: cacheVolumeName,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		}
	}
	pod.Spec.Volumes = append(pod.Spec.Volumes, cacheVolume)

	// Mount the registry volume to existing containers
	for j, container := range pod.Spec.Containers {
		container.Env = append(container.Env, corev1.EnvVar{
			Name:  "CONFIG_MODEL_REGISTRY",
			Value: registryPath,
		})
		container.Env = append(container.Env, corev1.EnvVar{
			Name:  "CONFIG_MODULE_TARGET",
			Value: modTarget,
		})
		container.Env = append(container.Env, corev1.EnvVar{
			Name:  "CONFIG_MODULE_REPLACE",
			Value: modReplace,
		})
		container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
			Name:      registryVolumeName,
			MountPath: registryPath,
		})
		pod.Spec.Containers[j] = container
	}

	args := []string{
		"--build-path",
		buildPath,
		"--registry-path",
		registryPath,
		//"--cache-path",
		//cachePath,
	}

	if modTarget != "" {
		args = append(args, "--target", modTarget)
	}

	if modReplace != "" {
		args = append(args, "--replace", modReplace)
	}

	var tags []string
	if compilerVersion != "" {
		tags = append(tags, compilerVersion)
	}
	image := fmt.Sprintf("onosproject/config-model-registry:%s", strings.Join(tags, "-"))

	// Add the registry sidecar container
	container := corev1.Container{
		Name:  "model-registry",
		Image: image,
		Args:  args,
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      registryVolumeName,
				MountPath: registryPath,
			},
			{
				Name:      cacheVolumeName,
				MountPath: cachePath,
			},
		},
	}

	// If the model is present, inject the init container into the pod
	pod.Spec.Containers = append(pod.Spec.Containers, container)
	return nil
}

func (i *RegistryInjector) injectCompilers(ctx context.Context, pod *corev1.Pod) error {
	// Load existing models via init containers
	modelList := &configv1beta1.ModelList{}
	modelListOpts := &client.ListOptions{
		Namespace: i.namespace,
	}
	if err := i.client.List(ctx, modelList, modelListOpts); err != nil {
		return err
	}

	for _, model := range modelList.Items {
		if err := i.injectCompiler(ctx, pod, model); err != nil {
			return err
		}
	}
	return nil
}

func (i *RegistryInjector) injectCompiler(ctx context.Context, pod *corev1.Pod, model configv1beta1.Model) error {
	registryPath, err := i.getRegistryPath(pod)
	if err != nil {
		return err
	}
	cachePath, err := i.getCachePath(pod)
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

	if model.Spec.Plugin == nil {
		return nil
	}

	log.Infof("Injecting model '%s' into Pod '%s/%s'", model.Name, pod.Name, pod.Namespace)

	// Load the model files
	files, err := i.getModelFiles(ctx, model)
	if err != nil {
		return err
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
		//"--cache-path",
		//cachePath,
	}

	if modTarget != "" {
		args = append(args, "--target", modTarget)
	}

	if modReplace != "" {
		args = append(args, "--replace", modReplace)
	}

	// Add module arguments
	for module, file := range files {
		args = append(args, "--module", fmt.Sprintf("%s=%s/%s", module, modelPath, file))
	}

	var tags []string
	if compilerVersion != "" {
		tags = append(tags, compilerVersion)
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
				Name:      registryVolumeName,
				MountPath: registryPath,
			},
			{
				Name:      cacheVolumeName,
				MountPath: cachePath,
			},
		},
	}

	// If the model is present, inject the init container into the pod
	pod.Spec.InitContainers = append(pod.Spec.InitContainers, container)
	return nil
}

func (i *RegistryInjector) getCompilerVersion(pod *corev1.Pod) (string, error) {
	compilerVersion, ok := pod.Annotations[CompilerVersionAnnotation]
	if !ok {
		return "", fmt.Errorf("'%s' annotation not found", CompilerVersionAnnotation)
	}
	return compilerVersion, nil
}

func (i *RegistryInjector) getCompilerTarget(pod *corev1.Pod) (string, error) {
	compilerTarget := pod.Annotations[CompilerTargetAnnotation]
	return compilerTarget, nil
}

func (i *RegistryInjector) getModTarget(pod *corev1.Pod) (string, error) {
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

func (i *RegistryInjector) getModReplace(pod *corev1.Pod) (string, error) {
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

func (i *RegistryInjector) getRegistryPath(pod *corev1.Pod) (string, error) {
	path, ok := pod.Annotations[RegistryPathAnnotation]
	if !ok {
		return defaultRegistryPath, nil
	}
	return path, nil
}

func (i *RegistryInjector) getRegistryName(pod *corev1.Pod) (string, error) {
	registry := pod.Annotations[RegistryInjectAnnotation]
	return registry, nil
}

func (i *RegistryInjector) getCachePath(pod *corev1.Pod) (string, error) {
	path, ok := pod.Annotations[CachePathAnnotation]
	if !ok {
		return defaultCachePath, nil
	}
	return path, nil
}

func (i *RegistryInjector) getModelFiles(ctx context.Context, model configv1beta1.Model) (map[string]string, error) {
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
		if err := i.client.Get(ctx, modelDepName, &modelDep); err != nil {
			return nil, err
		}

		refFiles, err := i.getModelFiles(ctx, modelDep)
		if err != nil {
			return nil, err
		}
		for name, value := range refFiles {
			files[name] = value
		}
	}
	return files, nil
}
