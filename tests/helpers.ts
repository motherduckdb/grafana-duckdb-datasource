import { Page } from '@playwright/test';
import { isGrafanaVersionAtLeast } from './fixtures';

/**
 * Workaround for Grafana 12.4.0+ where the visualization picker changed.
 * TODO: Remove once @grafana/plugin-e2e PR #2399 is released
 */
export async function setVisualization(page: Page, panelEditPage: any, vizName: string, grafanaVersion: string): Promise<void> {
  if (isGrafanaVersionAtLeast(grafanaVersion, '12.4.0')) {
    // Check if viz picker is already open (it opens automatically for new panels in 12.4.0+)
    const vizItem = page.getByTestId(`data-testid Plugin visualization item ${vizName}`);
    const vizButton = page.getByRole('button', { name: vizName, exact: true }).last();
    if (await vizButton.isVisible({ timeout: 500 }).catch(() => false)) {
      await vizButton.click();
      return;
    }

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

export async function dismissWhatsNewModal(page: Page): Promise<void> {
  const dialog = page.getByRole('dialog', { name: /What's new in Grafana/i });
  if (await dialog.isVisible({ timeout: 1000 }).catch(() => false)) {
    const closeButton = dialog.getByRole('button', { name: /Close/i });
    if (await closeButton.isVisible({ timeout: 1000 }).catch(() => false)) {
      await closeButton.click();
    } else {
      await page.keyboard.press('Escape');
    }
  }
}
