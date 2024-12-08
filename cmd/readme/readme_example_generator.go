package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/aerospike/aerospike-backup-service/v2/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v2/pkg/util"
	"os"
	"regexp"
)

var jsonExamples = map[string]any{
	"ClustersResponse": []dto.AerospikeCluster{
		{
			SeedNodes: []dto.SeedNode{
				{HostName: "host.docker.internal", Port: 3000},
			},
			Credentials: &dto.Credentials{
				User:     util.Ptr("user"),
				Password: util.Ptr("password"),
			},
		},
	},
}

func main() {
	_ = dto.AerospikeCluster{}
	readme, err := os.ReadFile("/Users/ak/work/aerospike-backup-service/README.md")
	if err != nil {
		panic(err)
	}

	re := regexp.MustCompile("<!--\\s*(\\w+)\\s*-->\\s*```json[\\s\\S]*?```")
	updatedReadme := re.ReplaceAllFunc(readme, func(match []byte) []byte {
		submatches := re.FindSubmatch(match)
		if len(submatches) < 2 {
			return match
		}

		name := string(submatches[1])
		example, exists := jsonExamples[name]
		if !exists {
			return match
		}

		formattedJSON, err := json.MarshalIndent(example, "", "  ")
		if err != nil {
			panic(err)
		}

		var buffer bytes.Buffer
		buffer.WriteString(fmt.Sprintf("<!-- %s -->\n```json\n", name))
		buffer.Write(formattedJSON)
		buffer.WriteString("\n```")
		return buffer.Bytes()
	})

	err = os.WriteFile("README.md", updatedReadme, 0644)
	if err != nil {
		panic(err)
	}
}
