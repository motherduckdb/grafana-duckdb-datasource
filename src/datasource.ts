import {  uniqBy } from 'lodash';
import { DataSourceInstanceSettings, ScopedVars, DataFrame, MetricFindValue, DataQueryRequest} from '@grafana/data';
import { TemplateSrv, HealthCheckError, HealthStatus } from '@grafana/runtime';
import { Aggregate, DB, ResponseParser, SQLOptions, SQLQuery, SQLSelectableValue, SqlDatasource, SqlQueryModel } from '@grafana/plugin-ui';
import { applyQueryDefaults } from './queryDefaults';
import { VariableFormatID } from '@grafana/schema';
import { getFieldConfig, toRawSql } from './sqlUtil';

import {
  ColumnDefinition,
  getStandardSQLCompletionProvider,
  LanguageCompletionProvider,
  TableDefinition,
  TableIdentifier,
} from '@grafana/experimental';

interface CompletionProviderGetterArgs {
  getColumns: React.MutableRefObject<(t: SQLQuery) => Promise<ColumnDefinition[]>>;
  getTables: React.MutableRefObject<(d?: string) => Promise<TableDefinition[]>>;
}


function buildSchemaConstraint() {
  // quote_ident protects hyphenated schemes
  return `
          quote_ident(table_schema) IN (
          SELECT
            CASE WHEN trim(s[i]) = '"$user"' THEN user ELSE trim(s[i]) END
          FROM
            generate_series(
              array_lower(string_to_array(current_setting('search_path'),','),1),
              array_upper(string_to_array(current_setting('search_path'),','),1)
            ) as i,
            string_to_array(current_setting('search_path'),',') s
          )`;
}

function showTables() {
  return `select quote_ident(table_name) as "table" from information_schema.tables
    where quote_ident(table_schema) not in ('information_schema',
                             'pg_catalog',
                             '_timescaledb_cache',
                             '_timescaledb_catalog',
                             '_timescaledb_internal',
                             '_timescaledb_config',
                             'timescaledb_information',
                             'timescaledb_experimental')
      and ${buildSchemaConstraint()}`;
}


