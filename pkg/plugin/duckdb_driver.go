package plugin

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"log"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"time"

	"github.com/mitchellh/mapstructure"

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
		os.Setenv("motherduck_token", config.Secrets.ApiKey)
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
	connector, err := duckdb.NewConnector(config.Path, func(execer driver.ExecerContext) error {
		bootQueries := []string{
			"INSTALL 'motherduck'",
			"LOAD 'motherduck'",
			"SELECT * FROM duckdb_extensions()",
		}

		for _, query := range bootQueries {
			_, err = execer.ExecContext(context.Background(), query, nil)
			if err != nil {
				return err
			}
		}
		return nil
	})

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
	return GetConverterList()
}

// Originally from https://github.com/snakedotdev/grafana-duckdb-datasource
// Apache 2.0 Licensed
// Copyright snakedotdev
// Modified from original version
type NullDecimal struct {
	Decimal duckdb.Decimal
	Valid   bool
}

func (n *NullDecimal) Scan(value any) error {
	if value == nil {
		n.Decimal = duckdb.Decimal{
			Width: 0,
			Scale: 0,
			Value: nil,
		}
		n.Valid = false
		return nil
	}
	n.Valid = true
	if err := mapstructure.Decode(value, &n.Decimal); err != nil {
		return err
	}
	return nil
}

func (n *NullDecimal) Value() (driver.Value, error) {
	if !n.Valid {
		return nil, nil
	}
	return n.Decimal, nil
}

