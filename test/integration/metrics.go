//go:build integration

package integration

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	prommodel "github.com/prometheus/common/model"
)

const (
	lastSuccessfulBackupTimestampMetric = "aerospike_backup_service_last_successful_backup_timestamp"
	backupEventsTotalMetric             = "aerospike_backup_service_backup_events_total"
	restoreEventsTotalMetric            = "aerospike_backup_service_restore_events_total"
)

func (e *env) metricsURL() string {
	return e.baseURL + "/metrics"
}

func (e *env) fetchMetricFamilies(ctx context.Context) (map[string]*dto.MetricFamily, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, e.metricsURL(), nil)
	if err != nil {
		return nil, err
	}

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("fetch metrics: status %d: %s", resp.StatusCode, body)
	}

	var parser = expfmt.NewTextParser(prommodel.UTF8Validation)

	return parser.TextToMetricFamilies(resp.Body)
}

func gaugeValue(families map[string]*dto.MetricFamily, name string, labels prommodel.LabelSet) (float64, bool) {
	mf := families[name]
	if mf == nil {
		return 0, false
	}

	for _, metric := range mf.GetMetric() {
		if !labelsMatch(metric.GetLabel(), labels) {
			continue
		}

		if gauge := metric.GetGauge(); gauge != nil {
			return gauge.GetValue(), true
		}
	}

	return 0, false
}

func counterValue(families map[string]*dto.MetricFamily, name string, labels prommodel.LabelSet) (float64, bool) {
	mf := families[name]
	if mf == nil {
		return 0, false
	}

	for _, metric := range mf.GetMetric() {
		if !labelsMatch(metric.GetLabel(), labels) {
			continue
		}

		if counter := metric.GetCounter(); counter != nil {
			return counter.GetValue(), true
		}
	}

	return 0, false
}

func labelsMatch(metricLabels []*dto.LabelPair, want prommodel.LabelSet) bool {
	if len(metricLabels) != len(want) {
		return false
	}

	for _, label := range metricLabels {
		if want[prommodel.LabelName(label.GetName())] != prommodel.LabelValue(label.GetValue()) {
			return false
		}
	}

	return true
}

func (e *env) lastSuccessfulBackupTimestamp(ctx context.Context, backupType model.BackupType) (float64, bool, error) {
	families, err := e.fetchMetricFamilies(ctx)
	if err != nil {
		return 0, false, err
	}

	value, ok := gaugeValue(families, lastSuccessfulBackupTimestampMetric, prommodel.LabelSet{
		"routine": prommodel.LabelValue(routineName),
		"type":    prommodel.LabelValue(backupType),
	})

	return value, ok, nil
}

func (e *env) backupSuccessEventCount(ctx context.Context, backupType model.BackupType) (float64, bool, error) {
	families, err := e.fetchMetricFamilies(ctx)
	if err != nil {
		return 0, false, err
	}

	value, ok := counterValue(families, backupEventsTotalMetric, prommodel.LabelSet{
		"routine": prommodel.LabelValue(routineName),
		"type":    prommodel.LabelValue(backupType),
		"outcome": "success",
	})

	return value, ok, nil
}

func (e *env) backupFailureEventCount(ctx context.Context, backupType model.BackupType) (float64, bool, error) {
	families, err := e.fetchMetricFamilies(ctx)
	if err != nil {
		return 0, false, err
	}

	value, ok := counterValue(families, backupEventsTotalMetric, prommodel.LabelSet{
		"routine": prommodel.LabelValue(routineName),
		"type":    prommodel.LabelValue(backupType),
		"outcome": "failure",
	})

	return value, ok, nil
}

// restoreSuccessEventCount returns restore_events_total{outcome="success"}. Unlike backup events,
// restore events carry no routine/type labels.
func (e *env) restoreSuccessEventCount(ctx context.Context) (float64, bool, error) {
	families, err := e.fetchMetricFamilies(ctx)
	if err != nil {
		return 0, false, err
	}

	value, ok := counterValue(families, restoreEventsTotalMetric, prommodel.LabelSet{
		"outcome": "success",
	})

	return value, ok, nil
}
