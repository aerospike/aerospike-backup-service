package aerospike

import (
	"log/slog"

	_ "github.com/aerospike/aerospike-backup-service/v3/modules/schema" // it's required to load configuration schemas in init method
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util"
	v8 "github.com/aerospike/aerospike-client-go/v8"
	"github.com/aerospike/aerospike-management-lib/asconfig"
	"github.com/aerospike/aerospike-management-lib/info"
	"github.com/go-logr/logr"
)

func ScanClusterConfiguration(client Cluster, logger *slog.Logger) []asconfig.DotConf {
	activeHosts := getActiveHosts(client)

	var outputs = make([]asconfig.DotConf, 0, len(activeHosts))

	policy := DowngradeClientPolicy(client.Cluster().ClientPolicy())
	for _, host := range activeHosts {
		asInfo := info.NewAsInfo(logr.Logger{}, DowngradeHost(host), &policy)

		conf, err := util.TryAndRecoverError(func() (*asconfig.GenConf, error) {
			return asconfig.GenerateConf(logr.Discard(), asInfo, true)
		})
		if err != nil {
			logger.Error("Error reading configuration",
				slog.Any("host", host), slog.Any("err", err))
			continue
		}

		asconf, err := util.TryAndRecoverError(func() (*asconfig.AsConfig, error) {
			return asconfig.NewMapAsConfig(logr.Discard(), conf.Conf)
		})
		if err != nil {
			logger.Error("Error parsing configuration",
				slog.Any("host", host), slog.Any("err", err))
			continue
		}

		configAsString, err := util.TryAndRecover(asconf.ToConfFile)
		if err != nil {
			logger.Error("Error serialising configuration",
				slog.Any("host", host), slog.Any("err", err))
			continue
		}

		outputs = append(outputs, configAsString)
	}

	return outputs
}

func getActiveHosts(client Cluster) []*v8.Host {
	var activeHosts []*v8.Host
	for _, node := range client.Cluster().GetNodes() {
		if node.IsActive() {
			activeHosts = append(activeHosts, node.GetHost())
		}
	}

	return activeHosts
}
