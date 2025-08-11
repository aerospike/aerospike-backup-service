package aerospike

import (
	"fmt"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	as "github.com/aerospike/aerospike-client-go/v8"
	"github.com/aerospike/backup-go/models"
	"github.com/aerospike/backup-go/pkg/asinfo"
)

// InfoRequest provides a simplified interface for making Aerospike "info protocol" requests.
type InfoRequest interface {
	// Status retrieves the Aerospike cluster's status string.
	Status(cluster asinfo.NodeGetter) (string, error)
	// DCs returns the list of data center names.
	DCs(cluster asinfo.NodeGetter) ([]string, error)
	// RecordCount returns the number of records in a namespace, optionally filtered by sets.
	RecordCount(cluster asinfo.NodeGetter, namespace string, setList []string) (uint64, error)
	// Namespaces returns the list of namespaces configured in the cluster.
	Namespaces(cluster asinfo.NodeGetter) ([]string, error)
}

// infoRequestImpl is the default InfoRequest implementation.
type infoRequestImpl struct {
	infoPolicy  *as.InfoPolicy
	retryPolicy *models.RetryPolicy
}

// NewInfoRequest creates a new InfoRequest with optional custom policies.
func NewInfoRequest(info *as.InfoPolicy, retry *models.RetryPolicy) InfoRequest {
	if info == nil {
		info = as.NewInfoPolicy()
	}
	if retry == nil {
		retry = model.InfoDefaultRetryPolicy
	}
	return &infoRequestImpl{
		infoPolicy:  info,
		retryPolicy: retry,
	}
}

// buildClient creates an asinfo.Client or returns a wrapped error.
func (r *infoRequestImpl) buildClient(cluster asinfo.NodeGetter) (*asinfo.Client, error) {
	client, err := asinfo.NewClient(cluster, r.infoPolicy, r.retryPolicy)
	if err != nil {
		return nil, fmt.Errorf("create infoRequest client: %w", err)
	}

	return client, nil
}

// Status retrieves the Aerospike cluster's status string.
func (r *infoRequestImpl) Status(cluster asinfo.NodeGetter) (string, error) {
	client, err := r.buildClient(cluster)
	if err != nil {
		return "", err
	}

	return client.GetStatus()
}

// DCs returns the list of data center names.
func (r *infoRequestImpl) DCs(cluster asinfo.NodeGetter) ([]string, error) {
	client, err := r.buildClient(cluster)
	if err != nil {
		return nil, err
	}

	return client.GetDCsList()
}

// RecordCount returns the number of records in a namespace, optionally filtered by sets.
func (r *infoRequestImpl) RecordCount(cluster asinfo.NodeGetter, namespace string, setList []string) (uint64, error) {
	client, err := r.buildClient(cluster)
	if err != nil {
		return 0, err
	}

	return client.GetRecordCount(namespace, setList)
}

// Namespaces returns the list of namespaces configured in the cluster.
func (r *infoRequestImpl) Namespaces(cluster asinfo.NodeGetter) ([]string, error) {
	client, err := r.buildClient(cluster)
	if err != nil {
		return nil, err
	}

	return client.GetNamespacesList()
}
