package cmd

import (
	"encoding/json"
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

// Version information - set via ldflags during build
var (
	Version   = "dev"
	Commit    = "none"
	BuildDate = "unknown"
)

type versionInfo struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildDate string `json:"build_date"`
	GoVersion string `json:"go_version"`
	Platform  string `json:"platform"`
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Long:  `Print the version, commit hash, build date, and platform information.`,
	Run: func(cmd *cobra.Command, args []string) {
		info := versionInfo{
			Version:   Version,
			Commit:    Commit,
			BuildDate: BuildDate,
			GoVersion: runtime.Version(),
			Platform:  fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
		}

		if jsonOut {
			data, _ := json.MarshalIndent(info, "", "  ")
			fmt.Println(string(data))
			return
		}

		fmt.Printf("op-ssh-manager %s\n", info.Version)
		fmt.Printf("  Commit:     %s\n", info.Commit)
		fmt.Printf("  Built:      %s\n", info.BuildDate)
		fmt.Printf("  Go version: %s\n", info.GoVersion)
		fmt.Printf("  Platform:   %s\n", info.Platform)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
