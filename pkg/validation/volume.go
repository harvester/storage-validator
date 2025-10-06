package validation

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (v *ValidationRun) createVolume(ctx context.Context) error {

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "pvc-storage-validation-",
			Namespace:    v.Configuration.Namespace,
			Labels: map[string]string{
				baselinePVCLabelKey: "true",
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany},
			StorageClassName: ptr.To(v.Configuration.StorageClass),
			Resources: corev1.VolumeResourceRequirements{
				Requests: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceStorage: resource.MustParse(DefaultPVCSize),
				},
			},
		},
	}

	// need to create pvc
	err := v.clients.runtimeClient.Create(ctx, pvc)
	if err != nil {
		return fmt.Errorf("error creating pvc: %w", err)
	}

	// store for cleanup later on
	v.createdObjects = append(v.createdObjects, pvc)
	v.pvcName = pvc.Name
	// attach pvc to pod to ensure creation
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "pvc-storage-validation-",
			Namespace:    v.Configuration.Namespace,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "nginx",
					Image: "registry.suse.com/suse/nginx:1.21",
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "pvc-storage-validation",
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: pvc.Name,
						},
					},
				},
			},
		},
	}

	err = v.clients.runtimeClient.Create(ctx, pod)
	if err != nil {
		return fmt.Errorf("error creating pvc: %w", err)
	}
	v.createdObjects = append(v.createdObjects, pod)

	if err := v.waitUntilObjectIsReady(ctx, pod, verifyPodIsReady); err != nil {
		return err
	}

	if err := v.waitUntilObjectIsReady(ctx, pvc, verifyPVCIsBound); err != nil {
		return err
	}
	return nil
}

// reconcile until pod is running and ensure pvc is bound
func verifyPodIsReady(obj client.Object) (bool, error) {
	podObj, ok := obj.(*corev1.Pod)
	if !ok {
		return false, fmt.Errorf("error asserting object %v to pod", client.ObjectKeyFromObject(obj))
	}
	if podObj.Status.Phase != corev1.PodRunning {
		return true, nil
	}
	return false, nil
}

// verify PVC is Bound
func verifyPVCIsBound(obj client.Object) (bool, error) {
	pvcObj, ok := obj.(*corev1.PersistentVolumeClaim)
	if !ok {
		return false, fmt.Errorf("error asserting object %v to pvc", client.ObjectKeyFromObject(obj))
	}
	if pvcObj.Status.Phase == corev1.ClaimBound {
		return true, nil
	}
	return false, nil
}
