import { test, expect } from '@grafana/plugin-e2e';
import { DuckDBDataSourceOptions, SecureJsonData } from '../src/types';
import { execSync } from 'child_process';
import path from 'path';
import fs from 'fs';

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

test('"Max Connections" field should be saved and persisted', async ({
  createDataSourceConfigPage,
  readProvisionedDataSource,
  page,
}) => {
  const ds = await readProvisionedDataSource<DuckDBDataSourceOptions, SecureJsonData>({ fileName: 'datasources.yml' });
  const configPage = await createDataSourceConfigPage({ type: ds.type });

  // Fill in required fields
  await page.getByRole('textbox', { name: 'Database name' }).fill('');
  await page.getByRole('textbox', { name: 'MotherDuck Token' }).fill('');

  // Set Max Connections value
  const maxConnsInput = page.getByRole('spinbutton', { name: 'Max Connections' });
  await expect(maxConnsInput).toBeVisible();
  await maxConnsInput.fill('10');
  await expect(maxConnsInput).toHaveValue('10');

  // Save
  await expect(configPage.saveAndTest()).toBeOK();

  // Reload the page and verify the value is persisted
  await page.reload();
  await expect(page.getByRole('spinbutton', { name: 'Max Connections' })).toHaveValue('10');
});
