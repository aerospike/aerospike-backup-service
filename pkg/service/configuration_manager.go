package service

import (
	"bytes"
	"fmt"
	"net/http"
	"net/url"
	"os"

	"github.com/aerospike/aerospike-backup-service/internal/server/dto"
	"github.com/aerospike/aerospike-backup-service/pkg/model"
	"gopkg.in/yaml.v3"
)

type ConfigurationManager interface {
	ReadConfiguration() (*dto.Config, error)
	WriteConfiguration(config *dto.Config) error
}

type Reader interface {
	read(string) ([]byte, error)
}

type HTTPReader struct{}

func (h HTTPReader) read(url string) ([]byte, error) {
	// #nosec G107
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(resp.Body)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

type FileReader struct{}

func (f FileReader) read(url string) ([]byte, error) {
	return os.ReadFile(url)
}

type ConfigManagerBuilder struct {
	http      Reader
	file      Reader
	s3Builder S3ManagerBuilder
}

// NewConfigManagerBuilder returns a new ConfigManagerBuilder.
func NewConfigManagerBuilder() *ConfigManagerBuilder {
	return &ConfigManagerBuilder{
		http:      HTTPReader{},
		file:      FileReader{},
		s3Builder: S3ManagerBuilderImpl{},
	}
}

// NewConfigManager returns a new ConfigurationManager.
func (b *ConfigManagerBuilder) NewConfigManager(configFile string, remote bool,
) (ConfigurationManager, error) {
	configStorage, err := b.makeConfigStorage(configFile, remote)
	if err != nil {
		return nil, err
	}

	switch configStorage.Type {
	case model.S3:
		return b.s3Builder.NewS3ConfigurationManager(configStorage)
	case model.Local:
		return newLocalConfigurationManager(configStorage)
	default:
		return nil, fmt.Errorf("unknown type %s", configStorage.Type)
	}
}

func newLocalConfigurationManager(configStorage *model.Storage) (
	ConfigurationManager, error) {
	isHTTP, err := isHTTPPath(*configStorage.Path)
	if err != nil {
		return nil, err
	}
	if isHTTP {
		return NewHTTPConfigurationManager(*configStorage.Path), nil
	}
	return NewFileConfigurationManager(*configStorage.Path), nil
}

func (b *ConfigManagerBuilder) makeConfigStorage(configURI string, remote bool,
) (*model.Storage, error) {
	if !remote {
		return &model.Storage{
			Type: model.Local,
			Path: &configURI,
		}, nil
	}

	content, err := b.loadFileContent(configURI)
	if err != nil {
		return nil, err
	}

	configStorage := &model.Storage{}
	err = yaml.Unmarshal(content, configStorage)
	if err != nil {
		return nil, err
	}

	err = configStorage.Validate()
	if err != nil {
		return nil, err
	}
	return configStorage, nil
}

func (b *ConfigManagerBuilder) loadFileContent(configFile string) ([]byte, error) {
	isHTTP, err := isHTTPPath(configFile)
	if err != nil {
		return nil, err
	}
	if isHTTP {
		return b.http.read(configFile)
	}
	return b.file.read(configFile)
}

// isHTTPPath determines whether the specified path is a valid http/https.
func isHTTPPath(path string) (bool, error) {
	uri, err := url.Parse(path)
	if err != nil {
		return false, err
	}

	return uri.Scheme == "http" || uri.Scheme == "https", nil
}
