package plugin

import (
	"context"
	"testing"
)

// DuckDB installs extensions and stores secrets under its home directory, and
// fails with "Can't find the home directory" when it cannot resolve one.
func homeDirectory(t *testing.T, config map[string]any) string {
	t.Helper()

	db, err := (&DuckDBDriver{}).Connect(context.Background(), connectSettings(t, config), nil)
	if err != nil {
		t.Fatalf("Connect() returned %v", err)
	}
	defer db.Close()

	var dir string
	if err := db.QueryRow("SELECT current_setting('home_directory')").Scan(&dir); err != nil {
		t.Fatalf("reading home_directory returned %v", err)
	}
	return dir
}

func TestDataDirIsTheHomeDirectory(t *testing.T) {
	configured := t.TempDir()
	t.Setenv("GF_PATHS_DATA", t.TempDir())

	if dir := homeDirectory(t, map[string]any{"dataDir": configured}); dir != configured {
		t.Errorf("home_directory = %q, want the configured %q", dir, configured)
	}
}

func TestGrafanaDataPathIsTheDefaultHomeDirectory(t *testing.T) {
	dataPath := t.TempDir()
	t.Setenv("GF_PATHS_DATA", dataPath)

	if dir := homeDirectory(t, nil); dir != dataPath {
		t.Errorf("home_directory = %q, want %q", dir, dataPath)
	}
}

// Without either, DuckDB keeps resolving the home directory itself.
func TestHomeDirectoryIsLeftAloneWhenUnconfigured(t *testing.T) {
	t.Setenv("GF_PATHS_DATA", "")

	if dir := homeDirectory(t, nil); dir != "" {
		t.Errorf("home_directory = %q, want it unset", dir)
	}
}
