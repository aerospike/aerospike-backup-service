package dto

import (
	"github.com/aerospike/backup-go/models"
)

// Metrics represents the current job speed.
// @Description Metrics represents the current job speed.
type Metrics struct {
	// RecordsPerSecond indicates the number of records processed by Aerospike per second.
	RecordsPerSecond uint64 `json:"records-per-second"`

	// KilobytesPerSecond indicates the amount of data processed by storage per second, in kilobytes.
	KilobytesPerSecond uint64 `json:"kilobytes-per-second"`

	// Pipeline represents the number of records that have been read from the source
	// but not yet written to the destination. This metric helps identify bottlenecks:
	// - If Pipeline is zero or fluctuates near zero, it means the destination is consuming data
	//   faster than the source can read.
	// - If Pipeline grows large, it indicates that the source is producing data faster
	//   than the destination can consume.
	Pipeline int `json:"pipeline"`
}

// NewMetricsFromModel creates a new Metrics DTO from a model.Metrics object.
func NewMetricsFromModel(m *models.Metrics) Metrics {
	var metrics Metrics
	metrics.fromModel(m)
	return metrics
}

// fromModel copies data from a model.Metrics into a DTO Metrics.
func (m *Metrics) fromModel(model *models.Metrics) {
	if model == nil {
		return
	}

	m.Pipeline = model.PipelineReadQueueSize + model.PipelineWriteQueueSize
	m.KilobytesPerSecond = model.KilobytesPerSecond
	m.RecordsPerSecond = model.RecordsPerSecond
}
