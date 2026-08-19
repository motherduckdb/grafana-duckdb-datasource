package plugin

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// unusableDir returns a path that cannot hold a directory, which root cannot
// write to either, unlike a directory with its permission bits cleared.
func unusableDir(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "regular-file")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestDuckDBDataDir(t *testing.T) {
	t.Run("uses the configured directory", func(t *testing.T) {
		configured := t.TempDir()
		t.Setenv("GF_PATHS_DATA", t.TempDir())

		got, err := duckDBDataDir(context.Background(), configured)
		if err != nil {
			t.Fatalf("duckDBDataDir() returned %v", err)
		}
		if got != configured {
			t.Errorf("duckDBDataDir() = %q, want the configured %q", got, configured)
		}
	})

	t.Run("reports a configured directory it cannot use", func(t *testing.T) {
		if _, err := duckDBDataDir(context.Background(), unusableDir(t)); err == nil {
			t.Error("duckDBDataDir() accepted an unusable configured directory")
		}
	})

	t.Run("falls back to the Grafana data path", func(t *testing.T) {
		dataPath := t.TempDir()
		t.Setenv("GF_PATHS_DATA", dataPath)

		got, err := duckDBDataDir(context.Background(), "")
		if err != nil {
			t.Fatalf("duckDBDataDir() returned %v", err)
		}
		if got != dataPath {
			t.Errorf("duckDBDataDir() = %q, want %q", got, dataPath)
		}
	})

	t.Run("skips an unusable Grafana data path", func(t *testing.T) {
		unusable := unusableDir(t)
		t.Setenv("GF_PATHS_DATA", unusable)

		got, err := duckDBDataDir(context.Background(), "")
		if err != nil {
			t.Fatalf("duckDBDataDir() returned %v", err)
		}
		if got == unusable {
			t.Error("duckDBDataDir() returned a directory it cannot use")
		}
	})

	// Extensions preinstalled into a read-only directory must keep being used.
	t.Run("keeps a directory that is already set up", func(t *testing.T) {
		prepared := t.TempDir()
		if err := os.Mkdir(filepath.Join(prepared, ".duckdb"), 0o500); err != nil {
			t.Fatal(err)
		}
		t.Setenv("GF_PATHS_DATA", prepared)

		got, err := duckDBDataDir(context.Background(), "")
		if err != nil {
			t.Fatalf("duckDBDataDir() returned %v", err)
		}
		if got != prepared {
			t.Errorf("duckDBDataDir() = %q, want the prepared %q", got, prepared)
		}
	})

	// A data source that never installs an extension worked before this
	// fallback existed, and must keep working when no directory is usable.
	t.Run("gives up quietly rather than failing the connection", func(t *testing.T) {
		unusable := unusableDir(t)
		t.Setenv("GF_PATHS_DATA", unusable)
		t.Setenv("HOME", unusable)
		t.Setenv("TMPDIR", unusable)

		got, err := duckDBDataDir(context.Background(), "")
		if err != nil {
			t.Fatalf("duckDBDataDir() returned %v, want no error", err)
		}
		if got != "" && claimDataDir(got) != nil {
			t.Errorf("duckDBDataDir() = %q, which is not usable", got)
		}
	})

	// Grafana 12.4 and later forward nothing, so a writable directory still has to be found.
	t.Run("finds a directory without any Grafana environment", func(t *testing.T) {
		t.Setenv("GF_PATHS_DATA", "")
		t.Setenv("HOME", "")

		got, err := duckDBDataDir(context.Background(), "")
		if err != nil {
			t.Fatalf("duckDBDataDir() returned %v", err)
		}
		if err := claimDataDir(got); err != nil {
			t.Errorf("duckDBDataDir() = %q, which is not writable: %v", got, err)
		}
	})
}
