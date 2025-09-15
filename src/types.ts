import { SQLOptions } from '@grafana/plugin-ui';


// export interface DuckDBQuery extends SQLQuery {
//   queryText?: string;
//   constant: number;
// }

// export const DEFAULT_QUERY: Partial<DuckDBQuery> = {
//   constant: 6.5,
// };

// export interface DataPoint {
//   Time: number;
//   Value: number;
// }

// export interface DataSourceResponse {
//   datapoints: DataPoint[];
// }

/**
 * These are options configured for each DataSource instance
 */
export interface DuckDBDataSourceOptions extends SQLOptions {
  path?: string;
  initSql?: string;
  databaseName?: string;
}

/**
 * Value that is used in the backend, but never sent over HTTP to the frontend
 */
export interface SecureJsonData {
  motherDuckToken?: string;
}
