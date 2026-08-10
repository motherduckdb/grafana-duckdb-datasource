package plugin

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mitchellh/mapstructure"

	duckdb "github.com/duckdb/duckdb-go/v2"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"
	"github.com/grafana/grafana-plugin-sdk-go/data/sqlutil"
	"github.com/grafana/sqlds/v3"
	"github.com/motherduckdb/grafana-duckdb-datasource/pkg/models"
)

type ConfigError struct {
	Msg string
}

func (e *ConfigError) Error() string {
	return e.Msg
}

type DuckDBDriver struct {
	mu          sync.Mutex
	Initialized bool
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

// Grafana 12.4 stopped forwarding its environment to plugin processes, so
// neither GF_PATHS_DATA nor HOME can be relied on to find a writable directory.
func duckDBDataDir(ctx context.Context, configured string) (string, error) {
	if configured != "" {
		if err := claimDataDir(configured); err != nil {
			return "", &ConfigError{"Data directory " + configured + " is not writable: " + err.Error()}
		}
		return configured, nil
	}

	candidates := []string{os.Getenv("GF_PATHS_DATA")}
	if cfg := backend.GrafanaConfigFromContext(ctx); cfg != nil {
		candidates = append(candidates, cfg.Get("GF_PATHS_DATA"))
	}
	if executable, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Dir(executable))
	}
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates, home)
	}
	candidates = append(candidates, os.TempDir())

	for _, dir := range candidates {
		if dir == "" || dir == "/" {
			continue
		}
		if err := claimDataDir(dir); err == nil {
			return dir, nil
		}
	}

	// Leave DuckDB to its own defaults rather than failing a data source that
	// may never install an extension or store a secret.
	return "", nil
}

// claimDataDir reports whether dir can hold DuckDB's directories. One that is
// already set up is accepted as is, so a read-only directory holding
// preinstalled extensions still wins over an empty writable one.
func claimDataDir(dir string) error {
	duckDir := filepath.Join(dir, ".duckdb")
	if info, err := os.Stat(duckDir); err == nil && info.IsDir() {
		return nil
	}
	return os.MkdirAll(duckDir, 0o755)
}

