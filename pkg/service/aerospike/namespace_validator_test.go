package aerospike

import (
	"errors"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/backup-go/mocks"
	"github.com/aws/smithy-go/ptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type testEnv struct {
	ctrl              *gomock.Controller
	mockClientManager *MockClientManager
	mockClient        *MockClient
	mockInfoGetter    *mocks.MockInfoGetter
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	return &testEnv{
		ctrl:              ctrl,
		mockClientManager: NewMockClientManager(ctrl),
		mockClient:        NewMockClient(ctrl),
		mockInfoGetter:    mocks.NewMockInfoGetter(t),
	}
}

// expectSingleCluster sets expectations for a single (possibly shared) cluster.
func (e *testEnv) expectSingleClusterFetch(c *model.AerospikeCluster, namespaces []string, err error) {
	e.mockClient.EXPECT().InfoClient().Return(e.mockInfoGetter).AnyTimes()
	e.mockClientManager.EXPECT().GetClient(c).Return(e.mockClient, nil)
	e.mockInfoGetter.EXPECT().GetNamespacesList().Return(namespaces, err)
	e.mockClientManager.EXPECT().Close(e.mockClient)
}

// expectPerCluster creates fresh client+info mocks per cluster to keep tests independent.
func expectPerCluster(
	t *testing.T,
	ctrl *gomock.Controller,
	mgr *MockClientManager,
	c *model.AerospikeCluster,
	returned []string,
	err error,
) {
	t.Helper()
	client := NewMockClient(ctrl)
	info := mocks.NewMockInfoGetter(t)

	mgr.EXPECT().GetClient(c).Return(client, nil)
	client.EXPECT().InfoClient().Return(info)
	info.EXPECT().GetNamespacesList().Return(returned, err)
	mgr.EXPECT().Close(client)
}

func TestFindMissingByRoutine_MissingDetected_SingleFetchForSharedCluster(t *testing.T) {
	env := newTestEnv(t)

	// Same cluster shared by two routines; only one fetch should occur.
	routines := map[string]*model.BackupRoutine{
		"r1": {SourceCluster: cluster, Namespaces: []string{"foo", "bar"}},
		"r2": {SourceCluster: cluster, Namespaces: []string{"foo", "baz"}},
	}

	env.expectSingleClusterFetch(cluster, []string{"foo"}, nil)

	nv := &NamespaceValidatorImpl{clientManager: env.mockClientManager}
	got := nv.findMissingByRoutine(routines)

	require.Len(t, got, 2)
	assert.ElementsMatch(t, []string{"bar"}, got["r1"])
	assert.ElementsMatch(t, []string{"baz"}, got["r2"])
}

func TestFindMissingByRoutine_NoNamespaces_NoFetch(t *testing.T) {
	env := newTestEnv(t)

	routines := map[string]*model.BackupRoutine{
		"empty": {SourceCluster: cluster, Namespaces: nil},
	}

	nv := &NamespaceValidatorImpl{clientManager: env.mockClientManager}
	got := nv.findMissingByRoutine(routines)

	require.Empty(t, got)
}

func TestFindMissingByRoutine_FetchError_SkipsCluster(t *testing.T) {
	env := newTestEnv(t)

	routines := map[string]*model.BackupRoutine{
		"r1": {SourceCluster: cluster, Namespaces: []string{"ns1"}},
	}

	env.mockClientManager.EXPECT().
		GetClient(cluster).
		Return(nil, errors.New("boom")).
		Times(1)

	nv := &NamespaceValidatorImpl{clientManager: env.mockClientManager}
	got := nv.findMissingByRoutine(routines)

	require.Empty(t, got)
}

func TestFindMissingByRoutine_InfoError_SkipsCluster(t *testing.T) {
	env := newTestEnv(t)

	routines := map[string]*model.BackupRoutine{
		"r1": {SourceCluster: cluster, Namespaces: []string{"ns1"}},
	}

	env.expectSingleClusterFetch(cluster, nil, errors.New("info failed"))

	nv := &NamespaceValidatorImpl{clientManager: env.mockClientManager}
	got := nv.findMissingByRoutine(routines)

	require.Empty(t, got)
}

func TestFindMissingByRoutine_TwoClusters_OK(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	mgr := NewMockClientManager(ctrl)
	a := &model.AerospikeCluster{ClusterLabel: ptr.String("A")}
	b := &model.AerospikeCluster{ClusterLabel: ptr.String("B")}

	routines := map[string]*model.BackupRoutine{
		"r1": {SourceCluster: a, Namespaces: []string{"ns1"}},
		"r2": {SourceCluster: b, Namespaces: []string{"ns2"}},
	}

	expectPerCluster(t, ctrl, mgr, a, []string{"ns1"}, nil)
	expectPerCluster(t, ctrl, mgr, b, []string{"ns2"}, nil)

	nv := &NamespaceValidatorImpl{clientManager: mgr}
	got := nv.findMissingByRoutine(routines)

	require.Empty(t, got)
}

func TestFindMissingByRoutine_TwoClusters_Fail(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	mgr := NewMockClientManager(ctrl)
	a := &model.AerospikeCluster{ClusterLabel: ptr.String("A")}
	b := &model.AerospikeCluster{ClusterLabel: ptr.String("B")}

	routines := map[string]*model.BackupRoutine{
		"r1": {SourceCluster: a, Namespaces: []string{"ns2"}},
		"r2": {SourceCluster: b, Namespaces: []string{"ns1"}},
	}

	expectPerCluster(t, ctrl, mgr, a, []string{"ns1"}, nil)
	expectPerCluster(t, ctrl, mgr, b, []string{"ns2"}, nil)

	nv := &NamespaceValidatorImpl{clientManager: mgr}
	got := nv.findMissingByRoutine(routines)

	require.Len(t, got, 2)
}
