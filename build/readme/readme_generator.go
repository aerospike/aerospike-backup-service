package main

import (
	"bytes"
	"cmp"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto/decoder"
	metrics "github.com/aerospike/aerospike-backup-service/v3/pkg/service/prometheus"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
	"github.com/prometheus/client_golang/prometheus"
	"gopkg.in/yaml.v3"
)

const (
	examplesDir   = "docs/examples"
	readmeRelPath = "README.md"
)

var allStorageTypes = map[string]dto.Storage{
	"local": {
		LocalStorage: &dto.LocalStorage{
			Path: "backups",
		},
	},
	"aws-s3": {
		S3Storage: &dto.S3Storage{
			Bucket:   "as-backup-bucket",
			Path:     "backups",
			S3Region: "eu-central-1",
		},
	},
	"gcp-gcs": {
		GcpStorage: &dto.GcpStorage{
			Path:       "backups",
			KeyFile:    "key-file.json",
			BucketName: "gcp-backup-bucket",
			Endpoint:   "http://127.0.0.1:9020",
		},
	},
	"azure-blob-storage": {
		AzureStorage: &dto.AzureStorage{
			Path:          "backups",
			Endpoint:      "http://127.0.0.1:6000/devstoreaccount1",
			AccountName:   "devstoreaccount1",
			ContainerName: "testcontainer",
			AccountKey:    "Eby8vdM02xNOcqFlqUwJPLlmEtlCDXJ1OUzFT50uSRZ6IFsuFq2UVErCz4I6tq/K1SZFPTOtr/KBHBeksoGMGw==",
		},
	},
}

var cluster = dto.AerospikeCluster{
	SeedNodes: []dto.SeedNode{{
		HostName: "host.docker.internal", Port: 3000},
	},
	Credentials: &dto.Credentials{
		User:     ptr.Of("user"),
		Password: ptr.Of("password"),
	},
}

