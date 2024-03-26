package service

import (
	"bytes"
	"github.com/aerospike/backup/pkg/model"
	"gopkg.in/yaml.v3"
	"net/http"
	"net/url"
	"os"
)

type ConfigurationManager interface {
	ReadConfiguration() (*model.Config, error)
	WriteConfiguration(config *model.Config) error
}

func NewConfigurationManager(configFile string, remote bool) (ConfigurationManager, error) {
	uri, err := url.Parse(configFile)
	if err != nil {
		return nil, err
	}

	isDownload := uri.Scheme == "http" || uri.Scheme == "https"

	if remote {
		return remoteConfigurationManager(configFile, isDownload)
	}
	if isDownload {
		return NewHTTPConfigurationManager(configFile), nil
	}
	return NewFileConfigurationManager(configFile), nil
}

func remoteConfigurationManager(configFile string, isDownload bool) (ConfigurationManager, error) {
	var buf []byte
	var err error

	if isDownload {
		buf, err = download(configFile)
	} else {
		buf, err = os.ReadFile(configFile)
	}
	if err != nil {
		return nil, err
	}

	configStorage := &model.Storage{}
	err = yaml.Unmarshal(buf, configStorage)
	if err != nil {
		return nil, err
	}

	err = configStorage.Validate()
	if err != nil {
		return nil, err
	}

	switch configStorage.Type {
	case model.S3:
		return NewS3ConfigurationManager(configStorage)
	default:
		return NewFileConfigurationManager(*configStorage.Path), nil
	}
}

func download(url string) ([]byte, error) {
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
