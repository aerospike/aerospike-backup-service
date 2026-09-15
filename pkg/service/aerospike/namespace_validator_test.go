package aerospike

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"sync"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type testEnv struct {
	ctrl              *gomock.Controller
	mockClientManager *MockClientManager
	mockClient        *MockClient
	mockInfoGetter    *MockInfoGetter
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	ctrl := gomock.NewController(t)

	return &testEnv{
		ctrl:              ctrl,
		mockClientManager: NewMockClientManager(ctrl),
		mockClient:        NewMockClient(ctrl),
		mockInfoGetter:    NewMockInfoGetter(ctrl),
	}
}

// expectSingleCluster sets expectations for a single (possibly shared) cluster.
func (e *testEnv) expectSingleClusterFetch(c *model.AerospikeCluster, namespaces []string, err error) {
	e.mockClient.EXPECT().InfoClient().Return(e.mockInfoGetter).AnyTimes()
	e.mockClientManager.EXPECT().GetClient(gomock.Any(), c, gomock.Any(), gomock.Any()).Return(e.mockClient, nil)
	e.mockInfoGetter.EXPECT().GetNamespacesList(gomock.Any()).Return(namespaces, err)

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
	info := NewMockInfoGetter(ctrl)

	mgr.EXPECT().GetClient(gomock.Any(), c, gomock.Any(), gomock.Any()).Return(client, nil)
	client.EXPECT().InfoClient().Return(info)
	info.EXPECT().GetNamespacesList(gomock.Any()).Return(returned, err)
	mgr.EXPECT().Close(client)
}

// clusterMap names the clusters a test configures, the way the service configuration does.
func clusterMap(clusters ...*model.AerospikeCluster) map[string]*model.AerospikeCluster {
	named := make(map[string]*model.AerospikeCluster, len(clusters))
	for i, c := range clusters {
		named["cluster"+strconv.Itoa(i)] = c
	}

	return named
}

func TestFindMissingByRoutine_MissingDetected_SingleFetchForSharedCluster(t *testing.T) {
	env := newTestEnv(t)

	// Same cluster shared by two routines; only one fetch should occur.
	routines := map[string]*model.BackupRoutine{
		"r1": {SourceCluster: cluster, Namespaces: []string{"foo", "bar"}},
		"r2": {SourceCluster: cluster, Namespaces: []string{"foo", "baz"}},
	}

	env.expectSingleClusterFetch(cluster, []string{"foo"}, nil)

	nv := &namespaceValidator{clientManager: env.mockClientManager}
	got := nv.findMissingNamespaces(t.Context(), clusterMap(cluster), routines)

	require.Len(t, got, 2)
	assert.ElementsMatch(t, []string{"bar"}, got["r1"])
	assert.ElementsMatch(t, []string{"baz"}, got["r2"])
}

// A configured cluster is reached even when no routine names a namespace: connectivity is
// checked for every cluster in the configuration, and a routine without namespaces simply
// has nothing to diff against what comes back.
func TestFindMissingByRoutine_NoNamespaces_NothingMissing(t *testing.T) {
	env := newTestEnv(t)

	routines := map[string]*model.BackupRoutine{
		"empty": {SourceCluster: cluster, Namespaces: nil},
	}

	env.expectSingleClusterFetch(cluster, []string{"ns1"}, nil)

	nv := &namespaceValidator{clientManager: env.mockClientManager}
	got := nv.findMissingNamespaces(t.Context(), clusterMap(cluster), routines)

	require.Empty(t, got)
}

// A cluster no routine uses is still reached, so that a credential or address mistake
// surfaces at startup rather than when the first routine is pointed at it.
func TestFindMissingByRoutine_UnusedCluster_IsStillReached(t *testing.T) {
	env := newTestEnv(t)

	env.expectSingleClusterFetch(cluster, []string{"ns1"}, nil)

	nv := &namespaceValidator{clientManager: env.mockClientManager}
	got := nv.findMissingNamespaces(t.Context(), clusterMap(cluster), nil)

	require.Empty(t, got)
}

func TestFindMissingByRoutine_FetchError_SkipsCluster(t *testing.T) {
	env := newTestEnv(t)

	routines := map[string]*model.BackupRoutine{
		"r1": {SourceCluster: cluster, Namespaces: []string{"ns1"}},
	}

	env.mockClientManager.EXPECT().
		GetClient(gomock.Any(), cluster, gomock.Any(), gomock.Any()).
		Return(nil, errors.New("boom")).
		Times(1)

	nv := &namespaceValidator{clientManager: env.mockClientManager}
	got := nv.findMissingNamespaces(t.Context(), clusterMap(cluster), routines)

	require.Empty(t, got)
}

func TestFindMissingByRoutine_InfoError_SkipsCluster(t *testing.T) {
	env := newTestEnv(t)

	routines := map[string]*model.BackupRoutine{
		"r1": {SourceCluster: cluster, Namespaces: []string{"ns1"}},
	}

	env.expectSingleClusterFetch(cluster, nil, errors.New("info failed"))

	nv := &namespaceValidator{clientManager: env.mockClientManager}
	got := nv.findMissingNamespaces(t.Context(), clusterMap(cluster), routines)

	require.Empty(t, got)
}

