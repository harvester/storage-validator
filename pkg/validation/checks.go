package validation

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/harvester/storage-validator/pkg/api"
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
	// Skip records the check in the report as "skipped" with SkipReason,
	// instead of executing it (e.g. live migration on a single node).
	Skip       bool
	SkipReason string
}

const migrationCheckName = "trigger VM migration"

type validationFunc func(ctx context.Context) error

func (v *ValidationRun) runChecks() error {
	ctx, cancel := context.WithTimeout(v.ctx, time.Duration(*v.Configuration.Timeout)*time.Second)
	defer cancel()
	cleanupComplete := make(chan bool)
	go func() {
		err := v.cleanupResources(ctx, cleanupComplete)
		if err != nil {
			logrus.Errorf("error during resource cleaup: %v", err)
		}
	}()

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
	}

	// Live migration requires a second node and a volume that can move. On a
	// single-node cluster, or for node-local (topology-pinned) storage such as
	// LVM CSI, it cannot run. Rather than silently dropping it, keep it in the
	// list and mark it skipped so it is still reported (see -single-node /
	// -skip-migration), and raise a prominent top-level warning: live migration
	// is a critical capability and its absence must not be mistaken for a pass.
	migration := Validation{
		Name:              migrationCheckName,
		ExecuteValidation: v.runVMMigration,
	}
	if v.skipMigration() {
		migration.Skip = true
		if v.singleNode() {
			migration.SkipReason = "single-node cluster (singleNode set): live migration requires at least 2 nodes"
			v.Report.Warnings = append(v.Report.Warnings,
				"This validation ran on a SINGLE NODE (singleNode mode), so LIVE MIGRATION WAS NOT TESTED. "+
					"Live migration is a critical storage capability for production clusters. These results do "+
					"not attest to it — re-run on a multi-node cluster with migration-capable (RWX) storage to validate it.")
		} else {
			migration.SkipReason = "skip-migration set"
			v.Report.Warnings = append(v.Report.Warnings,
				"LIVE MIGRATION WAS NOT TESTED (skip-migration set). Live migration is a critical storage "+
					"capability; these results do not attest to it. Re-run without -skip-migration to validate it.")
		}
	}
	validations = append(validations, migration)

	validations = append(validations, Validation{
		Name:              "hotplug 2 volumes to existing VM",
		ExecuteValidation: v.hotPlugVolume,
	})

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
		if check.Skip {
			result.Status = api.CheckStatusSkipped
			result.Info = check.SkipReason
			logrus.Warnf("skipping %q: %s", check.Name, check.SkipReason)
			continue
		}
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
