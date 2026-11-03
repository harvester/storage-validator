package validation

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/harvester/storage-validator/pkg/api"
)

func boolPtr(b bool) *bool { return &b }

func testNode(name string, ready corev1.ConditionStatus) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: ready}},
		},
	}
}

func newPreFlightRun(cfg *api.Configuration, nodes ...*corev1.Node) *ValidationRun {
	builder := fake.NewClientBuilder().WithScheme(scheme)
	for _, n := range nodes {
		builder = builder.WithObjects(n)
	}
	return &ValidationRun{
		ctx:           context.TODO(),
		Configuration: cfg,
		clients:       HarvesterClient{runtimeClient: builder.Build()},
	}
}

func Test_ApplyOverrides(t *testing.T) {
	assert := require.New(t)

	// unset flags keep the config file values
	v := &ValidationRun{Configuration: &api.Configuration{AccessMode: "ReadWriteOnce"}}
	v.applyOverrides()
	assert.Equal("ReadWriteOnce", v.Configuration.AccessMode)
	assert.Nil(v.Configuration.SingleNode)
	assert.Nil(v.Configuration.SkipMigration)

	// set flags win over the config file
	v = &ValidationRun{
		Configuration:         &api.Configuration{AccessMode: "ReadWriteMany", SkipMigration: boolPtr(false)},
		AccessModeOverride:    "ReadWriteOnce",
		SingleNodeOverride:    true,
		SkipMigrationOverride: true,
	}
	v.applyOverrides()
	assert.Equal("ReadWriteOnce", v.Configuration.AccessMode)
	assert.True(*v.Configuration.SingleNode)
	assert.True(*v.Configuration.SkipMigration)
}

func Test_SkipMigration(t *testing.T) {
	assert := require.New(t)
	cases := []struct {
		name          string
		singleNode    *bool
		skipMigration *bool
		want          bool
	}{
		{"defaults run migration", nil, nil, false},
		{"skip-migration skips", nil, boolPtr(true), true},
		{"single-node implies skip", boolPtr(true), nil, true},
		{"single-node wins over skipMigration=false", boolPtr(true), boolPtr(false), true},
		{"both false run migration", boolPtr(false), boolPtr(false), false},
	}
	for _, c := range cases {
		v := &ValidationRun{Configuration: &api.Configuration{SingleNode: c.singleNode, SkipMigration: c.skipMigration}}
		assert.Equal(c.want, v.skipMigration(), c.name)
	}
}

func Test_AccessModes(t *testing.T) {
	assert := require.New(t)
	v := &ValidationRun{Configuration: &api.Configuration{}}
	assert.Equal([]corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany}, v.accessModes())
	v.Configuration.AccessMode = "ReadWriteOnce"
	assert.Equal([]corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}, v.accessModes())
}

func Test_PreFlightChecks(t *testing.T) {
	assert := require.New(t)
	const image = "http://example.com/image.qcow2"

	// one Ready node: rejected by default, accepted in single-node mode
	v := newPreFlightRun(&api.Configuration{ImageURL: image}, testNode("n1", corev1.ConditionTrue))
	assert.ErrorContains(v.preFlightChecks(), "atleast 2 Ready node(s)")
	v = newPreFlightRun(&api.Configuration{ImageURL: image, SingleNode: boolPtr(true)},
		testNode("n1", corev1.ConditionTrue))
	assert.NoError(v.preFlightChecks())

	// a NotReady node does not count towards the minimum
	v = newPreFlightRun(&api.Configuration{ImageURL: image},
		testNode("n1", corev1.ConditionTrue), testNode("n2", corev1.ConditionFalse))
	assert.Error(v.preFlightChecks())
	v = newPreFlightRun(&api.Configuration{ImageURL: image, SingleNode: boolPtr(true)},
		testNode("n1", corev1.ConditionFalse))
	assert.ErrorContains(v.preFlightChecks(), "atleast 1 Ready node(s)")

	// two Ready nodes pass without single-node mode
	v = newPreFlightRun(&api.Configuration{ImageURL: image},
		testNode("n1", corev1.ConditionTrue), testNode("n2", corev1.ConditionTrue))
	assert.NoError(v.preFlightChecks())

	// imageURL is still required
	v = newPreFlightRun(&api.Configuration{SingleNode: boolPtr(true)}, testNode("n1", corev1.ConditionTrue))
	assert.ErrorContains(v.preFlightChecks(), "no imageURL")
}

// verifyPodIsReady used to return true for a pod that was NOT Running, so the
// volume checks passed before the consumer pod ever mounted the volume.
func Test_VerifyPodIsReady(t *testing.T) {
	assert := require.New(t)
	for phase, want := range map[corev1.PodPhase]bool{
		corev1.PodPending:   false,
		corev1.PodRunning:   true,
		corev1.PodFailed:    false,
		corev1.PodSucceeded: false,
	} {
		ready, err := verifyPodIsReady(&corev1.Pod{Status: corev1.PodStatus{Phase: phase}})
		assert.NoError(err)
		assert.Equal(want, ready, string(phase))
	}
	_, err := verifyPodIsReady(&corev1.PersistentVolumeClaim{})
	assert.Error(err)
}

func Test_IsFSResizeRequired(t *testing.T) {
	assert := require.New(t)
	pvc := &corev1.PersistentVolumeClaim{}
	assert.False(IsFSResizeRequired(pvc))
	pvc.Status.Conditions = []corev1.PersistentVolumeClaimCondition{{
		Type:   corev1.PersistentVolumeClaimFileSystemResizePending,
		Status: corev1.ConditionTrue,
	}}
	assert.True(IsFSResizeRequired(pvc))
}
