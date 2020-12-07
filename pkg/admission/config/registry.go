package config

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/onosproject/onos-operator/pkg/apis/config/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
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

	// If the pod is annotated with the InjectRegistryAnnotation, inject the module registry
	modelInject, ok := pod.Annotations[InjectRegistryAnnotation]
	if !ok || modelInject != "true" {
		return admission.Allowed(fmt.Sprintf("'%s' annotation not found", InjectRegistryAnnotation))
	}

	// If the pod is annotated with InjectModelAnnotation, ensure RegistryLanguageAnnotation
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
		Name:  fmt.Sprintf("model-registry"),
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

	// Load existing models via init containers
	models := &v1beta1.ModelList{}
	modelOpts := &client.ListOptions{
		Namespace: pod.Namespace,
	}
	if err := i.client.List(context.Background(), models, modelOpts); err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	// For each model, add an init container
	for _, model := range models.Items {
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

		// Add the compiler init container
		container := corev1.Container{
			Name:  fmt.Sprintf("%s-compiler", modelInject),
			Image: fmt.Sprintf("onosproject/config-model-compiler:%s-%s", compilerLanguage, compilerVersion),
			Args: []string{
				"--name",
				model.Spec.Type,
				"--version",
				model.Spec.Version,
				"--build-path",
				buildPath,
				"--output-path",
				registryPath,
			},
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

		// Add module arguments
		for _, module := range model.Spec.Modules {
			container.Args = append(container.Args, "--module", fmt.Sprintf("%s@%s=%s/%s@%s.yang", module.Name, module.Version, modelPath, module.Name, module.Version))
		}

		// If the model is present, inject the init container into the pod
		pod.Spec.InitContainers = append(pod.Spec.InitContainers, container)
	}

	// Marshal the pod and return a patch response
	marshaledPod, err := json.Marshal(pod)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}
	return admission.PatchResponseFromRaw(request.Object.Raw, marshaledPod)
}

var _ admission.Handler = &RegistryInjector{}
var _ admission.DecoderInjector = &RegistryInjector{}
