package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto/decoder"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util"
	"github.com/prometheus/client_golang/prometheus"
	"gopkg.in/yaml.v3"
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
		User:     util.Ptr("user"),
		Password: util.Ptr("password"),
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
			Namespaces:    []string{"test-namespace"},
		},
		"routine2": {
			BackupPolicy:     "removeFilesPolicy",
			SourceCluster:    "absDefaultCluster",
			Storage:          "local",
			IntervalCron:     "@monthly",
			IncrIntervalCron: "@daily",
			Namespaces:       []string{"test-namespace"},
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
			DurationSec:         300,
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
			NoGeneration: util.Ptr(true),
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
			EstimatedEndTime: util.Ptr(time.Date(2024, 01, 01, 13, 0, 0, 0, time.UTC)),
			Metrics: dto.Metrics{
				RecordsPerSecond:   1000,
				KilobytesPerSecond: 30000,
				Pipeline:           0,
			},
		},
	},
	"CurrentRestoreResponse": dto.RestoreJobStatus{
		ReadRecords:     100_000,
		TotalBytes:      30000000,
		ExpiredRecords:  0,
		SkippedRecords:  0,
		IgnoredRecords:  0,
		InsertedRecords: 5_000,
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
			EstimatedEndTime: util.Ptr(time.Date(2024, 01, 01, 13, 0, 0, 0, time.UTC)),
			Metrics: dto.Metrics{
				RecordsPerSecond:   1000,
				KilobytesPerSecond: 30000,
				Pipeline:           0,
			},
		},
		Status: dto.JobStatusRunning,
		Error:  "",
	},
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
	readme, err := os.ReadFile("README.md")
	if err != nil {
		panic(err)
	}

	updatedReadme := updateDtoExamples(readme)
	updatedReadme = updateDefaultConfigSection(updatedReadme)
	updatedReadme = updateMetrics(updatedReadme)
	updatedReadme = updateDtoDescription(updatedReadme)

	err = os.WriteFile("README.md", updatedReadme, 0600)
	if err != nil {
		panic(err)
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
		buffer.WriteString(fmt.Sprintf("<!-- %s -->\n\n```%s\n", name, format))
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

	_, err = dto.NewConfigFromReader(bytes.NewReader(configContent), decoder.YAML)
	if err != nil {
		panic(fmt.Errorf("failed to parse default config YAML: %w", err))
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

type Row struct {
	Name string
	Help string
}

func updateMetrics(readme []byte) []byte {
	var rows []Row

	prometheusRE := regexp.MustCompile(`fqName: "([^"]+)", help: "([^"]+)"`)
	for _, metric := range service.AllMetrics {
		ch := make(chan *prometheus.Desc, 1)
		metric.Describe(ch)
		close(ch)
		for desc := range ch {
			matches := prometheusRE.FindStringSubmatch(desc.String())
			if len(matches) == 3 {
				rows = append(rows, Row{matches[1], matches[2]})
			}
		}
	}

	// Determine column widths
	maxName := len("Name")
	maxHelp := len("Description")
	for _, r := range rows {
		if len(r.Name) > maxName {
			maxName = len(r.Name)
		}
		if len(r.Help) > maxHelp {
			maxHelp = len(r.Help)
		}
	}

	const quotes = 2

	// Build Markdown table string
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("| %-*s | %-*s |\n", maxName+quotes, "Name", maxHelp, "Description"))
	sb.WriteString(fmt.Sprintf("|-%s-|-%s-|\n", strings.Repeat("-", maxName+quotes), strings.Repeat("-", maxHelp)))
	for _, r := range rows {
		name := "`" + r.Name + "`"
		sb.WriteString(fmt.Sprintf("| %-*s | %-*s |\n", maxName+quotes, name, maxHelp, r.Help))
	}
	table := sb.String()

	// Replace section after <!-- Metrics -->
	metricsRe := regexp.MustCompile(`(?s)(<!-- Metrics -->\n\n)(\|.*?\|\n)(\n)`)

	return metricsRe.ReplaceAll(readme, []byte("${1}"+table+"${3}"))
}

const openapi = "docs/openapi.json"

var schemas = readSchemas()

func updateDtoDescription(readme []byte) []byte {
	re := regexp.MustCompile(`<!--\s*table\s+([\w.]+)\s*-->\s*\n+(?:(\|(?:.*\|.*\n)+?)\n)?`)

	updatedReadme := re.ReplaceAllFunc(readme, func(match []byte) []byte {
		submatches := re.FindSubmatch(match)
		if len(submatches) < 2 {
			return match // not enough groups
		}
		name := string(submatches[1]) // group 1: table name
		newTable := []byte(generateMarkdownTable(name))

		// submatches[2] = old table (may be nil)
		// Replace old table (group 2) with newTable
		if len(submatches) >= 3 && len(submatches[2]) > 0 {
			prefix := bytes.Split(match, submatches[2])[0]
			return append(append(bytes.TrimRight(prefix, "\n"), []byte("\n\n")...), append(newTable, '\n')...)
		}

		// No table previously → just append new one
		return append(bytes.TrimRight(match, "\n"), append([]byte("\n\n"), append(newTable, '\n')...)...)
	})

	return updatedReadme

}

func generateMarkdownTable(dtoName string) string {
	schema, ok := schemas[dtoName]
	if !ok {
		panic(fmt.Errorf("schema %q not found", dtoName))
	}

	// Flatten properties recursively
	rows := dtoToRows(schema)

	// Determine column widths
	maxName := len("Field")
	maxHelp := len("Description")
	for _, r := range rows {
		if len(r.Name) > maxName {
			maxName = len(r.Name)
		}
		for _, line := range strings.Split(r.Help, "\n") {
			if len(line) > maxHelp {
				maxHelp = len(line)
			}
		}
	}

	const quotes = 2
	var sb strings.Builder

	// Write header
	sb.WriteString(fmt.Sprintf("| %-*s | %-*s |\n", maxName+quotes, "Field", maxHelp, "Description"))
	sb.WriteString(fmt.Sprintf("|-%s-|-%s-|\n", strings.Repeat("-", maxName+quotes), strings.Repeat("-", maxHelp)))

	// Write rows
	for _, r := range rows {
		name := "`" + r.Name + "`"
		desc := strings.ReplaceAll(r.Help, "\n", "<br>")
		sb.WriteString(fmt.Sprintf("| %-*s | %-*s |\n", maxName+quotes, name, maxHelp, desc))
	}

	return sb.String()
}

func readSchemas() map[string]Schema {
	data, err := os.ReadFile(openapi)
	if err != nil {
		panic(fmt.Errorf("failed to read file: %w", err))
	}

	var api OpenAPI
	if err := json.Unmarshal(data, &api); err != nil {
		panic(fmt.Errorf("failed to unmarshal JSON: %w", err))
	}

	return api.Components.Schemas
}

type OpenAPI struct {
	Components Components `json:"components"`
}

type Components struct {
	Schemas map[string]Schema `json:"schemas"`
}

type Schema struct {
	Type       string              `json:"type"`
	Properties map[string]Property `json:"properties"`
}

type Property struct {
	Description string      `json:"description,omitempty"`
	Type        string      `json:"type,omitempty"`
	AllOf       []Reference `json:"allOf"`
	Items       Reference   `json:"items"`
}

type Reference struct {
	Ref string `json:"$ref,omitempty"`
}

func dtoToRows(input Schema) []Row {
	var rows []Row
	collectFields(input, "", &rows)
	sort.SliceStable(rows, func(i, j int) bool {
		depthI := strings.Count(rows[i].Name, ".")
		depthJ := strings.Count(rows[j].Name, ".")
		if depthI != depthJ {
			return depthI < depthJ
		}

		return strings.Compare(rows[i].Name, rows[j].Name) < 0
	})

	return rows
}

func collectFields(schema Schema, prefix string, out *[]Row) {
	for fieldName, prop := range schema.Properties {
		fullName := fieldName
		if prefix != "" {
			fullName = prefix + "." + fieldName
		}

		if prop.Type == "object" {
			// Check if it's a reference via allOf
			for _, ref := range prop.AllOf {
				if ref.Ref != "" {
					refName := extractRefName(ref.Ref)
					if refSchema, ok := schemas[refName]; ok {
						collectFields(refSchema, fullName, out)
					}
				}
			}
			continue // Don't print the top-level object
		}

		// Normal field
		*out = append(*out, Row{
			Name: fullName,
			Help: strings.ReplaceAll(prop.Description, "\n", "<br>"),
		})
	}
}

// Example: "#/components/schemas/dto.RunningJob" → "dto.RunningJob"
func extractRefName(ref string) string {
	parts := strings.Split(ref, "/")
	return parts[len(parts)-1]
}
