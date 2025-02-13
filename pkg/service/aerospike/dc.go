package aerospike

import (
	"fmt"
	"strings"

	as "github.com/aerospike/aerospike-client-go/v8"
)

// GetDCNames returns the list of data center names from a cluster.
func GetDCNames(client Cluster) ([]string, error) {
	node, err := client.Cluster().GetRandomNode()
	if err != nil {
		return nil, fmt.Errorf("failed to get node: %w", err)
	}

	infoRes, err := node.RequestInfo(&as.InfoPolicy{}, "get-config:context=xdr")
	if err != nil {
		return nil, fmt.Errorf("failed to get DC info: %w", err)
	}

	// sample response: dcs=;src-id=0;trace-sample=0
	for _, config := range infoRes {
		lines := strings.Split(config, ";")
		for _, line := range lines {
			if strings.HasPrefix(line, "dcs=") {
				dcNames := strings.TrimPrefix(line, "dcs=")
				if dcNames == "" {
					return nil, nil // no DCs are configured
				}
				return strings.Split(dcNames, ","), nil
			}
		}
	}

	return nil, nil
}
