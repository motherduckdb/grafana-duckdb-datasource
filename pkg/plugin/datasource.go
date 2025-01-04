package plugin

import (
	"context"
	"os"
	"strings"
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
	path         string
	isLocalFile  bool
	lastModified time.Time
}

func NewFileWatcher(path string) *FileWatcher {
	// If path is empty (in-memory duckdb) or connecting to motherduck, then file watcher is not needed.
	isLocalFile := !(strings.HasPrefix(path, "md:") || path == "")

	return &FileWatcher{path: path, isLocalFile: isLocalFile, lastModified: time.Now()}
}

func (f *FileWatcher) HasUpdate() bool {
	if !f.isLocalFile {
		backend.Logger.Debug("File watcher is not needed for non-local file (", "path=", f.path, ")")
		return false
	}

	info, err := os.Stat(f.path)
	if err != nil {
		return false
	}
	backend.Logger.Debug("Checking file modification", "path", f.path, "lastModified", f.lastModified, "currentModified", info.ModTime())

	if info.ModTime().After(f.lastModified) {
		f.lastModified = info.ModTime()
		return true
	}

	return false
}

// SQLDataSourceWrapper
type SQLDataSourceWrapper struct {
	*sqlds.SQLDatasource

	fileWatcher *FileWatcher
	settings    backend.DataSourceInstanceSettings
}

// NewDatasource initializes the Datasource wrapper and instance manager
func NewDatasource(c sqlds.Driver) *SQLDataSourceWrapper {
	return &SQLDataSourceWrapper{
		SQLDatasource: sqlds.NewDatasource(c),
	}
}

// Dispose here tells plugin SDK that plugin wants to clean up resources when a new instance
// created. As soon as SQLDataSourceWrapper settings change detected by SDK old SQLDataSourceWrapper instance will
// be disposed and a new one will be created using NewSampleSQLDatasourceWithDebug factory function.
func (d *SQLDataSourceWrapper) Dispose() {

	d.SQLDatasource.Dispose()

	// Clean up SQLDataSourceWrapper instance resources.
}

// QueryData handles multiple queries and returns multiple responses.
// req contains the queries []DataQuery (where each query contains RefID as a unique identifier).
// The QueryDataResponse contains a map of RefID to the response for each query, and each response
// contains Frames ([]*Frame).
func (d *SQLDataSourceWrapper) QueryData(ctx context.Context, req *backend.QueryDataRequest) (*backend.QueryDataResponse, error) {
	if d.fileWatcher.HasUpdate() {
		backend.Logger.Debug("DuckDB file has been modified, reloading DataSource.")
		newSqlDs, err := d.SQLDatasource.NewDatasource(ctx, d.settings)
		if err != nil {
			return nil, err
		}
		d.SQLDatasource = newSqlDs.(*sqlds.SQLDatasource)
	}

	response, err := d.SQLDatasource.QueryData(ctx, req)

	return response, err
}

// CheckHealth handles health checks sent from Grafana to the plugin.
// The main use case for these health checks is the test button on the
// SQLDataSourceWrapper configuration page which allows users to verify that
// a SQLDataSourceWrapper is working as expected.
func (d *SQLDataSourceWrapper) CheckHealth(_ context.Context, req *backend.CheckHealthRequest) (*backend.CheckHealthResult, error) {
	return d.SQLDatasource.CheckHealth(context.Background(), req)
}
