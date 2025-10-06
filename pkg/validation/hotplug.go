package validation

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	kubevirtv1 "kubevirt.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (v *ValidationRun) hotPlugVolume(ctx context.Context) error {

	pvcList := []*corev1.PersistentVolumeClaim{
		{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "hotplug-storage-validation-",
				Namespace:    v.Configuration.Namespace,
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany},
				StorageClassName: ptr.To(v.Configuration.StorageClass),
				Resources: corev1.VolumeResourceRequirements{
					Requests: map[corev1.ResourceName]resource.Quantity{
						corev1.ResourceStorage: resource.MustParse(DefaultPVCSize),
					},
				},
				VolumeMode: ptr.To(corev1.PersistentVolumeBlock),
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "hotplug-storage-validation-",
				Namespace:    v.Configuration.Namespace,
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany},
				StorageClassName: ptr.To(v.Configuration.StorageClass),
				Resources: corev1.VolumeResourceRequirements{
					Requests: map[corev1.ResourceName]resource.Quantity{
						corev1.ResourceStorage: resource.MustParse(DefaultPVCSize),
					},
				},
				VolumeMode: ptr.To(corev1.PersistentVolumeBlock),
			},
		},
	}

	for _, pvc := range pvcList {
		if err := v.clients.runtimeClient.Create(ctx, pvc); err != nil {
			return fmt.Errorf("error creating pvc: %w", err)
		}
		v.createdObjects = append(v.createdObjects, pvc)
	}

	// hotplug pvc to vm
	vmObj := &kubevirtv1.VirtualMachine{}
	if err := v.clients.runtimeClient.Get(ctx, types.NamespacedName{Name: v.vmName, Namespace: v.Configuration.Namespace}, vmObj); err != nil {
		return fmt.Errorf("error looking up VM during hotplug attachment: %w", err)
	}

	for i, pvc := range pvcList {
		name := fmt.Sprintf("hotplug-%d", i)
		volume := &kubevirtv1.AddVolumeOptions{
			Name: name,
			Disk: &kubevirtv1.Disk{
				DiskDevice: kubevirtv1.DiskDevice{
					Disk: &kubevirtv1.DiskTarget{
						Bus: "scsi",
					},
				},
			},
			VolumeSource: &kubevirtv1.HotplugVolumeSource{
				PersistentVolumeClaim: &kubevirtv1.PersistentVolumeClaimVolumeSource{
					PersistentVolumeClaimVolumeSource: corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: pvc.Name,
					},
				},
			},
		}

		// add volume
		if err := v.clients.kubevirtClient.VirtualMachine(vmObj.Namespace).AddVolume(ctx, vmObj.Name, volume); err != nil {
			return fmt.Errorf("error attempting to hot plug disks: %w", err)
		}
	}

	// wait until VMI reflects hot plug volumeStatus
	// need to create a VirtualMachineInstance object as volumestatus is part of VirtualMachineInstance object
	vmiObj := &kubevirtv1.VirtualMachineInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      vmObj.Name,
			Namespace: vmObj.Namespace,
		},
	}

	checkHotPlugStatus := func(obj client.Object) (bool, error) {
		vmiObj, ok := obj.(*kubevirtv1.VirtualMachineInstance)
		if !ok {
			return false, fmt.Errorf("error asserting object %v to vmi", client.ObjectKeyFromObject(obj))
		}

		attachedCount := 0
		for _, pvc := range pvcList {
			for _, volumeStatus := range vmiObj.Status.VolumeStatus {
				if volumeStatus.PersistentVolumeClaimInfo.ClaimName == pvc.Name && volumeStatus.Phase == kubevirtv1.HotplugVolumeAttachedToNode {
					attachedCount++
				}
			}
		}

		if len(pvcList) == attachedCount {
			return true, nil
		}
		return false, nil
	}

	// wait until VM is running
	if err := v.waitUntilObjectIsReady(ctx, vmiObj, checkHotPlugStatus); err != nil {
		return err
	}

	// harvester webhooks block pvc deletion if its hot plugged to a volume
	// to avoid this we remove the hot plugged disks
	for i := range pvcList {
		name := fmt.Sprintf("hotplug-%d", i)
		volume := &kubevirtv1.RemoveVolumeOptions{
			Name: name,
		}

		// remove volume
		if err := v.clients.kubevirtClient.VirtualMachine(vmObj.Namespace).RemoveVolume(ctx, vmObj.Name, volume); err != nil {
			return fmt.Errorf("error attempting to remove hot plug disks: %w", err)
		}
	}

	return nil
}
