package plugin

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

// json.Marshal escapes the path. Concatenating it into the JSON string by hand
// would leave the backslashes of a Windows path reading as escape codes,
// causing the tests to fail on Windows.
func settingsForPath(t *testing.T, path string) backend.DataSourceInstanceSettings {
	t.Helper()

	jsonData, err := json.Marshal(map[string]string{"path": path})
	if err != nil {
		t.Fatalf("marshalling settings for %q returned %v", path, err)
	}
	return backend.DataSourceInstanceSettings{JSONData: jsonData}
}

// connect opens path through a driver of its own, so each database is
// independent of the one under test.
func connect(t *testing.T, path string) (*DuckDBDriver, *sql.DB) {
	t.Helper()

	driver := &DuckDBDriver{}
	db, err := driver.Connect(context.Background(), settingsForPath(t, path), nil)
	if err != nil {
		t.Fatalf("Connect(%q) returned %v", path, err)
	}
	return driver, db
}

func countRows(t *testing.T, db *sql.DB) int {
	t.Helper()

	var rows int
	if err := db.QueryRow("SELECT count(*) FROM t").Scan(&rows); err != nil {
		t.Fatalf("counting rows returned %v", err)
	}
	return rows
}

func writeDatabase(t *testing.T, path string, rows int) {
	t.Helper()

	driver, db := connect(t, path)
	if _, err := db.Exec("CREATE TABLE t AS SELECT n FROM range(?) r(n)", rows); err != nil {
		t.Fatalf("seeding %q returned %v", path, err)
	}
	if _, err := db.Exec("CHECKPOINT"); err != nil {
		t.Fatalf("checkpointing %q returned %v", path, err)
	}
	driver.closePrevious()
}

func copyFile(t *testing.T, from, to string) {
	t.Helper()

	contents, err := os.ReadFile(from)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(to, contents, 0o644); err != nil {
		t.Fatal(err)
	}
}

// DuckDB returns the instance it already has open for a path, so reconnecting
// only sees a replaced file once the previous database has been closed.
func TestConnectReadsAReplacedFile(t *testing.T) {
	// Windows keeps the open database file locked, so neither this test nor the
	// scenario it covers can replace a file the plugin is holding open.
	if runtime.GOOS == "windows" {
		t.Skip("a file DuckDB has open cannot be replaced on Windows")
	}

	dir := t.TempDir()
	small := filepath.Join(dir, "small.db")
	large := filepath.Join(dir, "large.db")
	target := filepath.Join(dir, "target.db")

	writeDatabase(t, small, 3)
	writeDatabase(t, large, 7)
	copyFile(t, small, target)

	driver, db := connect(t, target)
	defer driver.closePrevious()

	if rows := countRows(t, db); rows != 3 {
		t.Fatalf("count = %d, want 3 before the file is replaced", rows)
	}

	copyFile(t, large, target)

	reopened, err := driver.Connect(context.Background(), settingsForPath(t, target), nil)
	if err != nil {
		t.Fatalf("reconnecting returned %v", err)
	}
	if rows := countRows(t, reopened); rows != 7 {
		t.Errorf("count = %d, want 7 from the replaced file", rows)
	}
}
