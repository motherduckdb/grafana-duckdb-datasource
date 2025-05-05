package plugin

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mitchellh/mapstructure"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"
	"github.com/grafana/grafana-plugin-sdk-go/data/sqlutil"
	"github.com/grafana/sqlds/v3"
	duckdb "github.com/marcboeker/go-duckdb/v2"
	"github.com/motherduckdb/grafana-duckdb-datasource/pkg/models"
)

type ConfigError struct {
	Msg string
}

func (e *ConfigError) Error() string {
	return e.Msg
}

type DuckDBDriver struct {
	HasSetMotherDuckToken bool
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

	if strings.HasPrefix(config.Path, "md:") && config.Secrets.MotherDuckToken == "" {
		return nil, &ConfigError{"MotherDuck Token is missing for motherduck connection"}
	}
	var path = config.Path
	if strings.HasPrefix(config.Path, "md:") && config.Secrets.MotherDuckToken != "" {
		path = config.Path + "?motherduck_token=" + config.Secrets.MotherDuckToken
	}

	connector, err := duckdb.NewConnector(path, func(execer driver.ExecerContext) error {
		bootQueries := []string{}

		// read env variable GF_PATHS_DATA and use it as the home directory for extension installation.
		homePath := os.Getenv("GF_PATHS_DATA")

		if homePath != "" {
			bootQueries = append(bootQueries, "SET home_directory='"+homePath+"';")
			extensionPath := filepath.Join(homePath, ".duckdb/extensions")
			bootQueries = append(bootQueries, "SET extension_directory='"+extensionPath+"';")
			secretsPath := filepath.Join(homePath, ".duckdb/stored_secrets")
			bootQueries = append(bootQueries, "SET secret_directory='"+secretsPath+"';")
		}

		if strings.HasPrefix(path, "md:") {
			bootQueries = append(bootQueries, "INSTALL 'motherduck';", "LOAD 'motherduck';")
		} else if config.Secrets.MotherDuckToken != "" {
			// Still need to install motherduck in order to set the config.
			bootQueries = append(bootQueries, "INSTALL 'motherduck';", "LOAD 'motherduck';")
			if !d.HasSetMotherDuckToken {
				bootQueries = append(bootQueries, "SET motherduck_token='"+config.Secrets.MotherDuckToken+"';")
				d.HasSetMotherDuckToken = true
			}
		}

		// Run other user defined init queries.
		if strings.TrimSpace(config.InitSql) != "" {
			bootQueries = append(bootQueries, config.InitSql)
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
	return append(converters, strConverters...)
}
