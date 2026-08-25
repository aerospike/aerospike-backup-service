package dto

import (
	"crypto/tls"
	"errors"
	"fmt"

	servertls "github.com/aerospike/aerospike-backup-service/v3/internal/server/tlsconfig"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/safepath"
)

var secureServerCipherSuites = func() map[string]bool {
	result := make(map[string]bool)
	for _, suite := range tls.CipherSuites() {
		result[suite.Name] = true
	}
	return result
}()

const clientCAField = "client-ca-file"

// HTTPSServerConfig represents the service's HTTPS server configuration.
// @Description HTTPSServerConfig represents the service's HTTPS server configuration.
//
//nolint:lll
type HTTPSServerConfig struct {
	ListenerConfig    `yaml:",inline"`
	SecretAgentConfig `yaml:",inline"`
	// The port to listen on.
	Port *Port `yaml:"port,omitempty" json:"port,omitempty" default:"8443" example:"8443"`
	// Path to the HTTPS server certificate in PEM format.
	CertFile string `yaml:"cert-file,omitempty" json:"cert-file,omitempty" example:"/path/to/server.pem" extensions:"x-nullable"`
	// Path to the HTTPS server private key in PEM format.
	KeyFile string `yaml:"key-file,omitempty" json:"key-file,omitempty" example:"/path/to/server-key.pem" extensions:"x-nullable"`
	// Passphrase for an encrypted HTTPS server private key.
	// This is sensitive information. Can be a path in secret agent or an actual value.
	// Literal values are redacted as "[secret]" in API responses; secret agent references are returned as-is.
	KeyFilePassword secret `yaml:"key-file-password,omitempty" json:"key-file-password,omitempty" format:"password" extensions:"x-nullable"`
	// Minimum accepted TLS protocol version.
	MinVersion TLSMinVersion `yaml:"min-version,omitempty" json:"min-version,omitempty" default:"1.2" enums:"1.2,1.3"`
	// Allowed TLS cipher suite names. An empty list uses Go's secure defaults.
	CipherSuites []string `yaml:"cipher-suites,omitempty" json:"cipher-suites,omitempty" extensions:"x-nullable"`
	// Path to trusted client CA certificates in PEM format.
	ClientCAFile string `yaml:"client-ca-file,omitempty" json:"client-ca-file,omitempty" example:"/path/to/client-ca.pem" extensions:"x-nullable"`
	// Client certificate authentication mode.
	ClientAuth TLSClientAuth `yaml:"client-auth,omitempty" json:"client-auth,omitempty" default:"none" enums:"none,request,require-and-verify"`
}

// Validate validates the HTTPS server configuration.
func (s *HTTPSServerConfig) Validate(opts ValidationOptions) error {
	if s == nil {
		return nil
	}

	if err := s.Port.Validate(); err != nil {
		return err
	}

	if err := s.ListenerConfig.validate(); err != nil {
		return err
	}
	if err := s.validateTLSFields(); err != nil {
		return err
	}

	if err := s.SecretAgentConfig.validate(opts); err != nil {
		return err
	}

	return s.validateTLSConfig(opts)
}

func (s *HTTPSServerConfig) validateTLSConfig(opts ValidationOptions) error {
	if opts.Has(ValidationSkipTLSFiles) {
		return nil
	}
	if s.CertFile == "" || s.KeyFile == "" {
		return nil
	}
	// Secret Agent references are resolved at runtime; they cannot be used to decrypt the key here.
	if err := s.KeyFilePassword.Validate(s.hasSecretAgent()); err != nil {
		return err
	}

	if _, err := servertls.New(s.ToModel()); err != nil {
		return fmt.Errorf("HTTPS TLS %w: %w", errValidation, err)
	}

	return nil
}

