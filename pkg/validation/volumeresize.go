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

func (v *ValidationRun) volumeOfflineResize(ctx context.Context) error {

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "volume-resize-storage-validation-",
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
	// attach pvc to pod to ensure creation
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "volume-resize-storage-validation-",
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

	if err := v.waitUntilObjectIsReady(ctx, pod, verifyPodIsReady); err != nil {
		return err
	}

	if err := v.waitUntilObjectIsReady(ctx, pvc, verifyPVCIsBound); err != nil {
		return err
	}

	// delete pod as we need to trigger offline expansion
	if err := v.clients.runtimeClient.Delete(ctx, pod); err != nil {
		return fmt.Errorf("error deleting pod: %w", err)
	}

	// resize PVC
	pvcObj := pvc.DeepCopy()
	pvc.Spec.Resources.Requests[corev1.ResourceStorage] = resource.MustParse(DefaultPVCResizeRequest)
	if err := v.clients.runtimeClient.Patch(ctx, pvc, client.MergeFrom(pvcObj)); err != nil {
		return fmt.Errorf("error patching pvc size: %w", err)
	}

	checkPVCResize := func(obj client.Object) (bool, error) {
		pvcObj, ok := obj.(*corev1.PersistentVolumeClaim)
		if !ok {
			return false, fmt.Errorf("error asserting object %v to pvc", client.ObjectKeyFromObject(obj))
		}
		if pvcObj.Status.Capacity[corev1.ResourceStorage] == pvcObj.Spec.Resources.Requests[corev1.ResourceStorage] {
			return true, nil
		}
		return false, nil
	}

	if err := v.waitUntilObjectIsReady(ctx, pvc, checkPVCResize); err != nil {
		return err
	}

	return nil
}
