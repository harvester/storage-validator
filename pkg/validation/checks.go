package validation

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"
)

// Current validation requirements are
// * create a volume
// * create a snapshot
// * perform offline volume expansion
// * create a vmimage using the storage class specified
// * boot a vm using storage class
// * hotplug 2 volumes to a vm
// * create vm snapshots
// * perform live migration across nodes

const (
	baselinePVCLabelKey = "storage-validator-baseline-pvc"
)

type checkups func(ctx context.Context) error

func (v *ValidationRun) runChecks() error {
	ctx, cancel := context.WithTimeout(v.ctx, time.Duration(*v.Configuration.Timeout)*time.Second)
	defer cancel()
	cleanupComplete := make(chan bool)
	go v.cleanupResources(ctx, cleanupComplete)

	var definedCheckups = []checkups{v.createVolume, v.createSnapshot, v.volumeOfflineResize, v.createVMImage,
		v.createVirtualMachine, v.runVMMigration, v.hotPlugVolume}

	var err error
	// on error break execution and ensure cleanup is triggered
	for _, check := range definedCheckups {
		err := check(ctx)
		if err != nil {
			logrus.Errorf("validation failure: %v", err)
			break
		}
	}

	cancel()
	<-cleanupComplete
	return err
}
