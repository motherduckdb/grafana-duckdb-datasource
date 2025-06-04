import {  uniqBy } from 'lodash';
// @ts-ignore
import sqlFormatter from 'sql-formatter-plus';
import { DataSourceInstanceSettings, ScopedVars, DataFrame, MetricFindValue, DataQueryRequest, TimeRange } from '@grafana/data';
import { TemplateSrv, HealthCheckError, HealthStatus } from '@grafana/runtime';
import { Aggregate, DB, ResponseParser, SQLOptions, SQLQuery, SQLSelectableValue, SqlDatasource, SqlQueryModel, LanguageDefinition, QueryFormat } from '@grafana/plugin-ui';
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

// Duplicated from https://github.com/grafana/plugin-ui/blob/main/src/datasource/SqlDatasource.ts

export interface SearchFilterOptions {
  searchFilter?: string;
}
interface MetricFindQueryOptions extends SearchFilterOptions {
  range?: TimeRange;
  variable?: { name: string };
}

const SEARCH_FILTER_VARIABLE = '__searchFilter';

const containsSearchFilter = (query: string | unknown): boolean =>
  query && typeof query === 'string' ? query.indexOf(SEARCH_FILTER_VARIABLE) !== -1 : false;

const getSearchFilterScopedVar = (args: {
  query: string;
  wildcardChar: string;
  options?: SearchFilterOptions;
}): ScopedVars => {
  const { query, wildcardChar } = args;
  if (!containsSearchFilter(query)) {
    return {};
  }

  let { options } = args;

  options = options || { searchFilter: '' };
  const value = options.searchFilter ? `${options.searchFilter}${wildcardChar}` : `${wildcardChar}`;

  return {
    __searchFilter: {
      value,
      text: '',
    },
  };
};

export function formatSQL(q: string) {
  return sqlFormatter.format(q).replace(/(\$ \{ .* \})|(\$ __)|(\$ \w+)/g, (m: string) => {
    return m.replace(/\s/g, '');
  });
}


interface CompletionProviderGetterArgs {
  getColumns: React.MutableRefObject<(t: SQLQuery) => Promise<ColumnDefinition[]>>;
  getTables: React.MutableRefObject<(d?: string) => Promise<TableDefinition[]>>;
}


function showTablesQuery() {
  return `select array_to_string(array_value(table_catalog, table_schema, table_name), '.') as "table" from information_schema.tables;`;
}


function getSchemaQuery(table: string, order?: boolean) {
  // we will put table-name between single-quotes, so we need to escape single-quotes
  // in the table-name
  const tableNamePart = "'" + (table.split(".").at(2)?.replace(/'/g, "''") || '') + "'";
  const orderByPart = order ? ' order by column_name' : '';
  return `select column_name as "column", data_type as "type"
    from information_schema.columns
    where table_name = ${tableNamePart} ${orderByPart};`;
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
  sqlLanguageDefinition: LanguageDefinition | undefined = undefined;

  query(request: DataQueryRequest<SQLQuery>) {;
    const result = super.query(request);
    return result;
  }

  async fetchTables(): Promise<string[]> {
    const tables = await this.runSql<{ table: string[] }>(showTablesQuery(), { refId: 'tables' });
    console.log("fetched tables", tables);
    return tables.fields.table?.values.flat() ?? [];
  }

  async fetchFields(query: SQLQuery, order?: boolean): Promise<SQLSelectableValue[]> {
    const { table } = query;
    console.log("fetching fields for table", table);
    if (table === undefined) {
      // if no table-name, we are not able to query for fields
      return [];
    }
    const schema = await this.runSql<{ column: string; type: string }>(getSchemaQuery(table, order), { refId: 'columns' });
    const result: SQLSelectableValue[] = [];
    for (let i = 0; i < schema.length; i++) {
      const column = schema.fields.column.values[i];
      const type = schema.fields.type.values[i];
      result.push({ label: column, value: column, type, ...getFieldConfig(type) });
    }
    return result;
  }


  // Tweaked the implementation from SqlDataSource, this makes sure the refIds are distinct 
  // so that queries for multiple query variables do not step on each other. 
  // TODO: contribute the fix to @grafana/plugin-ui 
  async metricFindQuery(query: string, options?: MetricFindQueryOptions): Promise<MetricFindValue[]> {
    const rawSql = this.templateSrv.replace(
      query,
      getSearchFilterScopedVar({ query, wildcardChar: '%', options: options }),
      this.interpolateVariable
    );

    let refId = 'tempvar';
    if (options && options.variable && options.variable.name) {
      refId = options.variable.name;
    }

    const interpolatedQuery: SQLQuery = {
      refId: refId,
      datasource: this.getRef(),
      rawSql,
      format: QueryFormat.Table,
    };

    const response = await (this as any).runMetaQuery(interpolatedQuery, options);
    return this.getResponseParser().transformMetricFindResponse(response);
  }
  
  getSqlLanguageDefinition(db: DB): LanguageDefinition {
    if (this.sqlLanguageDefinition !== undefined) {
      return this.sqlLanguageDefinition;
    }

    const args = {
      getColumns: { current: (query: SQLQuery) => fetchColumns(db, query) },
      getTables: { current: () => fetchTables(db) },
    };
    this.sqlLanguageDefinition = {
      id: 'pgsql',
      completionProvider: getSqlCompletionProvider(args),
      formatter: formatSQL,
    };
    return this.sqlLanguageDefinition;
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
      datasets: () => Promise.resolve(["default"]),
      tables: (dataset) => this.fetchTables(),
      fields: async (query: SQLQuery, order?: boolean) => {
        if (!query?.table) {
          return [];
        }
        return this.fetchFields(query, order);
      },
      // @ts-ignore
      getEditorLanguageDefinition: () => this.getSqlLanguageDefinition(this.db),
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