func (s *HTTPSServerConfig) validateTLSFields() error {
	if !s.Disabled {
		if s.CertFile == "" {
			return errValidationEmptyField(certField)
		}
		if s.KeyFile == "" {
			return errValidationEmptyField(keyField)
		}
	}
	if s.CertFile != "" && s.KeyFile == "" {
		return errValidationRequires(certField, keyField)
	}
	if s.KeyFile != "" && s.CertFile == "" {
		return errValidationRequires(keyField, certField)
	}
	if s.KeyFilePassword != "" && s.KeyFile == "" {
		return errValidationRequires("key-file-password", keyField)
	}
	if err := s.KeyFilePassword.Validate(s.hasSecretAgent()); err != nil {
		return errValidationSecret("key-file-password", err)
	}

	for field, path := range map[string]string{
		certField:     s.CertFile,
		keyField:      s.KeyFile,
		clientCAField: s.ClientCAFile,
	} {
		if err := safepath.ValidateClean(path); err != nil {
			return errValidationInvalidPath(field, path, err)
		}
	}

	if err := s.MinVersion.Validate(); err != nil {
		return err
	}
	for _, cipherSuite := range s.CipherSuites {
		if !secureServerCipherSuites[cipherSuite] {
			return errValidationInvalidValue("cipher-suites", cipherSuite, "secure Go TLS cipher suites")
		}
	}

	clientAuth, err := s.ClientAuth.ToModel()
	if err != nil {
		return err
	}
	if clientAuth != model.TLSClientAuthNone && s.ClientCAFile == "" {
		return errValidationRequires("client-auth", clientCAField)
	}

	return nil
}

// ToModel converts the HTTPS server DTO to its model representation.
func (s *HTTPSServerConfig) ToModel() *model.ServerConfigHTTPS {
	if s == nil {
		return nil
	}

	clientAuth, _ := s.ClientAuth.ToModel()
	minVersion, _ := s.MinVersion.ToModel()

	//nolint:staticcheck // We want to call embedded methods with embedded struct name.
	return &model.ServerConfigHTTPS{
		ListenerConfig:  s.ListenerConfig.toModel(),
		Port:            s.Port.ToModel(),
		CertFile:        s.CertFile,
		KeyFile:         s.KeyFile,
		KeyFilePassword: string(s.KeyFilePassword),
		SecretAgent:     s.SecretAgent.ToModel(),
		MinVersion:      minVersion,
		CipherSuites:    s.CipherSuites,
		ClientCAFile:    s.ClientCAFile,
		ClientAuth:      clientAuth,
	}
}

func (s *HTTPSServerConfig) fromModel(m *model.ServerConfigHTTPS) {
	if m == nil {
		return
	}

	s.ListenerConfig = newListenerFromModel(m.ListenerConfig)
	s.Port = NewPortFromModel(m.Port)
	s.CertFile = m.CertFile
	s.KeyFile = m.KeyFile
	s.KeyFilePassword = secret(m.KeyFilePassword)
	s.MinVersion = NewTLSMinVersionFromModel(m.MinVersion)
	s.CipherSuites = m.CipherSuites
	s.ClientCAFile = m.ClientCAFile
	s.ClientAuth = NewTLSClientAuthFromModel(m.ClientAuth)
	s.SecretAgent = newSecretAgentFromModel(m.SecretAgent)
}

// Compare compares HTTPS server configurations and reports every static field change.
func (s *HTTPSServerConfig) Compare(other *HTTPSServerConfig) error {
	if s == nil && other == nil {
		return nil
	}
	if s == nil {
		return errors.New("HTTPSServer added")
	}
	if other == nil {
		return errors.New("HTTPSServer removed")
	}

	var keyFilePasswordChange error
	if s.KeyFilePassword != other.KeyFilePassword {
		keyFilePasswordChange = errors.New("KeyFilePassword changed")
	}

	return errors.Join(
		//nolint:staticcheck // We want to call embedded methods with embedded struct name.
		s.ListenerConfig.compare(other.ListenerConfig),
		comparePointers("Port", s.Port, other.Port),
		compareValues("CertFile", s.CertFile, other.CertFile),
		compareValues("KeyFile", s.KeyFile, other.KeyFile),
		keyFilePasswordChange,
		compareValues("MinVersion", s.MinVersion, other.MinVersion),
		compareSlices("CipherSuites", s.CipherSuites, other.CipherSuites),
		compareValues("ClientCAFile", s.ClientCAFile, other.ClientCAFile),
		compareValues("ClientAuth", s.ClientAuth, other.ClientAuth),
		compareValues("SecretAgentName", s.SecretAgentName, other.SecretAgentName),
	)
}
