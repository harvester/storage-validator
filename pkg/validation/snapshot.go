package validation

import (
	"context"
	"fmt"

	snapshot "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (v *ValidationRun) createSnapshot(ctx context.Context) error {
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
		return fmt.Errorf("error creating volumesnapshot: %w", err)
	}

	v.createdObjects = append(v.createdObjects, volumeSnapshot)

	verifySnapshotIsReady := func(obj client.Object) (bool, error) {
		snapshotObj, ok := obj.(*snapshot.VolumeSnapshot)
		if !ok {
			return false, fmt.Errorf("error asserting object %v to volumesnapshot", client.ObjectKeyFromObject(obj))
		}

		if snapshotObj.Status != nil && snapshotObj.Status.ReadyToUse != nil && *snapshotObj.Status.ReadyToUse {
			return true, nil
		}
		return false, nil
	}

	if err := v.waitUntilObjectIsReady(ctx, volumeSnapshot, verifySnapshotIsReady); err != nil {
		return err
	}

	return nil
}
