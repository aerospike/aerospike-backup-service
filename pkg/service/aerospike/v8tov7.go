package aerospike

import (
	v7 "github.com/aerospike/aerospike-client-go/v7"
	v8 "github.com/aerospike/aerospike-client-go/v8"
)

func DowngradeHost(h *v8.Host) *v7.Host {
	if h == nil {
		return nil
	}

	return &v7.Host{
		Name:    h.Name,
		TLSName: h.TLSName,
		Port:    h.Port,
	}
}

// DowngradeClientPolicy copies a v8.ClientPolicy to a v7.ClientPolicy.
func DowngradeClientPolicy(cp v8.ClientPolicy) v7.ClientPolicy {
	newCp := v7.ClientPolicy{
		AuthMode:                    v7.AuthMode(cp.AuthMode),
		User:                        cp.User,
		Password:                    cp.Password,
		ClusterName:                 cp.ClusterName,
		Timeout:                     cp.Timeout,
		IdleTimeout:                 cp.IdleTimeout,
		LoginTimeout:                cp.LoginTimeout,
		ConnectionQueueSize:         cp.ConnectionQueueSize,
		MinConnectionsPerNode:       cp.MinConnectionsPerNode,
		MaxErrorRate:                cp.MaxErrorRate,
		ErrorRateWindow:             cp.ErrorRateWindow,
		LimitConnectionsToQueueSize: cp.LimitConnectionsToQueueSize,
		OpeningConnectionThreshold:  cp.OpeningConnectionThreshold,
		FailIfNotConnected:          cp.FailIfNotConnected,
		TendInterval:                cp.TendInterval,
		UseServicesAlternate:        cp.UseServicesAlternate,
		RackAware:                   cp.RackAware,
		IgnoreOtherSubnetAliases:    cp.IgnoreOtherSubnetAliases,
		SeedOnlyCluster:             cp.SeedOnlyCluster,
	}

	// Deep copy map
	if cp.IpMap != nil {
		newCp.IpMap = make(map[string]string, len(cp.IpMap))
		for k, v := range cp.IpMap {
			newCp.IpMap[k] = v
		}
	}

	// Deep copy slice
	if cp.RackIds != nil {
		newCp.RackIds = make([]int, len(cp.RackIds))
		copy(newCp.RackIds, cp.RackIds)
	}

	// Deep copy TlsConfig if exists
	if cp.TlsConfig != nil {
		newCp.TlsConfig = cp.TlsConfig
	}

	return newCp
}
