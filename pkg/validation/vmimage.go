package validation

import (
	"context"
	"fmt"

	harvesterv1beta1 "github.com/harvester/harvester/pkg/apis/harvesterhci.io/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (v *ValidationRun) createVMImage(ctx context.Context) error {
	vmImage := &harvesterv1beta1.VirtualMachineImage{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "vmimage-storage-validation-",
			Namespace:    v.Configuration.Namespace,
		},
		Spec: harvesterv1beta1.VirtualMachineImageSpec{
			DisplayName:            "storage-validation-test-image",
			TargetStorageClassName: v.Configuration.StorageClass,
			URL:                    v.Configuration.ImageURL,
			SourceType:             harvesterv1beta1.VirtualMachineImageSourceTypeDownload,
			Retry:                  3,
		},
	}

	// ensure we are using longhorn v1 engine and then use BackingImage backend
	if v.IsLonghornV1Engine() {
		vmImage.Spec.Backend = harvesterv1beta1.VMIBackendBackingImage
	} else {
		vmImage.Spec.Backend = harvesterv1beta1.VMIBackendCDI
	}

	// submit vmimage request
	if err := v.clients.runtimeClient.Create(ctx, vmImage); err != nil {
		return fmt.Errorf("error creating vmimage: %w", err)
	}

	v.createdObjects = append(v.createdObjects, vmImage)
	v.vmImageName = vmImage.Name //store vmimage details as it will be used later to create vm

	// verify VMImage is ready
	checkVMImage := func(obj client.Object) (bool, error) {
		vmImageObj, ok := obj.(*harvesterv1beta1.VirtualMachineImage)
		if !ok {
			return false, fmt.Errorf("error asserting object %v to vmimage", client.ObjectKeyFromObject(obj))
		}

		for _, condition := range vmImageObj.Status.Conditions {
			if condition.Type == harvesterv1beta1.ImageImported && condition.Status == corev1.ConditionTrue {
				return true, nil
			}
		}
		return false, nil
	}

	// wait until VMImage is ready
	if err := v.waitUntilObjectIsReady(ctx, vmImage, checkVMImage); err != nil {
		return err
	}

	return nil
}
