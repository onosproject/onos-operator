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
)

const (
	// InjectAnnotation is an annotation indicating whether to inject into a pod
	InjectAnnotation = "config.onosproject.org/inject"
	// ModelAnnotation is an annotation indicating the model name
	ModelAnnotation = "config.onosproject.org/model"
	// CompilerLanguageAnnotation is an annotation indicating which compiler language to use to compile a model
	CompilerLanguageAnnotation = "config.onosproject.org/compiler-language"
	// CompilerVersionAnnotation is an annotation indicating which compiler version to use to compile a model
	CompilerVersionAnnotation = "config.onosproject.org/compiler-version"
)

const (
	InjectModel = "model"
)

const (
	modelVolume = "model"
)

const (
	modelPath = "/root/models"
	buildPath = "/root/build"
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
	// Decode the pod
	pod := &corev1.Pod{}
	if err := i.decoder.Decode(request, pod); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	// If the pod is annotated with the InjectAnnotation, inject the module compiler
	modelInject, ok := pod.Annotations[InjectAnnotation]
	if !ok || modelInject != InjectModel {
		return admission.Allowed(fmt.Sprintf("'%s' annotation not found", InjectAnnotation))
	}
	modelName, ok := pod.Annotations[ModelAnnotation]
	if !ok {
		return admission.Allowed(fmt.Sprintf("'%s' annotation not found", ModelAnnotation))
	}

	// If the pod is annotated with InjectAnnotation, ensure CompilerLanguageAnnotation
	// and CompilerVersionAnnotation are present as well
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

	// Load the annotated model
	model := &configv1beta1.Model{}
	modelNamespacedName := types.NamespacedName{
		Name:      modelName,
		Namespace: pod.Namespace,
	}
	if err := i.client.Get(ctx, modelNamespacedName, model); err != nil {
		if errors.IsNotFound(err) {
			return admission.Denied(fmt.Sprintf("Model '%s' not found", modelName))
		}
		return admission.Errored(http.StatusInternalServerError, err)
	}

	// Load the associated ConfigMap
	cm := &corev1.ConfigMap{}
	cmNamespacedName := types.NamespacedName{
		Name:      modelName,
		Namespace: pod.Namespace,
	}
	if err := i.client.Get(ctx, cmNamespacedName, cm); err != nil {
		if errors.IsNotFound(err) {
			return admission.Denied(fmt.Sprintf("ConfigMap '%s' not found", cmNamespacedName))
		}
		return admission.Errored(http.StatusInternalServerError, err)
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

	// Add the model volume to the pod
	pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{
		Name: modelVolume,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: cm.Name,
				},
			},
		},
	})

	// Add the compiler init container
	container := corev1.Container{
		Name:  fmt.Sprintf("%s-compiler", modelName),
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
				Name:      modelVolume,
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
	pod.Spec.Containers = append(pod.Spec.Containers, container)

	// Marshal the pod and return a patch response
	marshaledPod, err := json.Marshal(pod)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}
	return admission.PatchResponseFromRaw(request.Object.Raw, marshaledPod)
}

var _ admission.Handler = &RegistryInjector{}
var _ admission.DecoderInjector = &RegistryInjector{}