import { test, expect } from '@grafana/plugin-e2e';
import { setVisualization } from './helpers';

test('simple aggregate query like select sum(1) should work', async ({ panelEditPage, readProvisionedDataSource, page }) => {
  const ds = await readProvisionedDataSource({ fileName: 'datasources.yml' });
  await panelEditPage.datasource.set(ds.name);
  await panelEditPage.getQueryEditorRow('A').getByRole("radiogroup").getByLabel("Code").click();
  await panelEditPage.getQueryEditorRow('A').getByLabel("Editor content;Press Alt+F1 for Accessibility Options.").fill('select sum(1)');

  await setVisualization(page, panelEditPage, 'Table');
  await panelEditPage.getQueryEditorRow('A').getByLabel("Query editor Run button").click();

  const grafanaVersion = process.env.GRAFANA_VERSION || '';
  if (grafanaVersion >= '12.2.') {
    const grid = page.locator('[role="grid"]');
    await expect(grid).toContainText(['1']);
  } else {
    await expect(panelEditPage.panel.data).toContainText(['1']);
  }
});
