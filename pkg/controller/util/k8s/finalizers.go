// SPDX-FileCopyrightText: 2020-present Open Networking Foundation <info@opennetworking.org>
//
// SPDX-License-Identifier: Apache-2.0

package k8s

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// HasFinalizer :
func HasFinalizer(object metav1.Object, finalizer string) bool {
	for _, f := range object.GetFinalizers() {
		if f == finalizer {
			return true
		}
	}
	return false
}

// AddFinalizer :
func AddFinalizer(object metav1.Object, finalizer string) {
	object.SetFinalizers(append(object.GetFinalizers(), finalizer))
}

// RemoveFinalizer :
func RemoveFinalizer(object metav1.Object, finalizer string) {
	finalizers := make([]string, 0, len(object.GetFinalizers())-1)
	for _, f := range object.GetFinalizers() {
		if f != finalizer {
			finalizers = append(finalizers, finalizer)
		}
	}
	object.SetFinalizers(finalizers)
}
