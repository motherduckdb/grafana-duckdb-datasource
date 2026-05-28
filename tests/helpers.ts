import { Page } from '@playwright/test';

/**
 * Workaround for Grafana 12.4.0+ where the visualization picker changed.
 * TODO: Remove once @grafana/plugin-e2e PR #2399 is released
 */
export async function setVisualization(page: Page, panelEditPage: any, vizName: string): Promise<void> {
  const grafanaVersion = process.env.GRAFANA_VERSION || '';
  if (grafanaVersion >= '12.4.') {
    // Check if viz picker is already open (it opens automatically for new panels in 12.4.0+)
    const vizItem = page.getByTestId(`data-testid Plugin visualization item ${vizName}`);
    if (!await vizItem.isVisible({ timeout: 500 }).catch(() => false)) {
      // Picker not open or item not visible - click toggle and switch to All visualizations tab
      await page.getByTestId('data-testid toggle-viz-picker').click();
      const allVizTab = page.getByRole('tab', { name: 'All visualizations' });
      if (await allVizTab.isVisible({ timeout: 1000 }).catch(() => false)) {
        await allVizTab.click();
      }
    }
    await vizItem.click();
  } else {
    await panelEditPage.setVisualization(vizName);
  }
}
