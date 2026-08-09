//go:build integration

package integration

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/model"
)

const lastSuccessfulBackupTimestampMetric = "aerospike_backup_service_last_successful_backup_timestamp"

func (e *env) metricsURL() string {
	return e.server.URL + "/metrics"
}

func (e *env) fetchMetricFamilies(ctx context.Context) (map[string]*dto.MetricFamily, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, e.metricsURL(), nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("fetch metrics: status %d: %s", resp.StatusCode, body)
	}

	var parser = expfmt.NewTextParser(model.UTF8Validation)
	return parser.TextToMetricFamilies(resp.Body)
}

func gaugeValue(families map[string]*dto.MetricFamily, name string, labels model.LabelSet) (float64, bool) {
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

func labelsMatch(metricLabels []*dto.LabelPair, want model.LabelSet) bool {
	if len(metricLabels) != len(want) {
		return false
	}

	for _, label := range metricLabels {
		if want[model.LabelName(label.GetName())] != model.LabelValue(label.GetValue()) {
			return false
		}
	}

	return true
}

func (e *env) lastSuccessfulBackupTimestamp(ctx context.Context, backupType string) (float64, bool, error) {
	families, err := e.fetchMetricFamilies(ctx)
	if err != nil {
		return 0, false, err
	}

	value, ok := gaugeValue(families, lastSuccessfulBackupTimestampMetric, model.LabelSet{
		"routine": model.LabelValue(routineName),
		"type":    model.LabelValue(backupType),
	})

	return value, ok, nil
}

// assertLastSuccessfulBackupMetric polls /metrics until the gauge matches the backup Created time.
func (s *Suite) assertLastSuccessfulBackupMetric(e *env, backupType string, want time.Time) {
	s.T().Helper()

	wantUnix := want.Unix()
	deadline := time.Now().Add(backupTimeout)

	for {
		value, ok, err := e.lastSuccessfulBackupTimestamp(s.T().Context(), backupType)
		s.Require().NoError(err)

		if ok && int64(value) == wantUnix {
			return
		}

		if time.Now().After(deadline) {
			if !ok {
				s.Require().Failf("timed out waiting for last successful backup metric",
					"metric %q with routine=%q type=%q not found after %s",
					lastSuccessfulBackupTimestampMetric, routineName, backupType, backupTimeout)
			}

			s.Require().Failf("timed out waiting for last successful backup metric",
				"metric %q with routine=%q type=%q: got %d, want %d after %s",
				lastSuccessfulBackupTimestampMetric, routineName, backupType, int64(value), wantUnix, backupTimeout)
		}

		time.Sleep(pollInterval)
	}
}
