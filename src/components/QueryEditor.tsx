import React from 'react';
import { QueryEditorProps } from '@grafana/data';
import { DuckDBDataSource } from '../datasource';
import { SqlQueryEditor, SQLQuery, SQLOptions } from '@grafana/plugin-ui';

export function DuckDBQueryEditor(props: QueryEditorProps<DuckDBDataSource, SQLQuery, SQLOptions>) {
  return <SqlQueryEditor {...props}/>;
}
