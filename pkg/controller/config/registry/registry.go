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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// Handler is a mutating webhook for injecting the registry container into pods
type Handler struct {
	client  client.Client
	decoder *admission.Decoder
}

// InjectDecoder :
func (h *Handler) InjectDecoder(decoder *admission.Decoder) error {
	h.decoder = decoder
	return nil
}

// Handle :
func (h *Handler) Handle(ctx context.Context, request admission.Request) admission.Response {
	log.Infof("Received admission request for Pod '%s/%s'", request.Name, request.Namespace)

	// Decode the pod
	pod := &corev1.Pod{}
	if err := h.decoder.Decode(request, pod); err != nil {
		log.Errorf("Failed to inject registry into Pod '%s/%s': %s", pod.Name, pod.Namespace, err)
		return admission.Errored(http.StatusBadRequest, err)
	}

	injector := newInjector(h.client, request.Namespace)
	ok, err := injector.inject(ctx, pod)
	if !ok {
		return admission.Allowed("Skipped injection")
	} else if err != nil {
		if errors.IsNotFound(err) {
			return admission.Denied(err.Error())
		}
		return admission.Errored(http.StatusInternalServerError, err)
	}

	// Marshal the pod and return a patch response
	marshaledPod, err := json.Marshal(pod)
	if err != nil {
		log.Errorf("Failed to inject registry into Pod '%s/%s': %s", pod.Name, pod.Namespace, err)
		return admission.Errored(http.StatusInternalServerError, err)
	}

	log.Infof("Completed injecting registry into Pod '%s/%s'", pod.Name, pod.Namespace)
	return admission.PatchResponseFromRaw(request.Object.Raw, marshaledPod)
}

var _ admission.Handler = &Handler{}
var _ admission.DecoderInjector = &Handler{}