func GetConverterList() []sqlutil.Converter {
	// NEED:
	// NULL to uint64, uint32, uint16, uint8,  not supported
	// Names: BIT, UBIGINT, UHUGEINT, UINTEGER, USMALLINT, UTINYINT
	strConverters := sqlutil.ToConverters([]sqlutil.StringConverter{
		{
			Name:           "handle FLOAT8",
			InputScanKind:  reflect.Interface,
			InputTypeName:  "FLOAT8",
			ConversionFunc: func(in *string) (*string, error) { return in, nil },
			Replacer: &sqlutil.StringFieldReplacer{
				OutputFieldType: data.FieldTypeNullableFloat64,
				ReplaceFunc: func(in *string) (any, error) {
					if in == nil {
						return nil, nil
					}
					v, err := strconv.ParseFloat(*in, 64)
					if err != nil {
						return nil, err
					}
					return &v, nil
				},
			},
		},
		{
			Name:           "handle FLOAT32",
			InputScanKind:  reflect.Interface,
			InputTypeName:  "FLOAT32",
			ConversionFunc: func(in *string) (*string, error) { return in, nil },
			Replacer: &sqlutil.StringFieldReplacer{
				OutputFieldType: data.FieldTypeNullableFloat64,
				ReplaceFunc: func(in *string) (any, error) {
					if in == nil {
						return nil, nil
					}
					v, err := strconv.ParseFloat(*in, 64)
					if err != nil {
						return nil, err
					}
					return &v, nil
				},
			},
		},
		{
			Name:           "handle FLOAT",
			InputScanKind:  reflect.Interface,
			InputTypeName:  "FLOAT",
			ConversionFunc: func(in *string) (*string, error) { return in, nil },
			Replacer: &sqlutil.StringFieldReplacer{
				OutputFieldType: data.FieldTypeNullableFloat64,
				ReplaceFunc: func(in *string) (any, error) {
					if in == nil {
						return nil, nil
					}
					v, err := strconv.ParseFloat(*in, 64)
					if err != nil {
						return nil, err
					}
					return &v, nil
				},
			},
		},
		{
			Name:           "handle INT2",
			InputScanKind:  reflect.Interface,
			InputTypeName:  "INT2",
			ConversionFunc: func(in *string) (*string, error) { return in, nil },
			Replacer: &sqlutil.StringFieldReplacer{
				OutputFieldType: data.FieldTypeNullableInt16,
				ReplaceFunc: func(in *string) (any, error) {
					if in == nil {
						return nil, nil
					}
					i64, err := strconv.ParseInt(*in, 10, 16)
					if err != nil {
						return nil, err
					}
					v := int16(i64)
					return &v, nil
				},
			},
		},
		{
			Name:           "handle INT8",
			InputScanKind:  reflect.Interface,
			InputTypeName:  "INT8",
			ConversionFunc: func(in *string) (*string, error) { return in, nil },
			Replacer: &sqlutil.StringFieldReplacer{
				OutputFieldType: data.FieldTypeNullableInt16,
				ReplaceFunc: func(in *string) (any, error) {
					if in == nil {
						return nil, nil
					}
					i64, err := strconv.ParseInt(*in, 10, 16)
					if err != nil {
						return nil, err
					}
					v := int16(i64)
					return &v, nil
				},
			},
		},
		{
			Name:           "handle TINYINT",
			InputScanKind:  reflect.Interface,
			InputTypeName:  "TINYINT",
			ConversionFunc: func(in *string) (*string, error) { return in, nil },
			Replacer: &sqlutil.StringFieldReplacer{
				OutputFieldType: data.FieldTypeNullableInt16,
				ReplaceFunc: func(in *string) (any, error) {
					if in == nil {
						return nil, nil
					}
					i64, err := strconv.ParseInt(*in, 10, 16)
					if err != nil {
						return nil, err
					}
					v := int16(i64)
					return &v, nil
				},
			},
		},
		{
			Name:           "handle INT16",
			InputScanKind:  reflect.Interface,
			InputTypeName:  "INT16",
			ConversionFunc: func(in *string) (*string, error) { return in, nil },
			Replacer: &sqlutil.StringFieldReplacer{
				OutputFieldType: data.FieldTypeNullableInt16,
				ReplaceFunc: func(in *string) (any, error) {
					if in == nil {
						return nil, nil
					}
					i64, err := strconv.ParseInt(*in, 10, 16)
					if err != nil {
						return nil, err
					}
					v := int16(i64)
					return &v, nil
				},
			},
		},
		{
			Name:           "handle SMALLINT",
			InputScanKind:  reflect.Interface,
			InputTypeName:  "SMALLINT",
			ConversionFunc: func(in *string) (*string, error) { return in, nil },
			Replacer: &sqlutil.StringFieldReplacer{
				OutputFieldType: data.FieldTypeNullableInt16,
				ReplaceFunc: func(in *string) (any, error) {
					if in == nil {
						return nil, nil
					}
					i64, err := strconv.ParseInt(*in, 10, 16)
					if err != nil {
						return nil, err
					}
					v := int16(i64)
					return &v, nil
				},
			},
		},
	}...,
	)
	converters := []sqlutil.Converter{
		{
			Name:           "NULLABLE decimal converter",
			InputScanType:  reflect.TypeOf(NullDecimal{}),
			InputTypeRegex: regexp.MustCompile("DECIMAL.*"),
			FrameConverter: sqlutil.FrameConverter{
				FieldType: data.FieldTypeNullableFloat64,
				ConverterFunc: func(n interface{}) (interface{}, error) {
					v := n.(*NullDecimal)

					if !v.Valid {
						return (*float64)(nil), nil
					}

					f := v.Decimal.Float64()
					return &f, nil
				},
			},
		},
	}
	//{
	//	Name:           "handle FLOAT4",
	//	InputScanType: reflect.TypeOf(sql.NullInt16{}),
	//	InputTypeName:  "FLOAT4",
	//	FrameConverter: sqlutil.FrameConverter{
	//		FieldType: data.FieldTypeNullableInt8,
	//		ConverterFunc: func(in interface{}) (interface{}, error) { return in, nil },
	//	},
	//	ConversionFunc:
	//	Replacer: &sqlutil.StringFieldReplacer{
	//		OutputFieldType: data.FieldTypeNullableFloat64,
	//		ReplaceFunc: func(in *string) (any, error) {
	//			if in == nil {
	//				return nil, nil
	//			}
	//			v, err := strconv.ParseFloat(*in, 64)
	//			if err != nil {
	//				return nil, err
	//			}
	//			return &v, nil
	//		},
	//	},
	//},
	//{
	//	Name:           "handle FLOAT8",
	//	InputScanKind:  reflect.Interface,
	//	InputTypeName:  "FLOAT8",
	//	ConversionFunc: func(in *string) (*string, error) { return in, nil },
	//	Replacer: &sqlutil.StringFieldReplacer{
	//		OutputFieldType: data.FieldTypeNullableFloat64,
	//		ReplaceFunc: func(in *string) (any, error) {
	//			if in == nil {
	//				return nil, nil
	//			}
	//			v, err := strconv.ParseFloat(*in, 64)
	//			if err != nil {
	//				return nil, err
	//			}
	//			return &v, nil
	//		},
	//	},
	//},
	//{
	//	Name:           "handle NUMERIC",
	//	InputScanKind:  reflect.Interface,
	//	InputTypeName:  "NUMERIC",
	//	ConversionFunc: func(in *string) (*string, error) { return in, nil },
	//	Replacer: &sqlutil.StringFieldReplacer{
	//		OutputFieldType: data.FieldTypeNullableFloat64,
	//		ReplaceFunc: func(in *string) (any, error) {
	//			if in == nil {
	//				return nil, nil
	//			}
	//			v, err := strconv.ParseFloat(*in, 64)
	//			if err != nil {
	//				return nil, err
	//			}
	//			return &v, nil
	//		},
	//	},
	//},
	//{
	//	Name:           "handle DECIMAL",
	//	InputScanKind:  reflect.Interface,
	//	InputTypeName:  "DECIMAL(15,2)",
	//	ConversionFunc: func(in *string) (*string, error) { return in, nil },
	//	Replacer: &sqlutil.StringFieldReplacer{
	//		OutputFieldType: data.FieldTypeNullableFloat64,
	//		ReplaceFunc: func(in *string) (any, error) {
	//			if in == nil {
	//				return nil, nil
	//			}
	//			v, err := strconv.ParseFloat(*in, 64)
	//			if err != nil {
	//				return nil, err
	//			}
	//			return &v, nil
	//		},
	//	},
	//},
	//{
	//	Name:           "handle INT2",
	//	InputScanKind:  reflect.Interface,
	//	InputTypeName:  "INT2",
	//	ConversionFunc: func(in *string) (*string, error) { return in, nil },
	//	Replacer: &sqlutil.StringFieldReplacer{
	//		OutputFieldType: data.FieldTypeNullableInt16,
	//		ReplaceFunc: func(in *string) (any, error) {
	//			if in == nil {
	//				return nil, nil
	//			}
	//			i64, err := strconv.ParseInt(*in, 10, 16)
	//			if err != nil {
	//				return nil, err
	//			}
	//			v := int16(i64)
	//			return &v, nil
	//		},
	//	},
	//},
	return append(converters, strConverters...)
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
