package plugin

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

func settingsForPath(path string) backend.DataSourceInstanceSettings {
	return backend.DataSourceInstanceSettings{JSONData: []byte(`{"path":"` + path + `"}`)}
}

// connect opens path through a driver of its own, so each database is
// independent of the one under test.
func connect(t *testing.T, path string) (*DuckDBDriver, *sql.DB) {
	t.Helper()

	driver := &DuckDBDriver{}
	db, err := driver.Connect(context.Background(), settingsForPath(path), nil)
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

	reopened, err := driver.Connect(context.Background(), settingsForPath(target), nil)
	if err != nil {
		t.Fatalf("reconnecting returned %v", err)
	}
	if rows := countRows(t, reopened); rows != 7 {
		t.Errorf("count = %d, want 7 from the replaced file", rows)
	}
}
