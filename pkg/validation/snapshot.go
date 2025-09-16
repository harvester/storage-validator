package validation

import (
	"context"
	"fmt"

	snapshot "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/harvester/storage-validator/pkg/api"
)

func (v *ValidationRun) createSnapshot(ctx context.Context) error {
	checkName := "ensure volume snapshot can be created successfully"
	initiateCheck(checkName)
	result := &api.Result{
		Name: checkName,
	}

	defer func() {
		v.AddResult(*result)
	}()

	volumeSnapshot := &snapshot.VolumeSnapshot{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "snapshot-storage-validation-",
			Namespace:    v.Configuration.Namespace,
		},
		Spec: snapshot.VolumeSnapshotSpec{
			Source: snapshot.VolumeSnapshotSource{
				PersistentVolumeClaimName: ptr.To(v.pvcName),
			},
			VolumeSnapshotClassName: ptr.To(v.Configuration.SnapshotClass),
		},
	}

	err := v.clients.runtimeClient.Create(ctx, volumeSnapshot)
	if err != nil {
		returnError := fmt.Errorf("error creating volumesnapshot: %w", err)
		result.AddFailureInfo(returnError)
		return returnError
	}

	v.createdObjects = append(v.createdObjects, volumeSnapshot)

	verifySnapshotIsReady := func(obj client.Object) (bool, error) {
		snapshotObj, ok := obj.(*snapshot.VolumeSnapshot)
		if !ok {
			return false, fmt.Errorf("error asserting object %v to volumesnapshot", client.ObjectKeyFromObject(obj))
		}

		if snapshotObj.Status.ReadyToUse != nil && *snapshotObj.Status.ReadyToUse == true {
			return true, nil
		}
		return false, nil
	}

	if err := v.waitUntilObjectIsReady(ctx, volumeSnapshot, verifySnapshotIsReady); err != nil {
		result.AddFailureInfo(err)
		return err
	}
	result.Status = api.CheckStatusSuccess
	completedCheck(checkName)
	return nil
}
