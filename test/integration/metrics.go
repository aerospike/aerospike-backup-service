//go:build integration

package integration

import (
	"bytes"
	"net/http"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	promdto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	prommodel "github.com/prometheus/common/model"
)

const (
	lastSuccessfulBackupTimestampMetric = "aerospike_backup_service_last_successful_backup_timestamp"
	backupEventsTotalMetric             = "aerospike_backup_service_backup_events_total"
	restoreEventsTotalMetric            = "aerospike_backup_service_restore_events_total"

	labelRoutine = "routine"
	labelType    = "type"
	labelOutcome = "outcome"

	outcomeSuccess = "success"
	outcomeFailure = "failure"
)

// metricBackupSuccessEventCount returns backup_events_total{outcome="success"} for
// the baseConfig routine and backupType. Prometheus omits a counter until its first
// event, so a missing series counts as zero.
func (s *Suite) metricBackupSuccessEventCount(e *env, backupType model.BackupType) int {
	return int(s.metricBackupEventCount(e, backupType, outcomeSuccess))
}

// metricRestoreSuccessEventCount returns restore_events_total{outcome="success"},
// with a missing series counting as zero.
func (s *Suite) metricRestoreSuccessEventCount(e *env) int {
	value, _ := s.restoreSuccessEventCount(e)

	return int(value)
}

func (s *Suite) metricBackupEventCount(e *env, backupType model.BackupType, outcome string) float64 {
	value, _ := s.backupEventCount(e, backupType, outcome)

	return value
}

func (s *Suite) backupEventCount(e *env, backupType model.BackupType, outcome string) (float64, bool) {
	return s.metricValue(e, backupEventsTotalMetric, prommodel.LabelSet{
		labelRoutine: routineName,
		labelType:    prommodel.LabelValue(backupType),
		labelOutcome: prommodel.LabelValue(outcome),
	})
}

// restoreSuccessEventCount reads restore_events_total{outcome="success"}. Unlike
// backup events, restore events carry no routine or type labels.
func (s *Suite) restoreSuccessEventCount(e *env) (float64, bool) {
	return s.metricValue(e, restoreEventsTotalMetric, prommodel.LabelSet{
		labelOutcome: outcomeSuccess,
	})
}

func (s *Suite) lastSuccessfulBackupTimestamp(e *env, backupType model.BackupType) (float64, bool) {
	return s.metricValue(e, lastSuccessfulBackupTimestampMetric, prommodel.LabelSet{
		labelRoutine: routineName,
		labelType:    prommodel.LabelValue(backupType),
	})
}

// metricValue scrapes /metrics and returns the counter or gauge named name whose
// labels are exactly labels, and whether that series exists.
func (s *Suite) metricValue(e *env, name string, labels prommodel.LabelSet) (float64, bool) {
	status, body := s.do(e, http.MethodGet, e.baseURL+"/metrics", nil)
	s.Require().Equal(http.StatusOK, status, "fetch metrics: %s", body)

	parser := expfmt.NewTextParser(prommodel.UTF8Validation)
	families, err := parser.TextToMetricFamilies(bytes.NewReader(body))
	s.Require().NoError(err)

	family := families[name]
	if family == nil {
		return 0, false
	}

	for _, metric := range family.GetMetric() {
		if !labelsMatch(metric.GetLabel(), labels) {
			continue
		}

		if counter := metric.GetCounter(); counter != nil {
			return counter.GetValue(), true
		}

		if gauge := metric.GetGauge(); gauge != nil {
			return gauge.GetValue(), true
		}
	}

	return 0, false
}

func labelsMatch(metricLabels []*promdto.LabelPair, want prommodel.LabelSet) bool {
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
