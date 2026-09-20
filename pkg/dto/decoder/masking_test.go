package decoder

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"reflect"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/redact"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRedactSecrets_ComplexFixture(t *testing.T) {
	original := testComplexConfig()
	literals := literalSecretsInConfig()
	nonSecrets := nonSecretValuesInConfig()

	t.Run("marshal json with redact", func(t *testing.T) {
		data, err := Marshal(&original, JSON, true)
		require.NoError(t, err)

		assertRedactedOutput(t, string(data), literals, nonSecrets)
	})

	t.Run("marshal yaml with redact", func(t *testing.T) {
		data, err := Marshal(&original, YAML, true)
		require.NoError(t, err)

		assertRedactedOutput(t, string(data), literals, nonSecrets)
	})

	t.Run("marshal json without redact preserves secrets", func(t *testing.T) {
		data, err := Marshal(&original, JSON, false)
		require.NoError(t, err)

		output := string(data)
		for _, literal := range literals {
			assert.Contains(t, output, literal)
		}
	})

	t.Run("redact secrets value", func(t *testing.T) {
		redacted := RedactSecrets(original)

		data, err := Marshal(&redacted, JSON, false)
		require.NoError(t, err)

		assertRedactedOutput(t, string(data), literals, nonSecrets)
	})

	t.Run("slog replace attr", func(t *testing.T) {
		var buf bytes.Buffer
		logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{
			ReplaceAttr: RedactSecretsReplaceAttr(),
		}))

		logger.Info("config", slog.Any("config", original))

		assertRedactedOutput(t, buf.String(), literals, nonSecrets)
	})

	t.Run("does not mutate original", func(t *testing.T) {
		before := original
		_ = RedactSecrets(original)

		assert.Equal(t, before, original)
		assert.Equal(t, redact.Secret(literalPassword), before.AerospikeClusters["cluster1"].Credentials.Password)
		assert.Equal(t, redact.Secret(literalAccessKey), before.StorageProviders["s3-main"].AccessKeyID)
		assert.Equal(t, redact.Secret(literalPassword), before.Routines[0].Keys[0])
	})

	t.Run("preserves time values", func(t *testing.T) {
		created := original.BackupHistory["routine1"][0].Created
		finished := original.BackupHistory["routine1"][0].Finished

		redacted := RedactSecrets(original)
		entry := redacted.BackupHistory["routine1"][0]

		assert.Equal(t, created, entry.Created)
		assert.Equal(t, finished, entry.Finished)
		assert.Equal(t, int64(1000), entry.Timestamp)
	})

	t.Run("preserves empty secret", func(t *testing.T) {
		redacted := RedactSecrets(original)

		assert.Empty(t, redacted.AerospikeClusters["cluster3"].Credentials.Password)
		assert.Empty(t, redacted.StorageProviders["s3-ref"].SecretAccessKey)

		clusterData, err := Marshal(redacted.AerospikeClusters["cluster3"], JSON, false)
		require.NoError(t, err)
		clusterOutput := string(clusterData)
		assert.Contains(t, clusterOutput, `"password":""`)
		assert.NotContains(t, clusterOutput, `"password":"`+redact.Placeholder+`"`)

		storageData, err := Marshal(redacted.StorageProviders["s3-ref"], JSON, false)
		require.NoError(t, err)
		storageOutput := string(storageData)
		assert.Contains(t, storageOutput, `"secret-access-key":""`)
		assert.NotContains(t, storageOutput, `"secret-access-key":"`+redact.Placeholder+`"`)
	})

	t.Run("preserves valid secret ref", func(t *testing.T) {
		redacted := RedactSecrets(original)

		assert.Equal(t, redact.Secret(validSecretRef), redacted.AerospikeClusters["cluster2"].Credentials.Password)
		assert.Equal(t, redact.Secret(validSecretRef), redacted.StorageProviders["s3-ref"].AccessKeyID)
		assert.Equal(t, redact.Secret(validSecretRef), redacted.Routines[0].Keys[1])
	})

	t.Run("redacts malformed secret ref", func(t *testing.T) {
		redacted := RedactSecrets(original)

		assert.Equal(t, redact.Secret(redact.Placeholder), redacted.AerospikeClusters["cluster2"].Encryption.KeySecret)

		data, err := Marshal(&redacted, JSON, false)
		require.NoError(t, err)
		assert.Contains(t, string(data), `"key-secret":"`+redact.Placeholder+`"`)
		assert.NotContains(t, string(data), malformedSecretRef)
	})

	t.Run("nil input", func(t *testing.T) {
		assert.Nil(t, RedactSecrets[*testConfig](nil))
	})

	t.Run("nil pointer fields", func(t *testing.T) {
		redacted := RedactSecrets(original)

		assert.Nil(t, redacted.AerospikeClusters["cluster2"].TLS)
	})
}

