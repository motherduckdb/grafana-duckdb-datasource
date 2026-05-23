import { test, expect, isGrafanaVersionAtLeast } from './fixtures';
import { setVisualization } from './helpers';

test('data query should return values 10 and 20', async ({ panelEditPage, readProvisionedDataSource, page, grafanaVersion }) => {
  const ds = await readProvisionedDataSource({ fileName: 'datasources.yml' });
  await panelEditPage.datasource.set(ds.name);
  await panelEditPage.getQueryEditorRow('A').getByRole("radiogroup").getByLabel("Code").click();
  await panelEditPage.getQueryEditorRow('A').getByLabel("Editor content;Press Alt+F1 for Accessibility Options.").fill('select 10 as val union select 20 as val');

  await setVisualization(page, panelEditPage, 'Table', grafanaVersion);
  await panelEditPage.getQueryEditorRow('A').getByLabel("Query editor Run button").click();

  if (isGrafanaVersionAtLeast(grafanaVersion, '12.2.0')) {
    const grid = page.locator('[role="grid"]');
    await expect(grid).toContainText(['10']);
    await expect(grid).toContainText(['20']);
  } else {
    await expect(panelEditPage.panel.data).toContainText(['10']);
    await expect(panelEditPage.panel.data).toContainText(['20']);
  }
});
