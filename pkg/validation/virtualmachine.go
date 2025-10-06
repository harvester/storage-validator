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
	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (v *ValidationRun) createVirtualMachine(ctx context.Context) error {
	pvc := &corev1.PersistentVolumeClaim{}
	var err error
	// when using longhornV1 Engine a storage class is created with same name as image
	// and we can use that directly
	if v.IsLonghornV1Engine() {
		pvc, err = v.createV1PVC(ctx)
		if err != nil {
			return err
		}
	} else {
		// CDI backed image, so we need to find golden pvc and use that
		pvc, err = v.createDataVolume(ctx)
		if err != nil {
			return err
		}
	}

	v.createdObjects = append(v.createdObjects, pvc)
	// create a VM referencing the pvc returned from above
	vmObj := &kubevirtv1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "vm-storage-validation-",
			Namespace:    v.Configuration.Namespace,
		},
		Spec: kubevirtv1.VirtualMachineSpec{
			RunStrategy: ptr.To(kubevirtv1.RunStrategyRerunOnFailure),
			Template: &kubevirtv1.VirtualMachineInstanceTemplateSpec{
				Spec: kubevirtv1.VirtualMachineInstanceSpec{
					Domain: kubevirtv1.DomainSpec{
						CPU: &kubevirtv1.CPU{
							Sockets: 1,
							Threads: 1,
							Cores:   v.Configuration.VMConfig.CPU,
						},
						Memory: &kubevirtv1.Memory{
							Guest: ptr.To(resource.MustParse(v.Configuration.VMConfig.Memory)),
						},
						Devices: kubevirtv1.Devices{
							Disks: []kubevirtv1.Disk{
								{
									Name: "boot",
									DiskDevice: kubevirtv1.DiskDevice{
										Disk: &kubevirtv1.DiskTarget{
											Bus: kubevirtv1.DiskBusVirtio,
										},
									},
									BootOrder: ptr.To(uint(1)),
								},
							},
							Interfaces: []kubevirtv1.Interface{
								{
									Name:  "default",
									Model: "virtio",
									InterfaceBindingMethod: kubevirtv1.InterfaceBindingMethod{
										Masquerade: &kubevirtv1.InterfaceMasquerade{},
									},
								},
							},
						},
					},
					Volumes: []kubevirtv1.Volume{
						{
							Name: "boot",
							VolumeSource: kubevirtv1.VolumeSource{
								PersistentVolumeClaim: &kubevirtv1.PersistentVolumeClaimVolumeSource{
									PersistentVolumeClaimVolumeSource: corev1.PersistentVolumeClaimVolumeSource{
										ClaimName: pvc.Name,
									},
								},
							},
						},
					},
					Networks: []kubevirtv1.Network{
						{
							Name: "default",
							NetworkSource: kubevirtv1.NetworkSource{
								Pod: &kubevirtv1.PodNetwork{},
							},
						},
					},
				},
			},
		},
	}

	// create VM object
	err = v.clients.runtimeClient.Create(ctx, vmObj)
	if err != nil {
		return fmt.Errorf("error creating vm: %w", err)
	}

	v.createdObjects = append(v.createdObjects, vmObj)
	v.vmName = vmObj.Name // store VM Name as it will be used later for hot plug of volumes and snapshots

	// verify VM is running
	checkVMStatus := func(obj client.Object) (bool, error) {
		vmObj, ok := obj.(*kubevirtv1.VirtualMachine)
		if !ok {
			return false, fmt.Errorf("error asserting object %v to vm", client.ObjectKeyFromObject(obj))
		}
		if vmObj.Status.PrintableStatus == kubevirtv1.VirtualMachineStatusRunning {
			return true, nil
		}

		return false, nil
	}

	// wait until VM is running
	if err := v.waitUntilObjectIsReady(ctx, vmObj, checkVMStatus); err != nil {
		return err
	}

	return nil
}

func (v *ValidationRun) createV1PVC(ctx context.Context) (*corev1.PersistentVolumeClaim, error) {
	// define pvc for usage
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "vm-storage-validation-",
			Namespace:    v.Configuration.Namespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany},
			Resources: corev1.VolumeResourceRequirements{
				Requests: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceStorage: resource.MustParse(v.Configuration.VMConfig.DiskSize),
				},
			},
			StorageClassName: ptr.To(fmt.Sprintf("longhorn-%s", v.vmImageName)),
			VolumeMode:       ptr.To(corev1.PersistentVolumeBlock),
		},
	}

	err := v.clients.runtimeClient.Create(ctx, pvc)
	if err != nil {
		return nil, fmt.Errorf("error creating pvc for virtualmachine: %w", err)
	}
	return pvc, nil
}

// for non longhorn v1 engines we will create a datavolume from image volume
// the pvc associated with datavolume is subsequently used to boot the vm
func (v *ValidationRun) createDataVolume(ctx context.Context) (*corev1.PersistentVolumeClaim, error) {
	dvObj := &cdiv1.DataVolume{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "vm-storage-validation-",
			Namespace:    v.Configuration.Namespace,
		},
		Spec: cdiv1.DataVolumeSpec{
			Source: &cdiv1.DataVolumeSource{
				PVC: &cdiv1.DataVolumeSourcePVC{
					// when a vmimage is created a golden pvc with same name is created and we use the same
					Name:      v.vmImageName,
					Namespace: v.Configuration.Namespace,
				},
			},
			Storage: &cdiv1.StorageSpec{
				StorageClassName: ptr.To(v.Configuration.StorageClass),
			},
		},
	}

	// wait for datavolume to be marked ready
	err := v.clients.runtimeClient.Create(ctx, dvObj)
	if err != nil {
		return nil, fmt.Errorf("error creating datavolume for vm: %w", err)
	}

	v.createdObjects = append(v.createdObjects, dvObj)

	// check if datavolume is ready
	isDataVolumeReady := func(obj client.Object) (bool, error) {
		dvObj, ok := obj.(*cdiv1.DataVolume)
		if !ok {
			return false, fmt.Errorf("error asserting object %v to datavolume", client.ObjectKeyFromObject(obj))
		}

		for _, condition := range dvObj.Status.Conditions {
			if condition.Type == cdiv1.DataVolumeReady && condition.Status == corev1.ConditionTrue {
				return true, nil
			}
		}
		return false, nil
	}

	// wait until datavolume is ready
	if err := v.waitUntilObjectIsReady(ctx, dvObj, isDataVolumeReady); err != nil {
		return nil, err
	}

	pvcObj := &corev1.PersistentVolumeClaim{}
	err = v.clients.runtimeClient.Get(ctx, types.NamespacedName{Name: dvObj.Name, Namespace: dvObj.Namespace}, pvcObj)
	return pvcObj, err
}
