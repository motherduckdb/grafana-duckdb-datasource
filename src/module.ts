import { DataSourcePlugin } from '@grafana/data';
import { DuckDBDataSource } from './datasource';
import { ConfigEditor } from './components/ConfigEditor';
import { DuckDBQueryEditor } from './components/QueryEditor';
import { DuckDBDataSourceOptions, SecureJsonData } from './types';
import { SQLQuery } from '@grafana/plugin-ui';

export const plugin = new DataSourcePlugin<DuckDBDataSource, SQLQuery, DuckDBDataSourceOptions, SecureJsonData>(DuckDBDataSource)
  .setConfigEditor(ConfigEditor)
  .setQueryEditor(DuckDBQueryEditor);