var jsonExamples = map[string]any{
	"ClustersResponse": []dto.AerospikeCluster{cluster},
	"RoutinesResponse": map[string]dto.BackupRoutine{
		"routine1": {
			BackupPolicy:  "keepFilesPolicy",
			SourceCluster: "absDefaultCluster",
			Storage:       "local",
			IntervalCron:  "@yearly",
			Namespaces:    ptr.Of([]string{"test-namespace"}),
		},
		"routine2": {
			BackupPolicy:     "removeFilesPolicy",
			SourceCluster:    "absDefaultCluster",
			Storage:          "local",
			IntervalCron:     "@monthly",
			IncrIntervalCron: "@daily",
			Namespaces:       ptr.Of([]string{"test-namespace"}),
			SetList:          []string{"backupSet"},
			BinList:          []string{"backupBin"},
		},
	},
	"StorageResponse": allStorageTypes,
	"FullBackupsResponse": map[string][]dto.BackupDetails{
		"routine1": {{
			Created:             time.Date(2024, 01, 01, 12, 0, 0, 0, time.UTC),
			Timestamp:           time.Date(2024, 01, 01, 12, 0, 0, 0, time.UTC).UnixMilli(),
			Finished:            time.Date(2024, 01, 01, 12, 5, 0, 0, time.UTC),
			Duration:            300,
			From:                time.Time{},
			Namespace:           "source-ns1",
			RecordCount:         42,
			ByteCount:           480_000,
			FileCount:           1,
			SecondaryIndexCount: 5,
			UDFCount:            1,
			Key:                 "routine1/backup/1704110400000/source-ns1",
			Storage: &dto.Storage{
				S3Storage: &dto.S3Storage{
					Bucket:   "as-backup-bucket",
					Path:     "backups",
					S3Region: "eu-central-1",
				},
			},
			Compression: dto.CompressZSTD,
			Encryption:  dto.EncryptNone,
		},
		},
	},
	"RestoreFullRequest": dto.RestoreRequest{
		DestinationClusterConfig: dto.DestinationClusterConfig{
			Cluster: &cluster,
		},
		Policy: &dto.RestorePolicy{
			BaseRestorePolicy: dto.BaseRestorePolicy{
				NoGeneration: ptr.Of(true),
			},
		},
		StorageConfig: dto.StorageConfig{
			Storage: &dto.Storage{
				S3Storage: &dto.S3Storage{
					Bucket:   "as-backup-bucket",
					Path:     "backups",
					S3Region: "eu-central-1",
				},
			},
		},
		BackupDataPath: "routine1/backup/1704110400000/source-ns1",
	},
	"RestoreTimestampRequest": dto.RestoreTimestampRequest{
		DestinationClusterConfig: dto.DestinationClusterConfig{
			Name: "abs-cluster",
		},
		Time:    1704110400000,
		Routine: "routine1",
	},
	"CurrentBackupResponse": dto.RoutineState{
		Full: &dto.RunningJob{
			TotalRecords:     100_000,
			DoneRecords:      50_000,
			StartTime:        time.Date(2024, 01, 01, 12, 0, 0, 0, time.UTC),
			FinishTime:       nil,
			PercentageDone:   50,
			EstimatedEndTime: ptr.Of(time.Date(2024, 01, 01, 13, 0, 0, 0, time.UTC)),
			Duration:         1800,
			Metrics: &dto.Metrics{
				RecordsPerSecond:   1000,
				KilobytesPerSecond: 30000,
				Pipeline:           167,
			},
		},
	},
	"CurrentRestoreResponse": dto.RestoreJobStatus{
		ReadRecords:     100_000,
		TotalBytes:      30000000,
		ExpiredRecords:  0,
		SkippedRecords:  0,
		IgnoredRecords:  0,
		InsertedRecords: 50_000,
		ExistedRecords:  0,
		FresherRecords:  0,
		IndexCount:      4,
		UDFCount:        1,
		ErrorsInDoubt:   0,
		CurrentRestore: &dto.RunningJob{
			TotalRecords:     100_000,
			DoneRecords:      50_000,
			StartTime:        time.Date(2024, 01, 01, 12, 0, 0, 0, time.UTC),
			FinishTime:       nil,
			PercentageDone:   50,
			EstimatedEndTime: ptr.Of(time.Date(2024, 01, 01, 13, 0, 0, 0, time.UTC)),
			Duration:         1800,
			Metrics: &dto.Metrics{
				RecordsPerSecond:   1000,
				KilobytesPerSecond: 30000,
				Pipeline:           8192,
			},
		},
		Status: dto.JobStatusRunning,
		Error:  "",
	},
	"CurrentRestoresResponse": map[int]dto.RestoreJobStatus{
		12345678: {
			ReadRecords:     100_000,
			TotalBytes:      30000000,
			ExpiredRecords:  0,
			SkippedRecords:  0,
			IgnoredRecords:  0,
			InsertedRecords: 50_000,
			ExistedRecords:  0,
			FresherRecords:  0,
			IndexCount:      4,
			UDFCount:        1,
			ErrorsInDoubt:   0,
			CurrentRestore: &dto.RunningJob{
				TotalRecords:     100_000,
				DoneRecords:      50_000,
				StartTime:        time.Date(2024, 01, 01, 12, 0, 0, 0, time.UTC),
				FinishTime:       nil,
				PercentageDone:   50,
				Duration:         1800,
				EstimatedEndTime: ptr.Of(time.Date(2024, 01, 01, 13, 0, 0, 0, time.UTC)),
				Metrics: &dto.Metrics{
					RecordsPerSecond:   1000,
					KilobytesPerSecond: 30000,
					Pipeline:           0,
				},
			},
			Status: dto.JobStatusRunning,
			Error:  "",
		}},
}

var yamlExamples = map[string]any{
	"Storage": allStorageTypes,
	"RemoteConfig": dto.Storage{
		S3Storage: &dto.S3Storage{
			Path:     "config.yml",
			Bucket:   "as-backup-bucket",
			S3Region: "eu-central-1",
		},
	},
}

func main() {
	// generate markdown dto descriptions from open-api
	generateMarkdownFiles()
	// generate example files from jsonExamples and yamlExamples
	generateExampleFiles()

	// Update README sections
	readme, err := os.ReadFile(readmeRelPath)
	if err != nil {
		panic(err)
	}

	// replace every <!-- DTONAME --> comment with a real example from jsonExamples and yamlExamples
	readme = updateDtoExamples(readme)
	// copy example configuration (with explanatory comments) to a <!-- DefaultConfig --> section
	readme = updateDefaultConfigSection(readme)
	// add Prometheus metrics explanation table after <!-- Metrics -->
	readme = updateMetrics(readme)

	//nolint:gosec // G703 readmeRelPath is a fixed project path; readme is generated from the same file and templates.
	err = os.WriteFile(readmeRelPath, readme, 0600)
	if err != nil {
		panic(err)
	}
}

