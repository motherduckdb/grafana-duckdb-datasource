import { test, expect } from '@grafana/plugin-e2e';

const PLUGIN_ID = 'motherduck-duckdb-datasource';

// Grafana's annotation data source picker filters on meta.annotations, which
// comes straight from plugin.json. Without it the data source is never offered,
// regardless of what the backend can answer.
test('advertises annotation support', async ({ page }) => {
  const settings = await (await page.request.get('/api/frontend/settings')).json();
  const ds: any = Object.values(settings.datasources).find((d: any) => d.type === PLUGIN_ID);

  expect(ds).toBeDefined();
  expect(ds.meta.annotations).toBe(true);
});

test('answers an annotation query with time, text and tags', async ({ page, readProvisionedDataSource }) => {
  const ds = await readProvisionedDataSource({ fileName: 'datasources.yml' });

  const response = await page.request.post('/api/ds/query', {
    data: {
      queries: [
        {
          refId: 'anno',
          // uid is numeric in datasources.yml, but Grafana identifies data sources by string.
          datasource: { type: ds.type, uid: String(ds.uid) },
          rawSql: "SELECT now() AS time, 'deploy' AS text, 'ci,prod' AS tags",
          format: 1, // sqlutil.FormatOptionTable
        },
      ],
      from: 'now-1h',
      to: 'now',
    },
  });
  expect(response.status(), await response.text()).toBe(200);

  const frame = (await response.json()).results.anno.frames[0];
  expect(frame.schema.fields.map((f: any) => f.name)).toEqual(['time', 'text', 'tags']);
  expect(frame.schema.fields[0].type).toBe('time');
});
