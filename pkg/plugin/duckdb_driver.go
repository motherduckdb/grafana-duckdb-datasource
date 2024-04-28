package plugin

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"os"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"
	"github.com/grafana/grafana-plugin-sdk-go/data/sqlutil"
	"github.com/grafana/sqlds/v3"
	"github.com/marcboeker/go-duckdb"
	"github.com/motherduck/duckdb-datasource/pkg/models"
)

type DuckDBDriver struct {
}

var allowedSettings = []string{"access_mode", "checkpoint_threshold", "debug_checkpoint_abort", "debug_force_external", "debug_force_no_cross_product", "debug_asof_iejoin", "prefer_range_joins", "debug_window_mode", "default_collation", "default_order", "default_null_order", "disabled_filesystems", "disabled_optimizers", "enable_external_access", "enable_fsst_vectors", "allow_unsigned_extensions", "custom_extension_repository", "autoinstall_extension_repository", "autoinstall_known_extensions", "autoload_known_extensions", "enable_object_cache", "enable_http_metadata_cache", "enable_profiling", "enable_progress_bar", "enable_progress_bar_print", "explain_output", "extension_directory", "external_threads", "file_search_path", "force_compression", "force_bitpacking_mode", "home_directory", "log_query_path", "lock_configuration", "immediate_transaction_mode", "integer_division", "max_expression_depth", "max_memory", "memory_limit", "null_order", "ordered_aggregate_threshold", "password", "perfect_ht_threshold", "pivot_filter_threshold", "pivot_limit", "preserve_identifier_case", "preserve_insertion_order", "profiler_history_size", "profile_output", "profiling_mode", "profiling_output", "progress_bar_time", "schema", "search_path", "temp_directory", "threads", "username", "arrow_large_buffer_size", "user", "wal_autocheckpoint", "worker_threads", "allocator_flush_threshold", "duckdb_api", "custom_user_agent", "motherduck_saas_mode", "motherduck_database_uuid", "motherduck_use_tls", "motherduck_background_catalog_refresh_long_poll_timeout", "motherduck_background_catalog_refresh_inactivity_timeout", "motherduck_lease_timeout", "motherduck_database_name", "motherduck_port", "motherduck_log_level", "pandas_analyze_sample", "motherduck_host", "motherduck_background_catalog_refresh", "binary_as_string", "motherduck_token", "Calendar", "TimeZone"}

// parse config from settings.JSONData
func parseConfig(settings backend.DataSourceInstanceSettings) (map[string]string, error) {

	config := make(map[string]string)
	err := json.Unmarshal(settings.JSONData, &config)
	if err != nil {
		return nil, err
	}
	return config, nil
}

func (d *DuckDBDriver) Connect(ctx context.Context, settings backend.DataSourceInstanceSettings, msg json.RawMessage) (*sql.DB, error) {

	config, err := models.LoadPluginSettings(settings)
	if err != nil {
		return nil, err
	}

	if config.Secrets.ApiKey != "" {
		os.Setenv("MOTHERDUCK_TOKEN", config.Secrets.ApiKey)
	}
	// // join config as url parmaeters
	// config, err := parseConfig(settings)

	// if err != nil {
	// 	return nil, err
	// }

	// // Create a slice to hold the URL-encoded key-value pairs
	// var parts []string
	// var dbPath string
	// // Iterate through the map
	// for key, value := range config {
	// 	if key == "path" {
	// 		dbPath = value
	// 		continue
	// 	} else if key == "apiKey" {
	// 	}

	// 	// URL-encode the key and the value
	// 	encodedKey := url.QueryEscape(key)
	// 	encodedValue := url.QueryEscape(value)

	// 	// Append the encoded key-value pair to the slice
	// 	parts = append(parts, fmt.Sprintf("%s=%s", encodedKey, encodedValue))
	// }

	// Join all parts with '&' to form the final query string
	// queryString := strings.Join(parts, "&")
	// dbString := strings.Join([]string{dbPath, queryString}, "?")
	log.Default().Printf("Connecting to DuckDB with %s\n", config.Path)
	driver := duckdb.Driver{}
	connector, err := driver.OpenConnector(config.Path)

	if err != nil {
		return nil, err
	}

	db := sql.OpenDB(connector)

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
