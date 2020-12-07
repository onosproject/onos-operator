package k8s

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func HasFinalizer(object metav1.Object, finalizer string) bool {
	for _, finalizer := range object.GetFinalizers() {
		if finalizer == finalizer {
			return true
		}
	}
	return false
}

func AddFinalizer(object metav1.Object, finalizer string) {
	object.SetFinalizers(append(object.GetFinalizers(), finalizer))
}

func RemoveFinalizer(object metav1.Object, finalizer string) {
	finalizers := make([]string, 0, len(object.GetFinalizers())-1)
	for _, f := range object.GetFinalizers() {
		if f != finalizer {
			finalizers = append(finalizers, finalizer)
		}
	}
	object.SetFinalizers(finalizers)
}
