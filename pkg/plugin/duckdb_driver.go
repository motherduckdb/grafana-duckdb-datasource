package plugin

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"
	"github.com/grafana/grafana-plugin-sdk-go/data/sqlutil"
	"github.com/grafana/sqlds/v3"
)

type DuckDBDriver struct {
}

func (d *DuckDBDriver) Connect(ctx context.Context, settings backend.DataSourceInstanceSettings, msg json.RawMessage) (*sql.DB, error) {
	db, err := sql.Open("duckdb", settings.URL)
	if err != nil {
		return nil, err
	}
	return db, nil
}

func (d *DuckDBDriver) Settings(ctx context.Context, settings backend.DataSourceInstanceSettings) sqlds.DriverSettings {
	return sqlds.DriverSettings{
		Timeout:        30 * time.Second,
		FillMode:       &data.FillMissing{Mode: data.FillModeNull},
		Retries:        3,
		Pause:          100,
		RetryOn:        []string{},
		ForwardHeaders: false,
		Errors:         false,
	}

}

func (d *DuckDBDriver) FillMode() *data.FillMissing {
	return &data.FillMissing{Mode: data.FillModeNull}
}

func (d *DuckDBDriver) Macros() sqlds.Macros {
	return sqlutil.DefaultMacros
}

func (d *DuckDBDriver) Converters() []sqlutil.Converter {
	return sqlutil.NewRowConverter().Converters
}

// // Driver is a simple interface that defines how to connect to a backend SQL datasource
// // Plugin creators will need to implement this in order to create a managed datasource
// type Driver interface {
// 	// Connect connects to the database. It does not need to call `db.Ping()`
// 	Connect(context.Context, backend.DataSourceInstanceSettings, json.RawMessage) (*sql.DB, error)
// 	// Settings are read whenever the plugin is initialized, or after the data source settings are updated
// 	Settings(context.Context, backend.DataSourceInstanceSettings) DriverSettings
// 	Macros() Macros
// 	Converters() []sqlutil.Converter
// }
