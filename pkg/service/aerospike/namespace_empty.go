package aerospike

import (
	"fmt"
	"strconv"
	"strings"

	as "github.com/aerospike/aerospike-client-go/v7"
	"github.com/aerospike/backup-go"
)

// IsNamespaceEmpty checks if the given namespace or specific sets within it are empty.
// If sets slice is empty, it checks the entire namespace.
// If sets are provided, it checks only those specific sets.
func IsNamespaceEmpty(client backup.AerospikeClient, namespace string, sets []string) (bool, error) {
	node, err := client.Cluster().GetRandomNode()
	if err != nil {
		return false, fmt.Errorf("failed to get node: %w", err)
	}

	if len(sets) == 0 {
		return isEntireNamespaceEmpty(node, namespace)
	}

	return areSetsEmpty(node, namespace, sets)
}

// isEntireNamespaceEmpty checks if the entire namespace has any records
func isEntireNamespaceEmpty(node *as.Node, namespace string) (bool, error) {
	statsKey := "namespace/" + namespace
	stats, err := getNodeStats(node, statsKey)
	if err != nil {
		return false, err
	}

	objectCount, err := parseObjectCount(stats, ";", "objects")
	if err != nil {
		return false, fmt.Errorf("failed to get object count for namespace %s: %w", namespace, err)
	}

	return objectCount == 0, nil
}

// areSetsEmpty checks if all specified sets in the namespace are empty
func areSetsEmpty(node *as.Node, namespace string, sets []string) (bool, error) {
	for _, set := range sets {
		isEmpty, err := isSetEmpty(node, namespace, set)
		if err != nil {
			return false, err
		}
		if !isEmpty {
			return false, nil
		}
	}
	return true, nil
}

// isSetEmpty checks if a specific set is empty
func isSetEmpty(node *as.Node, namespace, set string) (bool, error) {
	statsKey := fmt.Sprintf("sets/%s/%s", namespace, set)
	stats, err := getNodeStats(node, statsKey)
	if err != nil {
		return false, err
	}

	// Empty stats means set doesn't exist (treat as empty)
	if stats == "" {
		return true, nil
	}

	objectCount, err := parseObjectCount(stats, ":", "objects")
	if err != nil {
		return false, fmt.Errorf("failed to get object count for set %s: %w", set, err)
	}

	return objectCount == 0, nil
}

// getNodeStats retrieves statistics from a node for a given key
func getNodeStats(node *as.Node, statsKey string) (string, error) {
	infoRes, err := node.RequestInfo(&as.InfoPolicy{}, statsKey)
	if err != nil {
		return "", fmt.Errorf("failed to get stats for %s: %w", statsKey, err)
	}

	return infoRes[statsKey], nil
}

// parseObjectCount parses the object count from stats string, trying multiple possible key names
func parseObjectCount(stats, delimiter, key string) (int64, error) {
	statsPairs := strings.Split(stats, delimiter)
	for _, pair := range statsPairs {
		kv := strings.Split(pair, "=")
		if len(kv) != 2 {
			continue
		}

		if kv[0] == key {
			return strconv.ParseInt(kv[1], 10, 64)
		}
	}
	return 0, fmt.Errorf("object count not found in stats")
}
