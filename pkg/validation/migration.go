package validation

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubevirtv1 "kubevirt.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (v *ValidationRun) runVMMigration(ctx context.Context) error {

	vmMigrationObject := &kubevirtv1.VirtualMachineInstanceMigration{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "migration-storage-validator-",
			Namespace:    v.Configuration.Namespace,
		},
		Spec: kubevirtv1.VirtualMachineInstanceMigrationSpec{
			VMIName: v.vmName,
		},
	}

	if err := v.clients.runtimeClient.Create(ctx, vmMigrationObject); err != nil {
		return fmt.Errorf("error creating vm migration: %w", err)
	}

	v.createdObjects = append(v.createdObjects, vmMigrationObject)

	// reconcile if vm migration is successful
	checkMigrationStatus := func(obj client.Object) (bool, error) {
		vmimObj, ok := obj.(*kubevirtv1.VirtualMachineInstanceMigration)
		if !ok {
			return false, fmt.Errorf("error asserting object %v to vm migration", client.ObjectKeyFromObject(obj))
		}

		if vmimObj.Status.Phase == kubevirtv1.MigrationSucceeded {
			return true, nil
		}
		return false, nil

	}

	// wait until migration is completed
	if err := v.waitUntilObjectIsReady(ctx, vmMigrationObject, checkMigrationStatus); err != nil {
		return err
	}

	return nil
}
