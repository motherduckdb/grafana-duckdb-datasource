package plugin

import (
	"context"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt"
	"github.com/motherduckdb/grafana-duckdb-datasource/pkg/models"

	"github.com/grafana/sqlds/v3"
)

// Make sure Datasource implements required interfaces. This is important to do
// since otherwise we will only get a not implemented error response from plugin in
// runtime. In this example datasource instance implements backend.QueryDataHandler,
// backend.CheckHealthHandler interfaces. Plugin should not implement all these
// interfaces - only those which are required for a particular task.
var (
	_ backend.QueryDataHandler      = (*SQLDataSourceWrapper)(nil)
	_ backend.CheckHealthHandler    = (*SQLDataSourceWrapper)(nil)
	_ instancemgmt.InstanceDisposer = (*SQLDataSourceWrapper)(nil)
)

// NewDatasource creates a new `SQLDatasource`.
// It uses the provided settings argument to call the ds.Driver to connect to the SQL server
func (ds *SQLDataSourceWrapper) NewDatasource(ctx context.Context, settings backend.DataSourceInstanceSettings) (instancemgmt.Instance, error) {
	ds.settings = settings

	config, err := models.LoadPluginSettings(settings)
	if err != nil {
		return nil, err
	}

	ds.fileWatcher = NewFileWatcher(config.Path)

	newSqlDs, err := ds.SQLDatasource.NewDatasource(ctx, settings)
	if err != nil {
		return nil, err
	}
	ds.SQLDatasource = newSqlDs.(*sqlds.SQLDatasource)

	return ds, nil
}

type FileWatcher struct {
	mu           sync.Mutex
	path         string
	isLocalFile  bool
	lastModified time.Time
}

func NewFileWatcher(path string) *FileWatcher {
	// If path is empty (in-memory duckdb) or connecting to motherduck, then file watcher is not needed.
	isLocalFile := !(strings.HasPrefix(path, "md:") || path == "")

	return &FileWatcher{path: path, isLocalFile: isLocalFile, lastModified: time.Now()}
}

// HasUpdate reports a change to exactly one caller, so concurrent queries do
// not all try to reload the same replacement.
func (f *FileWatcher) HasUpdate() bool {
	if !f.isLocalFile {
		backend.Logger.Debug("File watcher is not needed for non-local file (", "path=", f.path, ")")
		return false
	}

	info, err := os.Stat(f.path)
	if err != nil {
		return false
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	backend.Logger.Debug("Checking file modification", "path", f.path, "lastModified", f.lastModified, "currentModified", info.ModTime())

	if info.ModTime().After(f.lastModified) {
		f.lastModified = info.ModTime()
		return true
	}

	return false
}

// retry makes the next check report an update again, so a reload that failed is
// tried once more instead of waiting for the file to change again.
func (f *FileWatcher) retry() {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.lastModified = time.Time{}
}

// SQLDataSourceWrapper
type SQLDataSourceWrapper struct {
	*sqlds.SQLDatasource

	// Held for reading while a query runs and for writing while the data source
	// is rebuilt, so a reload never happens underneath a query.
	mu          sync.RWMutex
	fileWatcher *FileWatcher
	settings    backend.DataSourceInstanceSettings
	driver      sqlds.Driver
}

// NewDatasource initializes the Datasource wrapper and instance manager
func NewDatasource(c sqlds.Driver) *SQLDataSourceWrapper {
	return &SQLDataSourceWrapper{
		SQLDatasource: sqlds.NewDatasource(c),
		driver:        c,
	}
}

// Dispose here tells plugin SDK that plugin wants to clean up resources when a new instance
// created. As soon as SQLDataSourceWrapper settings change detected by SDK old SQLDataSourceWrapper instance will
// be disposed and a new one will be created using NewSampleSQLDatasourceWithDebug factory function.
func (d *SQLDataSourceWrapper) Dispose() {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.SQLDatasource.Dispose()

	// sqlds.Dispose does nothing, so the database would otherwise stay open and
	// keep being handed back for this path after the data source is replaced.
	if duckDBDriver, ok := d.driver.(*DuckDBDriver); ok {
		duckDBDriver.closePrevious()
	}
}

// QueryData handles multiple queries and returns multiple responses.
// req contains the queries []DataQuery (where each query contains RefID as a unique identifier).
// The QueryDataResponse contains a map of RefID to the response for each query, and each response
// contains Frames ([]*Frame).
func (d *SQLDataSourceWrapper) QueryData(ctx context.Context, req *backend.QueryDataRequest) (*backend.QueryDataResponse, error) {
	if err := d.reloadIfReplaced(ctx); err != nil {
		return nil, err
	}

	// Held for the whole query: queries run concurrently with each other, and a
	// reload waits for them to finish rather than closing the database underneath.
	d.mu.RLock()
	defer d.mu.RUnlock()

	return d.SQLDatasource.QueryData(ctx, req)
}

// reloadIfReplaced reopens the database when the file behind it has changed.
// DuckDB keeps handing back the instance it already has open for a path, so the
// old one has to be closed before the new one is opened, and no query may be
// running while that happens.
func (d *SQLDataSourceWrapper) reloadIfReplaced(ctx context.Context) error {
	// Checked before locking, so unchanged files leave queries running concurrently.
	if !d.fileWatcher.HasUpdate() {
		return nil
	}
	backend.Logger.Debug("DuckDB file has been modified, reloading DataSource.")

	d.mu.Lock()
	defer d.mu.Unlock()

	newSqlDs, err := d.SQLDatasource.NewDatasource(ctx, d.settings)
	if err != nil {
		// The old database is closed by now, so leaving it in place would serve
		// errors until the file changed again. Reload on the next query instead.
		d.fileWatcher.retry()
		return err
	}
	d.SQLDatasource = newSqlDs.(*sqlds.SQLDatasource)

	return nil
}

// CheckHealth handles health checks sent from Grafana to the plugin.
// The main use case for these health checks is the test button on the
// SQLDataSourceWrapper configuration page which allows users to verify that
// a SQLDataSourceWrapper is working as expected.
func (d *SQLDataSourceWrapper) CheckHealth(ctx context.Context, req *backend.CheckHealthRequest) (*backend.CheckHealthResult, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return d.SQLDatasource.CheckHealth(ctx, req)
}
