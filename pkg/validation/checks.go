package validation

import (
	"context"
	"time"

	"github.com/harvester/storage-validator/pkg/api"
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

type Validation struct {
	Name              string
	ExecuteValidation validationFunc
}

type validationFunc func(ctx context.Context) error

func (v *ValidationRun) runChecks() error {
	ctx, cancel := context.WithTimeout(v.ctx, time.Duration(*v.Configuration.Timeout)*time.Second)
	defer cancel()
	cleanupComplete := make(chan bool)
	go v.cleanupResources(ctx, cleanupComplete)

	validations := []Validation{
		{
			Name:              "ensure volume is created and used successfully",
			ExecuteValidation: v.createVolume,
		},
		{
			Name:              "ensure volume snapshot can be created successfully",
			ExecuteValidation: v.createSnapshot,
		},
		{
			Name:              "ensure offline volume expansion is successful",
			ExecuteValidation: v.volumeOfflineResize,
		},
		{
			Name:              "ensure vm image creation is successful",
			ExecuteValidation: v.createVMImage,
		},
		{
			Name:              "ensure vm can boot from recently created vmimage",
			ExecuteValidation: v.createVirtualMachine,
		},
		{
			Name:              "trigger VM migration",
			ExecuteValidation: v.runVMMigration,
		},
		{
			Name:              "hotplug 2 volumes to existing VM",
			ExecuteValidation: v.hotPlugVolume,
		},
	}

	var err error
	// on error break execution and ensure cleanup is triggered
	for _, check := range validations {
		initiateCheck(check.Name)
		result := &api.Result{
			Name: check.Name,
		}
		defer func() {
			v.AddResult(*result)
		}()
		err := check.ExecuteValidation(ctx)
		if err != nil {
			result.AddFailureInfo(err)
			logrus.Errorf("validation failure: %v", err)
			break
		}
		result.Status = api.CheckStatusSuccess
		completedCheck(check.Name)
	}

	cancel()
	<-cleanupComplete
	return err
}
