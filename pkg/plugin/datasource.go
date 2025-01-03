package plugin

import (
	"context"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt"
	"github.com/grafana/sqlds/v3"
)

// Make sure Datasource implements required interfaces. This is important to do
// since otherwise we will only get a not implemented error response from plugin in
// runtime. In this example datasource instance implements backend.QueryDataHandler,
// backend.CheckHealthHandler interfaces. Plugin should not implement all these
// interfaces - only those which are required for a particular task.
var (
	_ backend.QueryDataHandler      = (*SQLDatasourceWithDebug)(nil)
	_ backend.CheckHealthHandler    = (*SQLDatasourceWithDebug)(nil)
	_ instancemgmt.InstanceDisposer = (*SQLDatasourceWithDebug)(nil)
)

// NewDatasource creates a new `SQLDatasource`.
// It uses the provided settings argument to call the ds.Driver to connect to the SQL server
func (ds *SQLDatasourceWithDebug) NewDatasource(ctx context.Context, settings backend.DataSourceInstanceSettings) (instancemgmt.Instance, error) {
	newSqlDs, err := ds.SQLDatasource.NewDatasource(ctx, settings)
	if err != nil {
		return nil, err
	}
	ds.SQLDatasource = newSqlDs.(*sqlds.SQLDatasource)
	return ds, nil
}

// SQLDatasourceWithDebug
type SQLDatasourceWithDebug struct {
	*sqlds.SQLDatasource
}

// NewDatasource initializes the Datasource wrapper and instance manager
func NewDatasource(c sqlds.Driver) *SQLDatasourceWithDebug {
	return &SQLDatasourceWithDebug{
		SQLDatasource: sqlds.NewDatasource(c),
	}
}

// Dispose here tells plugin SDK that plugin wants to clean up resources when a new instance
// created. As soon as SQLDatasourceWithDebug settings change detected by SDK old SQLDatasourceWithDebug instance will
// be disposed and a new one will be created using NewSampleSQLDatasourceWithDebug factory function.
func (d *SQLDatasourceWithDebug) Dispose() {

	d.SQLDatasource.Dispose()

	// Clean up SQLDatasourceWithDebug instance resources.
}

// QueryData handles multiple queries and returns multiple responses.
// req contains the queries []DataQuery (where each query contains RefID as a unique identifier).
// The QueryDataResponse contains a map of RefID to the response for each query, and each response
// contains Frames ([]*Frame).
func (d *SQLDatasourceWithDebug) QueryData(ctx context.Context, req *backend.QueryDataRequest) (*backend.QueryDataResponse, error) {
	response, err := d.SQLDatasource.QueryData(ctx, req)

	return response, err
}

// CheckHealth handles health checks sent from Grafana to the plugin.
// The main use case for these health checks is the test button on the
// SQLDatasourceWithDebug configuration page which allows users to verify that
// a SQLDatasourceWithDebug is working as expected.
func (d *SQLDatasourceWithDebug) CheckHealth(_ context.Context, req *backend.CheckHealthRequest) (*backend.CheckHealthResult, error) {

	return d.SQLDatasource.CheckHealth(context.Background(), req)
}