func TestPasswordMasking(t *testing.T) {
	t.Run("without redaction", func(t *testing.T) {
		var buf bytes.Buffer
		logger := slog.New(slog.NewJSONHandler(&buf, nil))
		logger.Info("cluster credentials", slog.Any("credentials", testCreds))

		assert.Contains(t, buf.String(), literalPassword)
	})

	t.Run("with redaction", func(t *testing.T) {
		var buf bytes.Buffer
		logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{
			ReplaceAttr: RedactSecretsReplaceAttr(),
		}))

		logger.Info("cluster credentials", slog.Any("credentials", testConfigWithCreds(testCreds)))

		output := buf.String()
		assert.Contains(t, output, `"user":"testUser"`)
		assert.NotContains(t, output, literalPassword)
	})
}

func assertRedactedOutput(t *testing.T, output string, literals, nonSecrets []string) {
	t.Helper()

	for _, literal := range literals {
		assert.NotContains(t, output, literal, "literal secret leaked")
	}

	for _, value := range nonSecrets {
		assert.Contains(t, output, value)
	}

	assert.Contains(t, output, redact.Placeholder)
}

func TestRedactSecrets_PreservesTime(t *testing.T) {
	created := time.UnixMilli(1000).UTC()
	finished := time.UnixMilli(5000).UTC()

	original := map[string][]testBackupDetails{
		"routine1": {
			{
				Key:       "backup1",
				Created:   created,
				Timestamp: 1000,
				Finished:  finished,
			},
		},
	}

	redacted := RedactSecrets(original)
	require.Len(t, redacted["routine1"], 1)
	assert.Equal(t, created, redacted["routine1"][0].Created)
	assert.Equal(t, finished, redacted["routine1"][0].Finished)
	assert.Equal(t, int64(1000), redacted["routine1"][0].Timestamp)
}

func TestRedactSecrets_UnexportedTimePointerFields(t *testing.T) {
	// Mirrors model.BackupTime: unexported *time.Time fields panic when redactValue
	// tries to deep-copy through reflect without skipping *time.Time.
	type backupTimeLike struct {
		full *time.Time
	}

	now := time.Now()
	backupTime := &backupTimeLike{full: &now}

	assert.NotPanics(t, func() {
		RedactSecrets(backupTime)
	})
}

func TestRedactSecrets_UnexportedLocationPointerFields(t *testing.T) {
	// Mirrors model.Location: unexported *time.Location field.
	type locationLike struct {
		resolved   *time.Location
		Configured string
	}

	loc, err := time.LoadLocation("America/New_York")
	require.NoError(t, err)

	location := locationLike{resolved: loc, Configured: "America/New_York"}

	assert.NotPanics(t, func() {
		RedactSecrets(location)
	})
}

// credentialError mirrors aerospike.AerospikeError: exported fields on a type
// stored behind fmt.Errorf("%w"). slog.Any("error", err) walks that wrapError,
// whose err field is unexported, then Set panics when copying ResultCode.
type credentialError struct {
	ResultCode int
	message    string
}

func (e *credentialError) Error() string {
	return e.message
}

func TestRedactSecrets_WrappedError(t *testing.T) {
	err := fmt.Errorf("failed to connect to cluster: %w", &credentialError{
		ResultCode: 65,
		message:    "Invalid credential",
	})

	assert.Equal(t, err, RedactSecrets(err))
}

