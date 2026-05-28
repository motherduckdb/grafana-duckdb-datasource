import { test, expect } from '@grafana/plugin-e2e';
import { setVisualization } from './helpers';

test('$__time macros expand and filter correctly', async ({ panelEditPage, readProvisionedDataSource, page }) => {
  // Note: this test depends on Grafana's default time range (last 6 hours).
  const ds = await readProvisionedDataSource({ fileName: 'datasources.yml' });
  await panelEditPage.datasource.set(ds.name);
  await panelEditPage.getQueryEditorRow('A').getByRole('radiogroup').getByLabel('Code').click();

  const query = `
    WITH samples(ts, label) AS ( VALUES 
      (now() - INTERVAL '30 minutes', 'in_range'),
      (now() - INTERVAL '2 day', 'out_of_range_past'),
      (now() + INTERVAL '2 day', 'out_of_range_future')
    )
    SELECT
      label,
      $__timeFrom() as time_from,
      $__timeTo()   as time_to
    FROM samples
    WHERE $__timeFilter(ts)
  `;

  await panelEditPage.getQueryEditorRow('A').getByLabel('Editor content;Press Alt+F1 for Accessibility Options.').fill(query);

  await setVisualization(page, panelEditPage, 'Table');
  await panelEditPage.getQueryEditorRow('A').getByLabel('Query editor Run button').click();

  const grafanaVersion = process.env.GRAFANA_VERSION || '';

  const checkResults = async (locator: any) => {
    // Basic presence/absence checks
    await expect(locator).toContainText(['in_range']);
    await expect(locator).not.toContainText(['out_of_range_past']);
    await expect(locator).not.toContainText(['out_of_range_future']);

    // Concatenate all text and check for timestamp (YYYY-MM-DD...Z).
    const texts = (await locator.allTextContents()).join(' ');
    const tsMatches = texts.match(/\d{4}-\d{2}-\d{2}.*?Z/g) || [];
    // Require at least two timestamps (time_from and time_to).
    expect(tsMatches.length).toBeGreaterThanOrEqual(2);
  };

  if (grafanaVersion >= '12.2.') {
    const grid = page.locator('[role="grid"]');
    await checkResults(grid);
  } else {
    await checkResults(panelEditPage.panel.data);
  }
});