func generateExampleFiles() {
	_ = os.RemoveAll(examplesDir)
	_ = os.MkdirAll(examplesDir, 0755)

	for name, example := range jsonExamples {
		fileName := filepath.Join(examplesDir, name+".json")
		fileContent, err := json.MarshalIndent(example, "", "  ")
		if err != nil {
			panic(fmt.Errorf("failed to marshal json example %q: %w", name, err))
		}
		fileContent = fmt.Appendln(fileContent, "")
		err = os.WriteFile(fileName, fileContent, 0600)
		if err != nil {
			panic(fmt.Errorf("failed to write json example file %q: %w", fileName, err))
		}
	}

	for name, example := range yamlExamples {
		fileName := filepath.Join(examplesDir, name+".yaml")
		fileContent, err := marshalYAML(example)
		if err != nil {
			panic(fmt.Errorf("failed to marshal yaml example %q: %w", name, err))
		}
		err = os.WriteFile(fileName, fileContent, 0600)
		if err != nil {
			panic(fmt.Errorf("failed to write yaml example file %q: %w", fileName, err))
		}
	}
}

func updateDtoExamples(readme []byte) []byte {
	// comment containing an example name (e.g.,key from jsonExamples)
	// followed by ```json/```yaml and the example code block.
	re := regexp.MustCompile("<!--\\s*(\\w+)\\s*-->\\s*```(json|yaml)[\\s\\S]*?```")

	updatedReadme := re.ReplaceAllFunc(readme, func(match []byte) []byte {
		submatches := re.FindSubmatch(match)
		if len(submatches) < 3 {
			panic(fmt.Errorf("failed to find submatch: %s", submatches))
		}

		name := string(submatches[1])
		format := string(submatches[2])

		var formattedExample []byte

		var err error
		switch format {
		case "json":
			example, exists := jsonExamples[name]
			if exists {
				formattedExample, err = json.MarshalIndent(example, "", "  ")
			}
		case "yaml":
			example, exists := yamlExamples[name]
			if exists {
				formattedExample, err = marshalYAML(example)
			}
		}

		if err != nil {
			panic(fmt.Errorf("failed to parse: %w", err))
		}

		var buffer bytes.Buffer
		fmt.Fprintf(&buffer, "<!-- %s -->\n\n```%s\n", name, format)
		buffer.Write(formattedExample)
		buffer.WriteString("\n```")

		return buffer.Bytes()
	})

	return updatedReadme
}

func updateDefaultConfigSection(readme []byte) []byte {
	configRe := regexp.MustCompile("<!--\\s*DefaultConfig\\s*-->\\s*```yaml[\\s\\S]*?```")

	configContent, err := os.ReadFile("build/package/config/aerospike-backup-service.yml")
	if err != nil {
		panic(fmt.Errorf("failed to read config YAML: %w", err))
	}

	config, err := dto.NewConfigFromReader(bytes.NewReader(configContent), decoder.YAML)
	if err != nil {
		panic(fmt.Errorf("failed to parse default config YAML: %w", err))
	}
	if err = config.Validate(); err != nil {
		panic(fmt.Errorf("failed to validate default config YAML: %w", err))
	}

	return configRe.ReplaceAllFunc(readme, func(_ []byte) []byte {
		var buffer bytes.Buffer
		buffer.WriteString("<!-- DefaultConfig -->\n\n```yaml\n")
		buffer.Write(configContent)
		buffer.WriteString("\n```")
		return buffer.Bytes()
	})
}

// MarshalYAML marshals the input into YAML and replaces 4-space indents with 2-space indents.
// we need this to be in sync with Goland's markdown formatter.
func marshalYAML(v any) ([]byte, error) {
	rawYAML, err := yaml.Marshal(v)
	if err != nil {
		return nil, err
	}

	formattedYAML := strings.ReplaceAll(string(rawYAML), "    ", "  ")

	return []byte(formattedYAML), nil
}

type MetricRow struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Labels      []string `json:"labels"`
	Deprecated  bool     `json:"deprecated"`
}

