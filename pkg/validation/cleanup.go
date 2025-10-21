package validation

import (
	"context"
	"errors"
	"time"

	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (v *ValidationRun) cleanupResources(ctx context.Context, complete chan bool) error {
	// block execution until checks have completed or timed out
	<-ctx.Done()
	if ctx.Err() != nil && errors.Is(ctx.Err(), context.DeadlineExceeded) {
		logrus.Errorf("validation timed out")
	}

	logrus.Infof("cleaning up objects created from validation\n")
	if *v.Configuration.SkipCleanup {
		logrus.Info("skipping object cleanup")
		return nil
	}
	// delete in reverse to ensure dependencies are resolved

	logrus.Debugf("need to cleanup %d objects\n", len(v.createdObjects))
	for i := len(v.createdObjects) - 1; i >= 0; i-- {
		obj := v.createdObjects[i]
		// ensure cleanup runs even if `ctx` passed has timed out
		// different context to ensure cleanup happens even if parent context is
		// cancelled due to user initiated termination
		if err := deleteObjectWithRetry(context.TODO(), v.clients.runtimeClient, obj); err != nil {
			// just log error and move on and attempt to clean up remaining objects
			logrus.Errorf("error deleting object %s: %v", obj.GetName(), err)
		}

	}
	complete <- true
	return nil
}

// deleteObjectWithRetry will try and delete object a few times before giving up
// this is essential as some objects may need cleanup of other pending objects
// which may take a while to occur, for example when pvc is deleted, it may take a few seconds for
// associated pv to be cleaned up, and attempts to delete vmimage or storage class
// in that interval may fail
func deleteObjectWithRetry(ctx context.Context, runtimeClient client.Client, obj client.Object) error {
	var err error
	for i := 0; i < maxRetryCount; i++ {
		logrus.Debugf("trying to delete object %v \n", client.ObjectKeyFromObject(obj))
		err = runtimeClient.Delete(ctx, obj)
		if err == nil {
			return nil
		}

		// if object is not found, then ignore and exit
		if apierrors.IsNotFound(err) {
			return nil
		}
		time.Sleep(20 * time.Second)
	}
	return err
}
