import { test, expect } from '@grafana/plugin-e2e';

test('"Save & test" should be successful when configuration is valid', async ({
  createDataSourceConfigPage,
  page,
}) => {
  // const ds = await readProvisionedDataSource<DuckDBDataSourceOptions, MySecureJsonData>({ fileName: 'datasources.yml' });
  const configPage = await createDataSourceConfigPage({ type: "grafana-duckdb-datasource" });
  await page.getByRole('textbox', { name: 'DB Path' }).fill("");
  await page.getByRole('textbox', { name: 'MotherDuck Token' }).fill("");
  await expect(configPage.saveAndTest()).toBeOK();
});

test('"Save & test" should fail when configuration is invalid', async ({
  createDataSourceConfigPage,
  page,
}) => {
  // const ds = await readProvisionedDataSource<DuckDBDataSourceOptions, MySecureJsonData>({ fileName: 'datasources.yml' });
  const configPage = await createDataSourceConfigPage({ type: "grafana-duckdb-datasource" });
  await page.getByRole('textbox', { name: 'DB Path' }).fill("md:");
  await page.getByRole('textbox', { name: 'MotherDuck Token' }).fill("");
  await expect(configPage.saveAndTest()).not.toBeOK();
  await expect(configPage).toHaveAlert('error', { hasText: 'MotherDuck Token is missing for motherduck connection' });
});
