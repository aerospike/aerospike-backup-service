package configuration

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"

	"github.com/aerospike/aerospike-backup-service/v2/internal/server/dto"
	"github.com/aerospike/aerospike-backup-service/v2/pkg/model"
	"gopkg.in/yaml.v3"
)

type Manager interface {
	ReadConfiguration() (io.ReadCloser, error)
	WriteConfiguration(config *model.Config) error
}

// NewConfigManager returns a new Manager.
func NewConfigManager(configFile string, remote bool) (Manager, error) {
	configStorage, err := makeConfigStorage(configFile, remote)
	if err != nil {
		return nil, err
	}

	switch configStorage.Type {
	case model.S3:
		return newS3ConfigurationManager(configStorage)
	case model.Local:
		return newLocalConfigurationManager(configStorage)
	default:
		return nil, fmt.Errorf("unknown type %s", configStorage.Type)
	}
}

func newLocalConfigurationManager(configStorage *model.Storage) (
	Manager, error) {
	isHTTP, err := isHTTPPath(*configStorage.Path)
	if err != nil {
		return nil, err
	}
	if isHTTP {
		return NewHTTPConfigurationManager(*configStorage.Path), nil
	}
	return NewFileConfigurationManager(*configStorage.Path), nil
}

func makeConfigStorage(configURI string, remote bool,
) (*model.Storage, error) {
	if !remote {
		return &model.Storage{
			Type: model.Local,
			Path: &configURI,
		}, nil
	}

	content, err := loadFileContent(configURI)
	if err != nil {
		return nil, err
	}

	configStorage := &dto.Storage{}
	err = yaml.Unmarshal(content, configStorage)
	if err != nil {
		return nil, err
	}

	err = configStorage.Validate()
	if err != nil {
		return nil, err
	}
	return configStorage.ToModel(), nil
}

func loadFileContent(configFile string) ([]byte, error) {
	isHTTP, err := isHTTPPath(configFile)
	if err != nil {
		return nil, err
	}
	if isHTTP {
		return readFromHTTP(configFile)
	}
	return os.ReadFile(configFile)
}

func readFromHTTP(url string) ([]byte, error) {
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

// isHTTPPath determines whether the specified path is a valid http/https.
func isHTTPPath(path string) (bool, error) {
	uri, err := url.Parse(path)
	if err != nil {
		return false, err
	}

	return uri.Scheme == "http" || uri.Scheme == "https", nil
}
