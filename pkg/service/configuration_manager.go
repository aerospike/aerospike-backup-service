package service

import (
	"bytes"
	"fmt"
	"net/http"
	"net/url"
	"os"

	"github.com/aerospike/backup/pkg/model"
	"gopkg.in/yaml.v3"
)

type ConfigurationManager interface {
	ReadConfiguration() (*model.Config, error)
	WriteConfiguration(config *model.Config) error
}

type Reader interface {
	read(string) ([]byte, error)
}

type HTTPReader struct{}

func (h HTTPReader) read(url string) ([]byte, error) {
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

func NewConfigManagerBuilder() *ConfigManagerBuilder {
	return &ConfigManagerBuilder{
		http:      HTTPReader{},
		file:      FileReader{},
		s3Builder: S3ManagerBuilderImpl{},
	}
}

func (b *ConfigManagerBuilder) NewConfigManager(configFile string, remote bool) (ConfigurationManager, error) {
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
		return nil, fmt.Errorf("unknown type %d", configStorage.Type)
	}
}

func newLocalConfigurationManager(configStorage *model.Storage) (ConfigurationManager, error) {
	isHttp, err := isHttpPath(*configStorage.Path)
	if err != nil {
		return nil, err
	}
	if isHttp {
		return NewHTTPConfigurationManager(*configStorage.Path), nil
	}
	return NewFileConfigurationManager(*configStorage.Path), nil
}

func (b *ConfigManagerBuilder) makeConfigStorage(configUri string, remote bool) (*model.Storage, error) {
	if !remote {
		return &model.Storage{
			Type: model.Local,
			Path: &configUri,
		}, nil
	}

	content, err := b.loadFileContent(configUri)
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
	isDownload, err := isHttpPath(configFile)
	if err != nil {
		return nil, err
	}
	if isDownload {
		return b.http.read(configFile)
	} else {
		return b.file.read(configFile)
	}
}

func isHttpPath(path string) (bool, error) {
	uri, err := url.Parse(path)
	if err != nil {
		return false, err
	}

	return uri.Scheme == "http" || uri.Scheme == "https", nil
}
