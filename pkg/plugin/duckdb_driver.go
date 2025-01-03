package plugin

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
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
	"github.com/motherduckdb/grafana-duckdb-datasource/pkg/models"
)

type DuckDBDriver struct {
}

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

	if config.Secrets.MotherDuckToken != "" {
		os.Setenv("motherduck_token", config.Secrets.MotherDuckToken)
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

	connector, err := duckdb.NewConnector(config.Path, func(execer driver.ExecerContext) error {

		bootQueries := []string{
			"INSTALL 'motherduck'",
			"LOAD 'motherduck'",
		}

		// read env variable GF_PATHS_HOME
		homePath := os.Getenv("GF_PATHS_HOME")
		if homePath != "" {
			bootQueries = append(bootQueries, "SET home_directory='"+homePath+"'")
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

// From https://github.com/snakedotdev/grafana-duckdb-datasource
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
