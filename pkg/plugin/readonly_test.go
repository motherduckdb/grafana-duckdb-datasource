package plugin

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

// connectSettings marshals rather than interpolates, keeping Windows path
// backslashes valid JSON.
func connectSettings(t *testing.T, config map[string]any) backend.DataSourceInstanceSettings {
	t.Helper()

	jsonData, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	return backend.DataSourceInstanceSettings{JSONData: jsonData}
}

func seedFile(t *testing.T, path string) {
	t.Helper()

	db, err := (&DuckDBDriver{}).Connect(context.Background(), connectSettings(t, map[string]any{"path": path}), nil)
	if err != nil {
		t.Fatalf("seeding %q returned %v", path, err)
	}
	if _, err := db.Exec("CREATE TABLE t AS SELECT n FROM range(5) r(n)"); err != nil {
		t.Fatalf("creating the table returned %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestReadOnlyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "read-only.db")
	seedFile(t, path)

	db, err := (&DuckDBDriver{}).Connect(context.Background(),
		connectSettings(t, map[string]any{"path": path, "readOnly": true}), nil)
	if err != nil {
		t.Fatalf("Connect() returned %v", err)
	}
	defer db.Close()

	var mode string
	if err := db.QueryRow("SELECT current_setting('access_mode')").Scan(&mode); err != nil {
		t.Fatalf("reading access_mode returned %v", err)
	}
	if mode != "read_only" {
		t.Errorf("access_mode = %q, want read_only", mode)
	}

	var rows int
	if err := db.QueryRow("SELECT count(*) FROM t").Scan(&rows); err != nil {
		t.Errorf("reading returned %v, want it to work read-only", err)
	} else if rows != 5 {
		t.Errorf("count = %d, want 5", rows)
	}

	// Extensions and secrets live outside the database, so they still work.
	if _, err := db.Exec("CREATE SECRET s1 (TYPE s3, KEY_ID 'x', SECRET 'y', REGION 'eu-west-2')"); err != nil {
		t.Errorf("creating a secret returned %v, want it to work read-only", err)
	}
	if _, err := db.Exec("CREATE TEMP TABLE tmp1 AS SELECT 1 AS a"); err != nil {
		t.Errorf("creating a temp table returned %v, want it to work read-only", err)
	}

	if _, err := db.Exec("CREATE TABLE nope AS SELECT 1"); err == nil {
		t.Error("writing to the database succeeded, want it refused read-only")
	}
}

// An in-memory database cannot be opened read-only at all, so the plugin says
// which setting is wrong instead of passing it to DuckDB.
func TestReadOnlyRejectsInMemory(t *testing.T) {
	var db *sql.DB
	db, err := (&DuckDBDriver{}).Connect(context.Background(),
		connectSettings(t, map[string]any{"path": "", "readOnly": true}), nil)
	if db != nil {
		defer db.Close()
	}

	if err == nil {
		t.Fatal("Connect() accepted read-only for an in-memory database")
	}
	if !strings.Contains(err.Error(), "Read-only") {
		t.Errorf("Connect() returned %q, want it to name the read-only setting", err)
	}
}

// MotherDuck is attached onto an in-memory base, which cannot be read-only
// itself, so the ATTACH carries the flag instead.
func TestReadOnlyMotherDuck(t *testing.T) {
	token := os.Getenv("motherduck_token")
	if token == "" {
		token = os.Getenv("MOTHERDUCK_TOKEN")
	}
	if token == "" {
		t.Skip("motherduck_token is not set")
	}

	settings := connectSettings(t, map[string]any{"path": "md:sample_data", "readOnly": true})
	settings.DecryptedSecureJSONData = map[string]string{"motherDuckToken": token}

	db, err := (&DuckDBDriver{}).Connect(context.Background(), settings, nil)
	if err != nil {
		t.Fatalf("Connect() returned %v", err)
	}
	defer db.Close()

	var rows int
	if err := db.QueryRow("SELECT count(*) FROM sample_data.hn.hacker_news").Scan(&rows); err != nil {
		t.Fatalf("reading returned %v, want it to work read-only", err)
	}
	if rows == 0 {
		t.Error("no rows read from MotherDuck")
	}

	if _, err := db.Exec("CREATE TABLE sample_data.main.should_not_exist (a INT)"); err == nil {
		t.Error("writing to MotherDuck succeeded, want it refused read-only")
	}
}
