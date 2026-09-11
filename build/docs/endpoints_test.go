package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func testRenderers() map[string]renderer {
	endpoints := testEndpoints()
	renderers := make(map[string]renderer)

	for id := range endpoints {
		renderers[id] = func(args string) string { return renderEndpoint(id, args, endpoints) }
	}

	return renderers
}

func testEndpoints() map[string]endpoint {
	return map[string]endpoint{
		"disableRoutine": {method: "PUT", path: "/v1/config/routines/{name}/disable", tag: "Configuration"},
		"getFullBackups": {method: "GET", path: "/v1/backups/full", tag: "Backup"},
		"restoreFull":    {method: "POST", path: "/v1/restore/full", tag: "Restore"},
	}
}

func TestEndpointMarker_RendersInlineSpan(t *testing.T) {
	input := []byte("A client calls <!-- tag restoreFull --><!-- /tag --> to begin.\n")

	got := string(applyTags(input, testRenderers()))

	assert.Equal(t,
		"A client calls <!-- tag restoreFull -->`POST /v1/restore/full`<!-- /tag --> to begin.\n",
		got)
}

func TestEndpointMarker_RendersLinkCallout(t *testing.T) {
	input := []byte("<!-- tag disableRoutine link --><!-- /tag -->\n")

	got := string(applyTags(input, testRenderers()))

	assert.Contains(t, got, "<!-- tag disableRoutine link -->\n"+
		"[`PUT {{baseUrl}}/v1/config/routines/{name}/disable`]"+
		"(https://aerospike.github.io/aerospike-backup-service/#/Configuration/disableRoutine)\n"+
		"<!-- /tag -->")
}

// TestEndpointMarker_ReplacesStaleContent is the case the marker exists for: the
// method and path drifted from the router, and the generator corrects them.
func TestEndpointMarker_ReplacesStaleContent(t *testing.T) {
	input := []byte("<!-- tag disableRoutine link -->\n" +
		"[`POST {{baseUrl}}/v1/routines/<routineName>/disable/`]" +
		"(https://aerospike.github.io/aerospike-backup-service/#/Configuration/disableRoutine)\n" +
		"<!-- /tag -->\n")

	got := string(applyTags(input, testRenderers()))

	assert.Contains(t, got, "PUT {{baseUrl}}/v1/config/routines/{name}/disable")
	assert.NotContains(t, got, "/v1/routines/")
}

// TestEndpointMarker_KeepsQuery checks that the query string stays with the
// author: which parameters an example demonstrates is a teaching decision.
func TestEndpointMarker_KeepsQuery(t *testing.T) {
	input := []byte("<!-- tag getFullBackups link ?from=<from>&to=<to> --><!-- /tag -->\n")

	got := string(applyTags(input, testRenderers()))

	assert.Contains(t, got, "<!-- tag getFullBackups link ?from=<from>&to=<to> -->")
	assert.Contains(t, got, "`GET {{baseUrl}}/v1/backups/full?from=<from>&to=<to>`")
}

// TestEndpointMarker_PairsEachRegionSeparately guards the case that motivated
// the closing marker: two mentions in one sentence must not merge into one region.
func TestEndpointMarker_PairsEachRegionSeparately(t *testing.T) {
	input := []byte("Calls <!-- tag restoreFull --><!-- /tag --> " +
		"or <!-- tag getFullBackups --><!-- /tag -->.\n")

	got := string(applyTags(input, testRenderers()))

	assert.Equal(t,
		"Calls <!-- tag restoreFull -->`POST /v1/restore/full`<!-- /tag --> "+
			"or <!-- tag getFullBackups -->`GET /v1/backups/full`<!-- /tag -->.\n",
		got)
}

func TestEndpointMarker_IsIdempotent(t *testing.T) {
	input := []byte("<!-- tag getFullBackups link --><!-- /tag -->\n" +
		"and inline <!-- tag restoreFull --><!-- /tag -->.\n")

	once := applyTags(input, testRenderers())
	twice := applyTags(once, testRenderers())

	assert.Equal(t, string(once), string(twice))
}

func TestEndpointMarker_PanicsOnUnknownOperation(t *testing.T) {
	input := []byte("<!-- tag noSuchOperation --><!-- /tag -->\n")

	assert.Panics(t, func() { applyTags(input, testRenderers()) })
}

func TestEndpointMarker_PanicsOnUnknownArgument(t *testing.T) {
	input := []byte("<!-- tag restoreFull sideways --><!-- /tag -->\n")

	assert.Panics(t, func() { applyTags(input, testRenderers()) })
}

func TestEndpointMarker_LeavesOtherContentAlone(t *testing.T) {
	input := []byte("See [the docs](https://example.com) for `GET /v1/backups/full`.\n")

	got := string(applyTags(input, testRenderers()))

	assert.Equal(t, string(input), got)
}
