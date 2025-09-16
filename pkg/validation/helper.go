package validation

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/harvester/storage-validator/pkg/api"
)

// fetch and verify that specified object is ready
// else keep retrying till verification times out
func (v *ValidationRun) waitUntilObjectIsReady(ctx context.Context, obj client.Object, check func(obj client.Object) (bool, error)) error {
	for {
		err := v.clients.runtimeClient.Get(ctx, client.ObjectKeyFromObject(obj), obj)
		if err != nil {
			return fmt.Errorf("error getting object %v: %w", client.ObjectKeyFromObject(obj), err)
		}

		ready, err := check(obj)
		if err != nil {
			return err
		}
		if !ready {
			logrus.Debugf("waiting for object %v to reach desired state\n", client.ObjectKeyFromObject(obj))
			time.Sleep(5 * time.Second)
		} else {
			return nil
		}

	}
}

func (v *ValidationRun) AddResult(result api.Result) {
	v.Report.Results = append(v.Report.Results, result)
}

func initiateCheck(msg string) {
	logrus.Infof("🚀 initiate: %s\n", msg)
}

func completedCheck(msg string) {
	logrus.Infof("✅  completed: %s\n", msg)
}
