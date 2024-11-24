package model

import (
	"errors"
	"fmt"
)

// LoggerConfig represents the backup service logger configuration.
// @Description LoggerConfig represents the backup service logger configuration.
//

type LoggerConfig struct {
	// Level is the logger level.
	Level *string
	// Format is the logger format (PLAIN, JSON).
	Format *string
	// Whether to enable logging to the standard output.
	StdoutWriter *bool
	// File writer logging configuration.
	FileWriter *FileLoggerConfig
}

// GetLevelOrDefault returns the value of the Level property.
// If the property is not set, it returns the default value.
func (l *LoggerConfig) GetLevelOrDefault() string {
	if l.Level != nil {
		return *l.Level
	}
	return *defaultConfig.logger.Level
}

// GetFormatOrDefault returns the value of the Format property.
// If the property is not set, it returns the default value.
func (l *LoggerConfig) GetFormatOrDefault() string {
	if l.Format != nil {
		return *l.Format
	}
	return *defaultConfig.logger.Format
}

// GetStdoutWriterOrDefault returns the value of the StdoutWriter property.
// If the property is not set, it returns the default value.
func (l *LoggerConfig) GetStdoutWriterOrDefault() bool {
	if l.StdoutWriter != nil {
		return *l.StdoutWriter
	}
	return *defaultConfig.logger.StdoutWriter
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

	var err error

	if e := comparePointers("Level", l.Level, other.Level); e != nil {
		err = errors.Join(err, e)
	}
	if e := comparePointers("Format", l.Format, other.Format); e != nil {
		err = errors.Join(err, e)
	}
	if e := comparePointers("StdoutWriter", l.StdoutWriter, other.StdoutWriter); e != nil {
		err = errors.Join(err, e)
	}
	if e := l.FileWriter.Compare(other.FileWriter); e != nil {
		err = errors.Join(err, fmt.Errorf("FileWriter changes: %w", e))
	}

	return err
}

// FileLoggerConfig represents the configuration for the file logger writer.
// @Description FileLoggerConfig represents the configuration for the file logger writer.
type FileLoggerConfig struct {
	// Filename is the file to write logs to.
	Filename string
	// MaxSize is the maximum size in megabytes of the log file before it gets rotated.
	MaxSize int
	// MaxAge is the maximum number of days to retain old log files based on the
	// timestamp encoded in their filename. The default is not to remove old log files
	// based on age.
	MaxAge int
	// MaxBackups is the maximum number of old log files to retain. The default
	// is to retain all old log files.
	MaxBackups int
	// Compress determines if the rotated log files should be compressed
	// using gzip. The default is not to perform compression.
	Compress bool
}

// Compare FileLoggerConfig with another and return detailed errors.
func (f *FileLoggerConfig) Compare(other *FileLoggerConfig) error {
	if f == nil && other == nil {
		return nil
	}
	if f == nil {
		return fmt.Errorf("FileLogger added")
	}
	if other == nil {
		return fmt.Errorf("FileLogger removed")
	}

	var err error

	if e := compareSimpleValues("Filename", f.Filename, other.Filename); e != nil {
		err = errors.Join(err, e)
	}
	if e := compareSimpleValues("MaxSize", f.MaxSize, other.MaxSize); e != nil {
		err = errors.Join(err, e)
	}
	if e := compareSimpleValues("MaxAge", f.MaxAge, other.MaxAge); e != nil {
		err = errors.Join(err, e)
	}
	if e := compareSimpleValues("MaxBackups", f.MaxBackups, other.MaxBackups); e != nil {
		err = errors.Join(err, e)
	}
	if e := compareSimpleValues("Compress", f.Compress, other.Compress); e != nil {
		err = errors.Join(err, e)
	}

	return err
}