// RedactSecrets returns errors untouched, so a secret interpolated into an error
// message can only be masked by Secret's Stringer at construction time.
func TestSecret_MaskedInErrorMessages(t *testing.T) {
	secret := redact.Secret(literalPassword)

	tests := []struct {
		name string
		err  error
	}{
		{
			name: "%s verb",
			err:  fmt.Errorf("login failed for password %s", secret),
		},
		{
			name: "%v verb",
			err:  fmt.Errorf("login failed for password %v", secret),
		},
		{
			name: "wrapped with %w",
			err:  fmt.Errorf("connect cluster1: %w", fmt.Errorf("bad password %s", secret)),
		},
		{
			name: "joined errors",
			err:  errors.Join(fmt.Errorf("cluster1: %v", secret), fmt.Errorf("cluster2: %v", secret)),
		},
		{
			name: "secret nested in struct",
			err:  fmt.Errorf("invalid credentials %v", testCreds),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Contains(t, tt.err.Error(), redact.Placeholder)
			assert.NotContains(t, tt.err.Error(), literalPassword)

			assert.NotContains(t, RedactSecrets(tt.err).Error(), literalPassword)
		})
	}

	t.Run("secret ref preserved", func(t *testing.T) {
		err := fmt.Errorf("cannot resolve %v", redact.Secret(validSecretRef))
		assert.Contains(t, err.Error(), validSecretRef)
	})

	t.Run("logged error attribute is masked", func(t *testing.T) {
		var buf bytes.Buffer
		logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{
			ReplaceAttr: RedactSecretsReplaceAttr(),
		}))

		logger.Error("connect failed", slog.Any("error", fmt.Errorf("bad password %s", secret)))

		assert.Contains(t, buf.String(), redact.Placeholder)
		assert.NotContains(t, buf.String(), literalPassword)
	})

	t.Run("%q verb", func(t *testing.T) {
		err := fmt.Errorf("password %q", secret)
		assert.Contains(t, err.Error(), redact.Placeholder)
		assert.NotContains(t, err.Error(), literalPassword)
	})

	// Guards the one sharp edge: converting to string bypasses the Stringer, and
	// RedactSecrets cannot repair an error message after the fact.
	t.Run("raw string conversion leaks", func(t *testing.T) {
		err := fmt.Errorf("password %s", string(secret))
		assert.Contains(t, err.Error(), literalPassword)
		assert.Equal(t, err.Error(), RedactSecrets(err).Error())
	})
}

type selfPointer struct {
	Name   string
	Secret redact.Secret
	Peer   *selfPointer
}

type selfInterface struct {
	Secret  redact.Secret
	Payload any
}

type noSecret struct {
	Name    string
	Payload any
}

func TestRedactSecrets_CyclicStructures(t *testing.T) {
	placeholder := redact.Secret(redact.Placeholder)

	t.Run("cycle through a typed pointer", func(t *testing.T) {
		v := &selfPointer{Name: "a", Secret: literalPassword}
		v.Peer = v

		res := RedactSecrets(v)

		assert.NotSame(t, v, res, "a value holding a secret is rebuilt")
		assert.Equal(t, placeholder, res.Secret)
		assert.Equal(t, "a", res.Name)
		assert.Same(t, res, res.Peer, "the copy closes the cycle on itself")
		assert.Equal(t, redact.Secret(literalPassword), v.Secret, "the original is untouched")
	})

	t.Run("cycle through an interface field", func(t *testing.T) {
		v := &selfInterface{Secret: literalPassword}
		v.Payload = v

		res := RedactSecrets(v)

		assert.NotSame(t, v, res)
		assert.Equal(t, placeholder, res.Secret)
		assert.Same(t, res, res.Payload)
	})

	t.Run("cycle with no secret is returned as it stands", func(t *testing.T) {
		v := &noSecret{Name: "a"}
		v.Payload = v

		assert.Same(t, v, RedactSecrets(v))
	})

	t.Run("map containing itself", func(t *testing.T) {
		v := map[string]any{"secret": redact.Secret(literalPassword)}
		v["self"] = v

		res := RedactSecrets(v)

		assert.Equal(t, placeholder, res["secret"])
		assert.Equal(t, reflect.ValueOf(res).Pointer(), reflect.ValueOf(res["self"]).Pointer())
	})

	t.Run("slice containing itself", func(t *testing.T) {
		v := make([]any, 2)
		v[0] = redact.Secret(literalPassword)
		v[1] = v

		res := RedactSecrets(v)

		assert.Equal(t, placeholder, res[0])
		assert.Equal(t, reflect.ValueOf(res).Pointer(), reflect.ValueOf(res[1]).Pointer())
	})

	t.Run("two-node cycle", func(t *testing.T) {
		a := &selfPointer{Name: "a", Secret: literalPassword}
		b := &selfPointer{Name: "b", Peer: a}
		a.Peer = b

		res := RedactSecrets(a)

		assert.Equal(t, placeholder, res.Secret)
		assert.Equal(t, "b", res.Peer.Name)
		assert.Same(t, res, res.Peer.Peer)
	})
}