func (d *DuckDBDriver) Connect(ctx context.Context, settings backend.DataSourceInstanceSettings, msg json.RawMessage) (*sql.DB, error) {
	config, err := models.LoadPluginSettings(settings)
	if err != nil {
		return nil, err
	}

	// Determine connector path based on input
	var path string
	trimmedPath := strings.TrimSpace(config.Path)

	// Check for invalid path with quotes
	cleanPath := trimmedPath
	if (strings.HasPrefix(trimmedPath, "'") && strings.HasSuffix(trimmedPath, "'")) ||
		(strings.HasPrefix(trimmedPath, "\"") && strings.HasSuffix(trimmedPath, "\"")) {
		return nil, &ConfigError{"Invalid path: " + trimmedPath + " -> example input: md:sample_data"}
	}

	if strings.HasPrefix(cleanPath, "md:") {
		// MotherDuck: use in-memory base and ATTACH later
		if config.Secrets.MotherDuckToken == "" {
			return nil, &ConfigError{"MotherDuck Token is missing for motherduck connection"}
		}
		path = ""
	} else if trimmedPath != "" {
		// Local file: use the path directly as connector path
		path = trimmedPath
		backend.Logger.Info("Local file path is: " + path)
	} else {
		// Empty: in-memory database
		path = ""
	}
	dataDir, err := duckDBDataDir(ctx, strings.TrimSpace(config.DataDir))
	if err != nil {
		return nil, err
	}
	options := url.Values{}
	options.Set("custom_user_agent", "grafana")

	// These must be set when the database is opened. Setting them per connection
	// leaves extension autoloading resolving a home directory of its own.
	if dataDir != "" {
		backend.Logger.Info("DuckDB extensions and secrets directory is: " + dataDir)
		options.Set("home_directory", dataDir)
		options.Set("extension_directory", filepath.Join(dataDir, ".duckdb/extensions"))
		options.Set("secret_directory", filepath.Join(dataDir, ".duckdb/stored_secrets"))
	} else {
		backend.Logger.Warn("No writable directory found for DuckDB extensions and secrets, using DuckDB defaults")
	}

	sep := "?"
	if strings.Contains(path, "?") {
		sep = "&"
	}
	path += sep + options.Encode()

	// connect with the path before any other queries are run.
	connector, err := duckdb.NewConnector(path, func(execer driver.ExecerContext) error {
		d.mu.Lock()
		defer d.mu.Unlock()

		if !d.Initialized {
			bootQueries := []string{}

			// Handle MotherDuck setup and ATTACH
			if strings.HasPrefix(cleanPath, "md:") {
				// MotherDuck: install extension, set token, and ATTACH
				bootQueries = append(bootQueries, "INSTALL 'motherduck';", "LOAD 'motherduck';")
				bootQueries = append(bootQueries, "SET motherduck_token='"+config.Secrets.MotherDuckToken+"';")

				// Quote the MotherDuck path for ATTACH
				quotedDB := "'" + strings.ReplaceAll(cleanPath, "'", "''") + "'"
				bootQueries = append(bootQueries, "ATTACH IF NOT EXISTS "+quotedDB+" (TYPE motherduck);")
				backend.Logger.Info("ATTACH IF NOT EXISTS " + quotedDB + " (TYPE motherduck);")
			} else if config.Secrets.MotherDuckToken != "" {
				// Token provided but not MotherDuck path: still install extension for potential use
				bootQueries = append(bootQueries, "INSTALL 'motherduck';", "LOAD 'motherduck';")
				bootQueries = append(bootQueries, "SET motherduck_token='"+config.Secrets.MotherDuckToken+"';")
			}
			// Run other user defined init queries.
			if strings.TrimSpace(config.InitSql) != "" {
				bootQueries = append(bootQueries, config.InitSql)
			}

			for _, query := range bootQueries {
				// TODO: Fix context cancellation happening somewhere in the plugin.
				// Left unset on failure, so the next connection retries the setup
				// instead of quietly connecting to an unconfigured database.
				if _, err := execer.ExecContext(context.Background(), query, nil); err != nil {
					return err
				}
			}

			d.Initialized = true
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	db := sql.OpenDB(connector)

	maxOpen := 25
	if config.MaxOpenConns > 0 {
		maxOpen = config.MaxOpenConns
	}
	db.SetMaxOpenConns(maxOpen)

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
	return sqlutil.Macros{
		"timeFrom": macroTimeFrom,
		"timeTo":   macroTimeTo,
	}
}

func macroTimeFrom(query *sqlutil.Query, args []string) (string, error) {
	if len(args) == 0 || (len(args) == 1 && strings.TrimSpace(args[0]) == "") {
		return "'" + query.TimeRange.From.UTC().Format(time.RFC3339) + "'", nil
	}
	return "", fmt.Errorf("%w: expected 0 arguments, received %d", sqlutil.ErrorBadArgumentCount, len(args))
}

func macroTimeTo(query *sqlutil.Query, args []string) (string, error) {
	if len(args) == 0 || (len(args) == 1 && strings.TrimSpace(args[0]) == "") {
		return "'" + query.TimeRange.To.UTC().Format(time.RFC3339) + "'", nil
	}
	return "", fmt.Errorf("%w: expected 0 arguments, received %d", sqlutil.ErrorBadArgumentCount, len(args))
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

// NullBigInt is a wrapper for *big.Int that implements sql.Scanner
type NullBigInt struct {
	BigInt *big.Int
	Valid  bool
}

func (n *NullBigInt) Scan(value any) error {
	if value == nil {
		n.BigInt = nil
		n.Valid = false
		return nil
	}
	n.Valid = true
	bi, ok := value.(*big.Int)
	if !ok {
		backend.Logger.Info("got unexpected value not of big.Int type: %+v", value)
		n.BigInt = nil
		n.Valid = false
		return errors.New("expected value to be big.Int")
	}
	n.BigInt = bi
	return nil
}

func (n *NullBigInt) Value() (driver.Value, error) {
	if !n.Valid || n.BigInt == nil {
		return nil, nil
	}
	return n.BigInt, nil
}

func GetConverterList() []sqlutil.Converter {
	// NEED:
	// NULL to uint64, uint32, uint16, uint8,  not supported
	// Names: BIT, UBIGINT, UHUGEINT, UINTEGER, USMALLINT, UTINYINT

	// Add converter for HUGEINT that returns *big.Int
	bigIntConverters := []sqlutil.Converter{
		{
			Name:          "handle HUGEINT (returns *big.Int)",
			InputScanType: reflect.TypeOf(NullBigInt{}),
			InputTypeName: "HUGEINT",
			FrameConverter: sqlutil.FrameConverter{
				// There's no numerical FieldType that's big enough for HUGEINTs, so
				// output as string.
				FieldType: data.FieldTypeNullableString,
				ConverterFunc: func(in interface{}) (interface{}, error) {
					v := in.(*NullBigInt)
					if !v.Valid || v.BigInt == nil {
						return (*string)(nil), nil
					}
					str := v.BigInt.String()
					return &str, nil
				},
			},
		},
	}

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
	allConverters := append(bigIntConverters, converters...)
	return append(allConverters, strConverters...)
}