function getSchema(table: string) {
  // we will put table-name between single-quotes, so we need to escape single-quotes
  // in the table-name
  const tableNamePart = "'" + table.replace(/'/g, "''") + "'";

  return `select quote_ident(column_name) as "column", data_type as "type"
    from information_schema.columns
    where quote_ident(table_name) = ${tableNamePart};
    `;
}



const getSqlCompletionProvider: (args: CompletionProviderGetterArgs) => LanguageCompletionProvider =
  ({ getColumns, getTables }) =>
  (monaco, language) => ({
    ...(language && getStandardSQLCompletionProvider(monaco, language)),
    tables: {
      resolve: async () => {
        return await getTables.current();
      },
    },
    columns: {
      resolve: async (t?: TableIdentifier) => {
        return await getColumns.current({ table: t?.table, refId: 'A' });
      },
    },
  });

async function fetchColumns(db: DB, q: SQLQuery) {
  const cols = await db.fields(q);
  if (cols.length > 0) {
    return cols.map((c) => {
      return { name: c.value, type: c.value, description: c.value };
    });
  } else {
    return [];
  }
}

async function fetchTables(db: DB) {
  const tables = await db.lookup?.();
  return tables || [];
}



export class DuckDBQueryModel implements SqlQueryModel {
  target: SQLQuery;
  templateSrv?: TemplateSrv;
  scopedVars?: ScopedVars;

  constructor(target?: SQLQuery, templateSrv?: TemplateSrv, scopedVars?: ScopedVars) {
    this.target = applyQueryDefaults(target || { refId: 'A' });
    this.templateSrv = templateSrv;
    this.scopedVars = scopedVars;
  }

  interpolate() {
    return this.templateSrv?.replace(this.target.rawSql, this.scopedVars, VariableFormatID.SQLString) || '';
  }

  quoteLiteral(value: string) {
    return "'" + value.replace(/'/g, "''") + "'";
  }
}


export class DuckDBResponseParser implements ResponseParser {
  transformMetricFindResponse(frame: DataFrame): MetricFindValue[] {
    const values: MetricFindValue[] = [];
    const textField = frame.fields.find((f) => f.name === '__text');
    const valueField = frame.fields.find((f) => f.name === '__value');

    if (textField && valueField) {
      for (let i = 0; i < textField.values.length; i++) {
        values.push({ text: '' + textField.values[i], value: '' + valueField.values[i] });
      }
    } else {
      for (const field of frame.fields) {
        for (const value of field.values) {
          values.push({ text: value });
        }
      }
    }

    return uniqBy(values, 'text');
  }
}

async function functions(): Promise<Aggregate[]> {
  // Define the Aggregate objects
  const aggregates: Aggregate[] = [
      { id: '1', name: 'COUNT', description: 'Counts the number of rows' },
      { id: '2', name: 'SUM', description: 'Calculates the sum' },
      { id: '3', name: 'AVG', description: 'Calculates the average' },
      { id: '4', name: 'MIN', description: 'Calculates the min' },
      { id: '5', name: 'MAX', description: 'Calculates the max' },
  ];

  // Return the aggregates as a Promise
  return Promise.resolve(aggregates);
}


export class DuckDBDataSource extends SqlDatasource {
  query(request: DataQueryRequest<SQLQuery>) {
    console.log('EEEEEEEEE DuckDBDataSource.query=', request);
    const result = super.query(request);
    return result;
  }


  async fetchTables(): Promise<string[]> {
    const tables = await this.runSql<{ table: string[] }>(showTables(), { refId: 'tables' });
    return tables.fields.table?.values.flat() ?? [];
  }

  async fetchFields(query: SQLQuery): Promise<SQLSelectableValue[]> {
    const { table } = query;
    if (table === undefined) {
      // if no table-name, we are not able to query for fields
      return [];
    }
    const schema = await this.runSql<{ column: string; type: string }>(getSchema(table), { refId: 'columns' });
    const result: SQLSelectableValue[] = [];
    for (let i = 0; i < schema.length; i++) {
      const column = schema.fields.column.values[i];
      const type = schema.fields.type.values[i];
      result.push({ label: column, value: column, type, ...getFieldConfig(type) });
    }
    return result;
  }

  getDB(): DB {
    if (this.db !== undefined) {
      return this.db;
    }

    const args = {
      getColumns: { current: (query: SQLQuery) => fetchColumns(this.db, query) },
      getTables: { current: () => fetchTables(this.db) },
    };

    return {
      init: () => Promise.resolve(true),
      datasets: () => Promise.resolve([]),
      tables: () => this.fetchTables(),
      fields: async (query: SQLQuery) => {
        if (!query?.table) {
          return [];
        }
        return this.fetchFields(query);
      },
      validateQuery: (query) =>
        Promise.resolve({ isError: false, isValid: true, query, error: '', rawSql: query.rawSql }),
      dsID: () => this.id,
      toRawSql,
      lookup: async () => {
        const tables = await this.fetchTables();
        return tables.map((t) => ({ name: t, completion: t }));
      },
      getSqlCompletionProvider: () =>  getSqlCompletionProvider(args),
      functions,
      labels: new Map(),
    };
  }

  getResponseParser(): ResponseParser {
    return new DuckDBResponseParser();
  }

  getQueryModel(target?: SQLQuery, templateSrv?: TemplateSrv, scopedVars?: ScopedVars): DuckDBQueryModel {
    return new DuckDBQueryModel(target, templateSrv, scopedVars);
  }

  testDatasource(): Promise<{ status: string; message: string }> {
    return this.callHealthCheck().then((res) => {
      if (res.status === HealthStatus.OK) {
        return {
          status: 'success',
          message: res.message,
        };
      }

      return Promise.reject({
        status: 'error',
        message: res.message,
        error: new HealthCheckError(res.message, res.details),
      });
    });
  }


  
  constructor(instanceSettings: DataSourceInstanceSettings<SQLOptions>) {
    super(instanceSettings);
  }

}
