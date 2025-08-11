package aerospike

import (
	as "github.com/aerospike/aerospike-client-go/v8"
	"github.com/aerospike/backup-go/pkg/asinfo"
)

// GetDCNames returns the list of data center names from a cluster.
func GetDCNames(client Cluster) ([]string, error) {
	infoclient, err := asinfo.NewClient(client.Cluster(), as.NewInfoPolicy(), nil)
	if err != nil {
		return nil, err
	}

	return infoclient.GetDCsList()
}