// A pointer reachable by two paths is copied once, so the copy has the shape of the original.
func TestRedactSecrets_SharedPointerIsCopiedOnce(t *testing.T) {
	shared := &selfPointer{Name: "shared", Secret: literalPassword}
	v := struct{ Left, Right *selfPointer }{Left: shared, Right: shared}

	res := RedactSecrets(v)

	assert.Same(t, res.Left, res.Right)
	assert.Equal(t, redact.Secret(redact.Placeholder), res.Left.Secret)
}

// Two slices over one backing array share a data pointer but are different values, so each
// must be walked and copied on its own: neither the shorter view's length nor its verdict on
// holding a secret may stand in for the longer one's.
func TestRedactSecrets_SlicesOverOneArray(t *testing.T) {
	t.Run("each view keeps its own length", func(t *testing.T) {
		full := []redact.Secret{"s0", "s1", "s2"}
		v := struct{ Head, Full []redact.Secret }{Head: full[:1], Full: full}

		res := RedactSecrets(v)

		require.Len(t, res.Head, 1)
		require.Len(t, res.Full, 3)
		for _, s := range res.Full {
			assert.Equal(t, redact.Secret(redact.Placeholder), s)
		}
	})

	t.Run("secret beyond the shorter view is still redacted", func(t *testing.T) {
		type entry struct{ Payload any }
		full := []entry{{Payload: "harmless"}, {Payload: redact.Secret(literalPassword)}}
		v := struct{ Head, Full []entry }{Head: full[:1], Full: full}

		res := RedactSecrets(v)

		require.Len(t, res.Full, 2)
		assert.Equal(t, redact.Secret(redact.Placeholder), res.Full[1].Payload)
	})
}

// requestCarryingError stands for an SDK error that keeps the failed request: through the
// stdlib's own Request.Response / Response.Request back-pointers the value graph is cyclic.
type requestCarryingError struct {
	Op      string
	Request *http.Request
	Secret  redact.Secret
}

func (e *requestCarryingError) Error() string { return e.Op + " failed" }

func TestRedactSecrets_ErrorWithRequestResponseCycle(t *testing.T) {
	req := &http.Request{
		Method: http.MethodGet,
		URL:    &url.URL{Scheme: "https", Host: "my-bucket.s3.amazonaws.com", Path: "/backup.tar.gz"},
		Header: http.Header{"X-Request-Id": []string{"abc"}},
	}
	req.Response = &http.Response{StatusCode: http.StatusForbidden, Request: req}

	err := &requestCarryingError{Op: "GetObject", Request: req, Secret: literalPassword}

	redacted := RedactSecrets(err)

	assert.Equal(t, redact.Secret(redact.Placeholder), redacted.Secret)
	assert.Equal(t, err.Error(), redacted.Error())
	assert.Equal(t, "/backup.tar.gz", redacted.Request.URL.Path)
	assert.Equal(t, http.StatusForbidden, redacted.Request.Response.StatusCode)
	assert.Same(t, redacted.Request, redacted.Request.Response.Request, "the stdlib cycle is preserved in the copy")
	assert.Equal(t, redact.Secret(literalPassword), err.Secret, "the original is untouched")
}

// credentialPair is a Redactable that is not a string: it redacts itself as a unit, and the
// walk stores what it hands back without knowing how it is represented.
type credentialPair struct {
	User     string
	Password string
}

func (c credentialPair) DisplayString() string { return c.User + ":" + redact.Placeholder }
func (c credentialPair) IsRedacted() bool      { return c.Password == redact.Placeholder }
func (c credentialPair) Redacted() redact.Redactable {
	return credentialPair{User: c.User, Password: redact.Placeholder}
}

func TestRedactSecrets_RedactableDecidesItsOwnForm(t *testing.T) {
	type holder struct {
		Pair    credentialPair
		PairPtr *credentialPair
		Pairs   map[string]credentialPair
	}

	v := holder{
		Pair:    credentialPair{User: "admin", Password: literalPassword},
		PairPtr: &credentialPair{User: "root", Password: literalPassword},
		Pairs:   map[string]credentialPair{"c1": {User: "c1", Password: literalPassword}},
	}

	res := RedactSecrets(v)

	assert.Equal(t, credentialPair{User: "admin", Password: redact.Placeholder}, res.Pair)
	assert.Equal(t, credentialPair{User: "root", Password: redact.Placeholder}, *res.PairPtr,
		"the pointer is followed, not redacted")
	assert.Equal(t, credentialPair{User: "c1", Password: redact.Placeholder}, res.Pairs["c1"])
	assert.Equal(t, literalPassword, v.Pair.Password, "the original is untouched")
}
