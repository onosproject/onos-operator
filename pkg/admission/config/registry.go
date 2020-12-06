package config

import (
	"context"
	"encoding/json"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const (
	// RegistryVersionAnnotation is an annotation indicating the path at which to mount the registry
	RegistryPathAnnotation = "config.onosproject.org/registry-path"
)

const (
	// InjectRegistry is an annotation value indicating that the registry should be injected
	InjectRegistry = "registry"
)

const (
	registryVolume = "registry"
)

const (
	defaultRegistryPath = "/root/registry"
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
	// Decode the pod
	pod := &corev1.Pod{}
	if err := i.decoder.Decode(request, pod); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	// If the pod is annotated with the RegistryInjectAnnotation, inject the module registry
	modelInject, ok := pod.Annotations[InjectAnnotation]
	if !ok || modelInject != InjectRegistry {
		return admission.Allowed(fmt.Sprintf("'%s' annotation not found", InjectAnnotation))
	}

	// If the pod is annotated with InjectAnnotation, ensure RegistryLanguageAnnotation
	// and RegistryVersionAnnotation are present as well
	compilerLanguage, ok := pod.Annotations[CompilerLanguageAnnotation]
	if !ok {
		return admission.Denied(fmt.Sprintf("'%s' annotation not found", CompilerLanguageAnnotation))
	}
	compilerVersion, ok := pod.Annotations[CompilerVersionAnnotation]
	if !ok {
		return admission.Denied(fmt.Sprintf("'%s' annotation not found", CompilerVersionAnnotation))
	}
	registryPath, ok := pod.Annotations[RegistryPathAnnotation]
	if !ok {
		registryPath = defaultRegistryPath
	}

	// Add a registry volume to the pod
	pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{
		Name: registryVolume,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	})

	// Mount the registry volume to existing containers
	for i, container := range pod.Spec.Containers {
		container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
			Name:      registryVolume,
			MountPath: registryPath,
		})
		pod.Spec.Containers[i] = container
	}

	// Add the registry init container
	container := corev1.Container{
		Name:  "model-registry",
		Image: fmt.Sprintf("onosproject/config-model-registry:%s-%s", compilerLanguage, compilerVersion),
		Args: []string{
			"--build-path",
			buildPath,
			"--registry-path",
			registryPath,
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      registryVolume,
				MountPath: registryPath,
			},
		},
	}

	// If the model is present, inject the init container into the pod
	pod.Spec.Containers = append(pod.Spec.Containers, container)

	// Mount the registry volume to all containers

	// Marshal the pod and return a patch response
	marshaledPod, err := json.Marshal(pod)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}
	return admission.PatchResponseFromRaw(request.Object.Raw, marshaledPod)
}

var _ admission.Handler = &RegistryInjector{}
var _ admission.DecoderInjector = &RegistryInjector{}