// updateMetrics generates a Markdown table from a list of Prometheus collectors
// and replaces a placeholder section in a given README file.
func updateMetrics(readme []byte) []byte {
	rows := extractRows()
	writeMetricsToFile(rows)

	maxName := len("Name")
	maxType := len("Type")
	maxHelp := len("Description")
	maxLabels := len("Labels")
	for _, r := range rows {
		if len(r.Name) > maxName {
			maxName = len(r.Name)
		}
		if len(r.Type) > maxType {
			maxType = len(r.Type)
		}
		if len(r.Description) > maxHelp {
			maxHelp = len(r.Description)
		}
		labelsStr := strings.Join(r.Labels, ", ")
		if len(labelsStr) > maxLabels {
			maxLabels = len(labelsStr)
		}
	}

	// Adding 2 for the backticks `` around the name
	const quotes = 2

	var sb strings.Builder
	// Header
	fmt.Fprintf(&sb, "| %-*s | %-*s | %-*s | %-*s |\n",
		maxName+quotes, "Name", maxType, "Type", maxHelp, "Description", maxLabels, "Labels")
	// Separator
	fmt.Fprintf(&sb, "|-%s-|-%s-|-%s-|-%s-|\n",
		strings.Repeat("-", maxName+quotes),
		strings.Repeat("-", maxType),
		strings.Repeat("-", maxHelp),
		strings.Repeat("-", maxLabels))
	// Body
	for _, r := range rows {
		name := "`" + r.Name + "`"
		labelsStr := strings.Join(r.Labels, ", ")
		fmt.Fprintf(&sb, "| %-*s | %-*s | %-*s | %-*s |\n",
			maxName+quotes, name, maxType, r.Type, maxHelp, r.Description, maxLabels, labelsStr)
	}
	table := sb.String()

	// Replace section after <!-- Metrics -->
	metricsRe := regexp.MustCompile(`(?s)(<!-- Metrics -->\n\n)(\|.*?\|\n)(\n)`)

	return metricsRe.ReplaceAll(readme, []byte("${1}"+table+"${3}"))
}

func extractRows() []MetricRow {
	var rows []MetricRow
	// This regex extracts the name, help text, and variable labels from the
	// description string of a Prometheus metric.
	prometheusRE := regexp.MustCompile(
		`Desc{fqName:\s*"([^"]+)",\s*help:\s*"([^"]+)",\s*constLabels:\s*{[^}]*},\s*variableLabels:\s*{([^}]*)}}`)

	// Iterate over all registered metrics.
	for _, metric := range metrics.AllMetrics {
		ch := make(chan *prometheus.Desc, 1)
		metric.Describe(ch)
		close(ch)
		for desc := range ch {
			str := desc.String()
			matches := prometheusRE.FindStringSubmatch(str)
			if len(matches) != 4 {
				panic("Failed to match Prometheus description: " + str)
			}
			helpText := matches[2]
			deprecated := strings.Contains(helpText, "(Deprecated")

			var labels []string
			if matches[3] != "" {
				labels = strings.Split(matches[3], ",")
			}
			rows = append(rows, MetricRow{matches[1], metricsType(metric), helpText, labels, deprecated})
		}
	}

	// Sort rows to have non-deprecated metrics on top.
	slices.SortFunc(rows, func(a, b MetricRow) int {
		if a.Deprecated != b.Deprecated {
			if a.Deprecated {
				return 1
			}

			return -1
		}

		return cmp.Compare(a.Name, b.Name)
	})

	return rows
}

func writeMetricsToFile(rows []MetricRow) {
	// write metrics to json file
	jsonBytes, err := json.MarshalIndent(rows, "", "  ")
	if err != nil {
		panic(err)
	}
	err = os.WriteFile("docs/metrics.json", jsonBytes, 0600)
	if err != nil {
		panic(err)
	}
}

func metricsType(metric prometheus.Collector) string {
	switch metric.(type) {
	case *prometheus.CounterVec:
		return "Counter"
	case *prometheus.GaugeVec:
		return "Gauge"
	case *prometheus.HistogramVec:
		return "Histogram"
	case *prometheus.SummaryVec:
		return "Summary"
	default:
		// Readme generator only; panic is acceptable if an unknown metric type is registered.
		panic(fmt.Sprintf("Unknown metric type %v", metric))
	}
}