func TestFindMissingByRoutine_TwoClusters_OK(t *testing.T) {
	ctrl := gomock.NewController(t)

	mgr := NewMockClientManager(ctrl)
	a := &model.AerospikeCluster{ClusterLabel: "A"}
	b := &model.AerospikeCluster{ClusterLabel: "B"}

	routines := map[string]*model.BackupRoutine{
		"r1": {SourceCluster: a, Namespaces: []string{"ns1"}},
		"r2": {SourceCluster: b, Namespaces: []string{"ns2"}},
	}

	expectPerCluster(t, ctrl, mgr, a, []string{"ns1"}, nil)
	expectPerCluster(t, ctrl, mgr, b, []string{"ns2"}, nil)

	nv := &namespaceValidator{clientManager: mgr}
	got := nv.findMissingNamespaces(t.Context(), clusterMap(a, b), routines)

	require.Empty(t, got)
}

func TestFindMissingByRoutine_TwoClusters_Fail(t *testing.T) {
	ctrl := gomock.NewController(t)

	mgr := NewMockClientManager(ctrl)
	a := &model.AerospikeCluster{ClusterLabel: "A"}
	b := &model.AerospikeCluster{ClusterLabel: "B"}

	routines := map[string]*model.BackupRoutine{
		"r1": {SourceCluster: a, Namespaces: []string{"ns2"}},
		"r2": {SourceCluster: b, Namespaces: []string{"ns1"}},
	}

	expectPerCluster(t, ctrl, mgr, a, []string{"ns1"}, nil)
	expectPerCluster(t, ctrl, mgr, b, []string{"ns2"}, nil)

	nv := &namespaceValidator{clientManager: mgr}
	got := nv.findMissingNamespaces(t.Context(), clusterMap(a, b), routines)

	require.Len(t, got, 2)
}

// Clusters have to be dialed concurrently: one cluster that is down would otherwise hold
// up every cluster behind it for its whole connect timeout. Each fetch here waits for all
// the others, so the call only returns if they overlap.
func TestFetchNamespacesByCluster_DialsConcurrently(t *testing.T) {
	ctrl := gomock.NewController(t)
	mgr := NewMockClientManager(ctrl)

	clusters := map[string]*model.AerospikeCluster{
		"a": {ClusterLabel: "A"},
		"b": {ClusterLabel: "B"},
		"c": {ClusterLabel: "C"},
	}

	var arrived sync.WaitGroup
	arrived.Add(len(clusters))

	for _, cluster := range clusters {
		client := NewMockClient(ctrl)
		info := NewMockInfoGetter(ctrl)

		mgr.EXPECT().GetClient(gomock.Any(), cluster, gomock.Any(), gomock.Any()).
			DoAndReturn(func(context.Context, *model.AerospikeCluster, any, any) (Client, error) {
				arrived.Done()
				arrived.Wait()

				return client, nil
			})
		client.EXPECT().InfoClient().Return(info)
		info.EXPECT().GetNamespacesList(gomock.Any()).Return([]string{"ns1"}, nil)
		mgr.EXPECT().Close(client)
	}

	nv := &namespaceValidator{clientManager: mgr}
	got := nv.fetchNamespacesByCluster(t.Context(), clusters)

	require.Len(t, got, len(clusters))
}

// A probe that fails because the service is shutting down says nothing about the cluster,
// so it must not be reported as one that is unavailable. The startup check runs in a
// goroutine, so it straddles shutdown routinely.
func TestFetchNamespacesByCluster_CanceledContextIsNotReported(t *testing.T) {
	env := newTestEnv(t)
	warnings := captureWarnings(t)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	env.mockClientManager.EXPECT().
		GetClient(gomock.Any(), cluster, gomock.Any(), gomock.Any()).
		Return(nil, context.Canceled)

	nv := &namespaceValidator{clientManager: env.mockClientManager}

	require.Empty(t, nv.fetchNamespacesByCluster(ctx, clusterMap(cluster)))
	assert.Empty(t, warnings.messages())
}

// A cluster that is genuinely unreachable is still reported, so the guard above cannot
// pass by silencing everything.
func TestFetchNamespacesByCluster_UnreachableClusterIsReported(t *testing.T) {
	env := newTestEnv(t)
	warnings := captureWarnings(t)

	env.mockClientManager.EXPECT().
		GetClient(gomock.Any(), cluster, gomock.Any(), gomock.Any()).
		Return(nil, errors.New("connection refused"))

	nv := &namespaceValidator{clientManager: env.mockClientManager}

	require.Empty(t, nv.fetchNamespacesByCluster(t.Context(), clusterMap(cluster)))
	assert.Equal(t, []string{"Configured Aerospike cluster is not available"}, warnings.messages())
}

// recorder collects the warnings the validator writes to the default logger.
type recorder struct {
	slog.Handler

	mu      sync.Mutex
	records []slog.Record
}

func captureWarnings(t *testing.T) *recorder {
	t.Helper()

	r := &recorder{Handler: slog.DiscardHandler}
	previous := slog.Default()
	slog.SetDefault(slog.New(r))
	t.Cleanup(func() { slog.SetDefault(previous) })

	return r
}

func (r *recorder) Enabled(context.Context, slog.Level) bool { return true }

func (r *recorder) Handle(_ context.Context, record slog.Record) error {
	if record.Level < slog.LevelWarn {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.records = append(r.records, record)

	return nil
}

func (r *recorder) messages() []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	messages := make([]string, 0, len(r.records))
	for _, record := range r.records {
		messages = append(messages, record.Message)
	}

	return messages
}
