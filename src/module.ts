import { DataSourcePlugin } from '@grafana/data';
import { DuckDBDataSource } from './datasource';
import { ConfigEditor } from './components/ConfigEditor';
import { QueryEditor } from './components/QueryEditor';
import { DuckDBQuery, DuckDBDataSourceOptions } from './types';

export const plugin = new DataSourcePlugin<DuckDBDataSource, DuckDBQuery, DuckDBDataSourceOptions>(DuckDBDataSource)
  .setConfigEditor(ConfigEditor)
  .setQueryEditor(QueryEditor);
