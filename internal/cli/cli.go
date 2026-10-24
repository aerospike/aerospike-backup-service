// Package cli declares the command line of the service binary.
//
// It lives outside cmd/backup so that the documentation generator can build the
// same command and publish its help text, instead of a copy pasted into the docs.
package cli

import (
	"errors"

	backup "github.com/aerospike/aerospike-backup-service/v3"
	"github.com/spf13/cobra"
)

// NewRootCommand returns the service command. start runs the service with the
// parsed configuration file path and remote flag.
func NewRootCommand(start func(configFile string, remote bool) error) *cobra.Command {
	var (
		configFile string
		remote     bool
	)

	validateFlags := func(_ *cobra.Command, _ []string) error {
		if len(configFile) == 0 {
			return errors.New("--config is required")
		}
		return nil
	}

	rootCmd := &cobra.Command{
		Use:     "aerospike-backup-service",
		Short:   "Aerospike Backup Service",
		Version: backup.Version,
		PreRunE: validateFlags,
		RunE: func(_ *cobra.Command, _ []string) error {
			return start(configFile, remote)
		},
	}

	rootCmd.Flags().StringVarP(&configFile, "config", "c", "", "configuration file path/URL")
	rootCmd.Flags().BoolVarP(&remote, "remote", "r", false, "use remote config file")

	return rootCmd
}
