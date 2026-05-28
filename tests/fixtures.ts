import { PanelEditPage, test as base } from '@grafana/plugin-e2e';

const normalizeGrafanaVersion = (version: string | undefined) => {
  if (!version || !/^\d+\.\d+\.\d+/.test(version)) {
    return '';
  }

  return version.replace(/\+.*/, '').replace(/-.*/, '');
};

export const isGrafanaVersionAtLeast = (version: string | undefined, target: string) => {
  const currentParts = normalizeGrafanaVersion(version).split('.').map(Number);
  const targetParts = target.split('.').map(Number);

  if (currentParts.length < 3 || currentParts.some(Number.isNaN)) {
    return false;
  }

  for (let i = 0; i < 3; i += 1) {
    if (currentParts[i] > targetParts[i]) {
      return true;
    }
    if (currentParts[i] < targetParts[i]) {
      return false;
    }
  }

  return true;
};

/**
 * True when running against Grafana 13+, where the dashboard editor UI
 * changed and @grafana/plugin-e2e's panelEditPage fixture doesn't work yet.
 */
export const isGrafana13 = isGrafanaVersionAtLeast(process.env.GRAFANA_VERSION, '13.0.0');

/**
 * Extend the base test fixtures to auto-dismiss the "What's new in Grafana"
 * modal that appears in Grafana 13+ and blocks interaction with page elements.
 */
export const test = base.extend({
  panelEditPage: async ({ dashboardPage, grafanaVersion, page, request, selectors }, run, testInfo) => {
    if (!isGrafanaVersionAtLeast(grafanaVersion, '13.0.0')) {
      await run(await dashboardPage.addPanel());
      return;
    }

    await dashboardPage.goto();
    await page.getByRole('button', { name: 'Panel', exact: true }).click();
    await page.getByRole('textbox', { name: 'Title' }).waitFor();
    await page.getByRole('button', { name: 'Configure visualization' }).click();

    await run(new PanelEditPage({ page, selectors, grafanaVersion, request, testInfo }, { id: '1' }));
  },

  page: async ({ page }, run) => {
    const dialog = page.getByRole('dialog', { name: /What's new in Grafana/i });
    await page.addLocatorHandler(dialog, async () => {
      const closeButton = dialog.getByRole('button', { name: /Close/i });
      if (await closeButton.isVisible({ timeout: 1000 }).catch(() => false)) {
        await closeButton.click();
      } else {
        await page.keyboard.press('Escape');
      }
    });
    await run(page);
  },
});

export { expect } from '@grafana/plugin-e2e';
