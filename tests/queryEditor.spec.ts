import { test, expect } from '@grafana/plugin-e2e';

test('data query should return values 10 and 20', async ({ panelEditPage, readProvisionedDataSource, page }) => {
  const ds = await readProvisionedDataSource({ fileName: 'datasources.yml' });
  await panelEditPage.datasource.set(ds.name);
  await panelEditPage.getQueryEditorRow('A').getByRole("radiogroup").getByLabel("Code").click();
  await panelEditPage.getQueryEditorRow('A').getByLabel("Editor content;Press Alt+F1 for Accessibility Options.").fill('select 10 as val union select 20 as val');
  await panelEditPage.setVisualization('Table');
  await panelEditPage.getQueryEditorRow('A').getByLabel("Query editor Run button").click();

  // 12.2.0 or higher uses the new data grid
  const GrafanaVersion = (process.env.GRAFANA_VERSION || '');
  if (GrafanaVersion >= '12.2.0') {
    const grid = page.locator('[role="grid"]');
    await expect(grid).toContainText(['10']);
    await expect(grid).toContainText(['20']);
  } else {
    await expect(panelEditPage.panel.data).toContainText(['10']);
    await expect(panelEditPage.panel.data).toContainText(['20']);
  }
});
