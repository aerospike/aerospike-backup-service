package dto

import (
	"errors"
	"fmt"
	"slices"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

// LoggerConfig represents the backup service logger configuration.
// @Description LoggerConfig represents the backup service logger configuration.
//
//nolint:lll
type LoggerConfig struct {
	// Level is the logger level.
	Level *string `yaml:"level,omitempty" json:"level,omitempty" default:"INFO" enums:"TRACE,DEBUG,INFO,WARN,WARNING,ERROR"`
	// Format is the logger format (PLAIN, JSON).
	Format *string `yaml:"format,omitempty" json:"format,omitempty" default:"PLAIN" enums:"PLAIN,JSON"`
	// Whether to enable logging to the standard output.
	StdoutWriter *bool `yaml:"stdout-writer,omitempty" json:"stdout-writer,omitempty" default:"true"`
	// File writer logging configuration.
	FileWriter *FileLoggerConfig `yaml:"file-writer,omitempty" json:"file-writer,omitempty" default:""`
}

var (
	validLoggerLevels      = []string{"TRACE", "DEBUG", "INFO", "WARN", "WARNING", "ERROR"}
	supportedLoggerFormats = []string{"PLAIN", "JSON"}
)

// Validate validates the logger configuration.
func (l *LoggerConfig) Validate() error {
	if l == nil {
		return nil
	}
	if l.Level != nil && !slices.Contains(validLoggerLevels, *l.Level) {
		return fmt.Errorf("invalid logger level: %s", *l.Level)
	}
	if l.Format != nil && !slices.Contains(supportedLoggerFormats, *l.Format) {
		return fmt.Errorf("invalid logger format: %s", *l.Format)
	}
	if err := l.FileWriter.Validate(); err != nil {
		return err
	}

	return nil
}

func (l *LoggerConfig) ToModel() *model.LoggerConfig {
	if l == nil {
		return nil
	}

	return &model.LoggerConfig{
		Level:        l.Level,
		Format:       l.Format,
		StdoutWriter: l.StdoutWriter,
		FileWriter:   l.FileWriter.ToModel(),
	}
}

func (l *LoggerConfig) fromModel(m *model.LoggerConfig) {
	l.Level = m.Level
	l.Format = m.Format
	l.StdoutWriter = m.StdoutWriter
	if m.FileWriter != nil {
		l.FileWriter = &FileLoggerConfig{}
		l.FileWriter.fromModel(m.FileWriter)
	}
}

// Compare compares two LoggerConfig structs and returns an error if they differ.
func (l *LoggerConfig) Compare(other *LoggerConfig) error {
	if l == nil && other == nil {
		return nil
	}
	if l == nil {
		return errors.New("logger added")
	}
	if other == nil {
		return errors.New("logger removed")
	}

	var err = errors.Join(
		comparePointers("Level", l.Level, other.Level),
		comparePointers("Format", l.Format, other.Format),
		comparePointers("StdoutWriter", l.StdoutWriter, other.StdoutWriter),
	)

	if e := l.FileWriter.Compare(other.FileWriter); e != nil {
		err = errors.Join(err, fmt.Errorf("FileWriter changes: %w", e))
	}

	return err
}

// FileLoggerConfig represents the configuration for the file logger writer.
// @Description FileLoggerConfig represents the configuration for the file logger writer.
type FileLoggerConfig struct {
	// Filename is the file to write logs to.
	Filename string `yaml:"filename" json:"filename" example:"log.txt" validate:"required"`
	// Maximum size in megabytes of the log file before it gets rotated.
	MaxSize int `yaml:"maxsize" json:"maxsize" example:"100" extensions:"x-nullable" default:"100"`
	// Maximum number of days to retain old log files based on the
	// timestamp encoded in their filename. The default is not to remove old log files
	// based on age.
	MaxAge int `yaml:"maxage" json:"maxage" extensions:"x-nullable" default:"7"`
	// Maximum number of old log files to retain. The default
	// is to retain all old log files.
	MaxBackups int `yaml:"maxbackups" json:"maxbackups" extensions:"x-nullable" default:"3"`
	// Determines if the rotated log files should be compressed
	// using gzip. The default is not to perform compression.
	Compress bool `yaml:"compress" json:"compress" default:"false"`
}

// Validate validates the file logger configuration.
func (f *FileLoggerConfig) Validate() error {
	if f == nil {
		return nil
	}
	if f.Filename == "" {
		return errValidationEmptyField("logger file")
	}
	if f.MaxSize < 0 {
		return errValidationNegative("maxsize", f.MaxSize)
	}
	if f.MaxAge < 0 {
		return errValidationNegative("maxage", f.MaxAge)
	}
	if f.MaxBackups < 0 {
		return errValidationNegative("maxbackups", f.MaxBackups)
	}

	return nil
}

func (f *FileLoggerConfig) ToModel() *model.FileLoggerConfig {
	if f == nil {
		return nil
	}

	return &model.FileLoggerConfig{
		Filename:   f.Filename,
		MaxSize:    f.MaxSize,
		MaxAge:     f.MaxAge,
		MaxBackups: f.MaxBackups,
		Compress:   f.Compress,
	}
}

func (f *FileLoggerConfig) fromModel(m *model.FileLoggerConfig) {
	f.Filename = m.Filename
	f.MaxSize = m.MaxSize
	f.MaxAge = m.MaxAge
	f.MaxBackups = m.MaxBackups
	f.Compress = m.Compress
}

// Compare FileLoggerConfig with another and return detailed errors.
func (f *FileLoggerConfig) Compare(other *FileLoggerConfig) error {
	if f == nil && other == nil {
		return nil
	}
	if f == nil {
		return errors.New("FileLogger added")
	}
	if other == nil {
		return errors.New("FileLogger removed")
	}

	return errors.Join(
		compareValues("Filename", f.Filename, other.Filename),
		compareValues("MaxSize", f.MaxSize, other.MaxSize),
		compareValues("MaxAge", f.MaxAge, other.MaxAge),
		compareValues("MaxBackups", f.MaxBackups, other.MaxBackups),
		compareValues("Compress", f.Compress, other.Compress),
	)
}
