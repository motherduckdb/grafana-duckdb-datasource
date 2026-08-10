package plugin

import (
	"os"
	"testing"
)

func TestDuckDBDataDir(t *testing.T) {
	tests := []struct {
		name     string
		dataPath string
		home     string
		want     string
	}{
		{name: "prefers the Grafana data path", dataPath: "/var/lib/grafana", home: "/home/grafana", want: "/var/lib/grafana"},
		{name: "falls back to the process home", dataPath: "", home: "/home/grafana", want: "/home/grafana"},
		{name: "rejects an unwritable root home", dataPath: "", home: "/", want: os.TempDir()},
		{name: "falls back to a temporary directory", dataPath: "", home: "", want: os.TempDir()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("GF_PATHS_DATA", tt.dataPath)
			t.Setenv("HOME", tt.home)

			if got := duckDBDataDir(); got != tt.want {
				t.Errorf("duckDBDataDir() = %q, want %q", got, tt.want)
			}
		})
	}
}
