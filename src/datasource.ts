import { DataSourceInstanceSettings, CoreApp, ScopedVars } from '@grafana/data';
import { DataSourceWithBackend, getTemplateSrv } from '@grafana/runtime';

import { DuckDBQuery, DuckDBDataSourceOptions as DuckDBDataSourceOptions, DEFAULT_QUERY } from './types';

export class DuckDBDataSource extends DataSourceWithBackend<DuckDBQuery, DuckDBDataSourceOptions> {
  constructor(instanceSettings: DataSourceInstanceSettings<DuckDBDataSourceOptions>) {
    super(instanceSettings);

  }

  getDefaultQuery(_: CoreApp): Partial<DuckDBQuery> {
    return DEFAULT_QUERY;
  }

  applyTemplateVariables(query: DuckDBQuery, scopedVars: ScopedVars): Record<string, any> {
    return {
      ...query,
      queryText: getTemplateSrv().replace(query.queryText, scopedVars),
    };
  }

  filterQuery(query: DuckDBQuery): boolean {
    // if no query has been provided, prevent the query from being executed
    return !!query.queryText;
  }
}
