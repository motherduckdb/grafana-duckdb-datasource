import { test, expect } from '@grafana/plugin-e2e';

test('data source supports multiple query variables in one dashboard', async ({
    readProvisionedDashboard,
    selectors,
    gotoDashboardPage
  }) => {
    const dashboard = await readProvisionedDashboard({fileName: 'variables.json' });
    const dashboardPage = await gotoDashboardPage(dashboard);

    const query0Label = await dashboardPage.getByGrafanaSelector(
        selectors.pages.Dashboard.SubMenu.submenuItemLabels("query0")
      );
    const query0DropdownFromLabel = await query0Label.locator('~ * [data-testid*="DropDown"]').first();
    await expect(query0DropdownFromLabel).toHaveText('1');
      

    const query1Label = await dashboardPage.getByGrafanaSelector(
        selectors.pages.Dashboard.SubMenu.submenuItemLabels("species")
      );
    const query1DropdownFromLabel = await query1Label.locator('~ * [data-testid*="DropDown"]').first();
    await expect(query1DropdownFromLabel).toHaveText('duck');
      
  });
