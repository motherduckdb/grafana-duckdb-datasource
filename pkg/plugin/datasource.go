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

	newSqlDs, err := ds.SQLDatasource.NewDatasource(ctx, settings)
	if err != nil {
		return nil, err
	}
	ds.SQLDatasource = newSqlDs.(*sqlds.SQLDatasource)

	// Created once the database is open, as the file may not exist until DuckDB
	// opens it.
	ds.fileWatcher = NewFileWatcher(config.Path)

	return ds, nil
}

type FileWatcher struct {
	mu          sync.Mutex
	path        string
	isLocalFile bool

	// Modification time of the file the data source was built from.
	lastModified time.Time
}

func NewFileWatcher(path string) *FileWatcher {
	// If path is empty (in-memory duckdb) or connecting to motherduck, then file watcher is not needed.
	isLocalFile := !(strings.HasPrefix(path, "md:") || path == "")

	watcher := &FileWatcher{path: path, isLocalFile: isLocalFile}
	if isLocalFile {
		// Left zero if the file cannot be read, so the first check that can read
		// it reports an update and the data source is rebuilt against it.
		if info, err := os.Stat(path); err == nil {
			watcher.lastModified = info.ModTime()
		}
	}

	return watcher
}

// HasUpdate reports whether the file has been replaced since the last reload it
// was told about, along with the modification time it saw. It records nothing,
// so a reload that fails is simply reported again by the next check.
func (f *FileWatcher) HasUpdate() (time.Time, bool) {
	if !f.isLocalFile {
		backend.Logger.Debug("File watcher is not needed for non-local file (", "path=", f.path, ")")
		return time.Time{}, false
	}

	info, err := os.Stat(f.path)
	if err != nil {
		return time.Time{}, false
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	backend.Logger.Debug("Checking file modification", "path", f.path, "lastModified", f.lastModified, "currentModified", info.ModTime())

	return info.ModTime(), info.ModTime().After(f.lastModified)
}

func (f *FileWatcher) recordReload(modTime time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.lastModified = modTime
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
	if _, hasUpdate := d.fileWatcher.HasUpdate(); !hasUpdate {
		return nil
	}
	backend.Logger.Debug("DuckDB file has been modified, reloading DataSource.")

	d.mu.Lock()
	defer d.mu.Unlock()

	// Checked again under the lock: concurrent queries all see the same
	// replacement, and only the one that gets here first reloads it.
	modTime, hasUpdate := d.fileWatcher.HasUpdate()
	if !hasUpdate {
		return nil
	}

	newSqlDs, err := d.SQLDatasource.NewDatasource(ctx, d.settings)
	if err != nil {
		// Nothing is recorded, so the next query tries the reload again. The old
		// database is closed by now, so queries error until one succeeds either way.
		return err
	}
	d.SQLDatasource = newSqlDs.(*sqlds.SQLDatasource)
	d.fileWatcher.recordReload(modTime)

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
