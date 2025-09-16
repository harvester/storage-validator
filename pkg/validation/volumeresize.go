package validation

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/harvester/storage-validator/pkg/api"
)

func (v *ValidationRun) volumeOfflineResize(ctx context.Context) error {
	checkName := "ensure offline volume expansion is successful"
	initiateCheck(checkName)
	result := &api.Result{
		Name: checkName,
	}

	defer func() {
		v.AddResult(*result)
	}()

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
		returnError := fmt.Errorf("error creating pvc: %w", err)
		result.AddFailureInfo(returnError)
		return returnError
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
		returnError := fmt.Errorf("error creating pvc: %w", err)
		result.AddFailureInfo(returnError)
		return returnError
	}

	if err := v.waitUntilObjectIsReady(ctx, pod, verifyPodIsReady); err != nil {
		result.AddFailureInfo(err)
		return err
	}

	if err := v.waitUntilObjectIsReady(ctx, pvc, verifyPVCIsBound); err != nil {
		result.AddFailureInfo(err)
		return err
	}

	// delete pod as we need to trigger offline expansion
	if err := v.clients.runtimeClient.Delete(ctx, pod); err != nil {
		returnError := fmt.Errorf("error deleting pod: %w", err)
		result.AddFailureInfo(returnError)
		return returnError
	}

	// resize PVC
	pvcObj := pvc.DeepCopy()
	pvc.Spec.Resources.Requests[corev1.ResourceStorage] = resource.MustParse(DefaultPVCResizeRequest)
	if err := v.clients.runtimeClient.Patch(ctx, pvc, client.MergeFrom(pvcObj)); err != nil {
		returnError := fmt.Errorf("error patching pvc size: %w", err)
		result.AddFailureInfo(returnError)
		return returnError
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
		result.AddFailureInfo(err)
		return err
	}

	result.Status = api.CheckStatusSuccess
	completedCheck(checkName)
	return nil
}
