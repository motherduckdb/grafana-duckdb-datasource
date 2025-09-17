import { test, expect } from '@grafana/plugin-e2e';
import { DuckDBDataSourceOptions, SecureJsonData } from '../src/types';

test('"Save & test" should be successful when configuration is valid', async ({
  createDataSourceConfigPage,
  readProvisionedDataSource,
  page,
}) => {
  const ds = await readProvisionedDataSource<DuckDBDataSourceOptions, SecureJsonData>({ fileName: 'datasources.yml' });
  const configPage = await createDataSourceConfigPage({ type: ds.type });
  await page.getByRole('textbox', { name: 'Database name' }).fill("");
  await page.getByRole('textbox', { name: 'MotherDuck Token' }).fill("");
  await expect(configPage.saveAndTest()).toBeOK();
});

test('"Save & test" should fail when configuration is invalid', async ({
  createDataSourceConfigPage,
  readProvisionedDataSource,
  page,
}) => {
  const ds = await readProvisionedDataSource<DuckDBDataSourceOptions, SecureJsonData>({ fileName: 'datasources.yml' });
  const configPage = await createDataSourceConfigPage({ type: ds.type });
  await page.getByRole('textbox', { name: 'Database name' }).fill("md:");
  await page.getByRole('textbox', { name: 'MotherDuck Token' }).fill("");
  await expect(configPage.saveAndTest()).not.toBeOK();
  await expect(configPage).toHaveAlert('error', { hasText: 'MotherDuck Token is missing for motherduck connection' });
});
