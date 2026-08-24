package plugin

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

func motherDuckSettings(t *testing.T, path string) backend.DataSourceInstanceSettings {
	t.Helper()

	token := os.Getenv("motherduck_token")
	if token == "" {
		token = os.Getenv("MOTHERDUCK_TOKEN")
	}
	if token == "" {
		t.Skip("motherduck_token is not set")
	}

	return backend.DataSourceInstanceSettings{
		JSONData:                []byte(`{"path":"` + path + `"}`),
		DecryptedSecureJSONData: map[string]string{"motherDuckToken": token},
	}
}

// A path of just "md:" attaches the whole workspace. ATTACH IF NOT EXISTS
// silently attaches something unqueryable, and TYPE motherduck is rejected
// outright, so neither may be used here.
func TestMotherDuckRootIsQueryable(t *testing.T) {
	driver := &DuckDBDriver{}
	db, err := driver.Connect(context.Background(), motherDuckSettings(t, "md:"), nil)
	if err != nil {
		t.Fatalf("Connect() returned %v", err)
	}
	defer db.Close()

	var attached int
	err = db.QueryRow(`
		SELECT count(*) FROM duckdb_databases()
		WHERE database_name NOT IN ('memory', 'system', 'temp')
	`).Scan(&attached)
	if err != nil {
		t.Fatalf("querying attached databases returned %v", err)
	}
	if attached == 0 {
		t.Error("no MotherDuck database attached, so the workspace is not queryable")
	}
}

// The MotherDuck setup cannot be repeated on a database, so a later boot query
// failing must not turn into an attach error that hides the real cause.
func TestInitSqlFailureKeepsItsOwnError(t *testing.T) {
	settings := motherDuckSettings(t, "md:")
	settings.JSONData = []byte(`{"path":"md:","initSql":"SELECT no_such_function()"}`)

	driver := &DuckDBDriver{}
	db, err := driver.Connect(context.Background(), settings, nil)
	if err != nil {
		t.Fatalf("Connect() returned %v", err)
	}
	defer db.Close()

	for attempt := 1; attempt <= 2; attempt++ {
		err := db.Ping()
		if err == nil {
			t.Fatalf("attempt %d: initSql failure went unreported", attempt)
		}
		if !strings.Contains(err.Error(), "no_such_function") {
			t.Errorf("attempt %d reported %q, want the initSql error", attempt, err)
		}
	}
}
