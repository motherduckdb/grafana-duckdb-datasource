import { test, expect } from '@grafana/plugin-e2e';
import fs from 'fs';
import path from 'path';
import { execSync } from 'child_process';


test('data query should return values 10 and 20', async ({ panelEditPage, readProvisionedDataSource, page }) => {
  const ds = await readProvisionedDataSource({ fileName: 'datasources.yml' });
  await panelEditPage.datasource.set(ds.name);
  await panelEditPage.getQueryEditorRow('A').getByRole("radiogroup").getByLabel("Code").click();
  await panelEditPage.getQueryEditorRow('A').getByLabel("Editor content;Press Alt+F1 for Accessibility Options.").fill('select 10 as val union select 20 as val');
  await panelEditPage.setVisualization('Table');
  await panelEditPage.getQueryEditorRow('A').getByLabel("Query editor Run button").click();

  const isNewGrafana = (process.env.GRAFANA_VERSION || '').includes('12.3.0');
  if (isNewGrafana) {
    const grid = page.locator('[role="grid"]');
    await expect(grid).toContainText(['10']);
    await expect(grid).toContainText(['20']);
  } else {
    await expect(panelEditPage.panel.data).toContainText(['10']);
    await expect(panelEditPage.panel.data).toContainText(['20']);
  }
});


test('duckdb db file test', async ({ panelEditPage, readProvisionedDataSource, createDataSourceConfigPage, page }) => {
  const projectRoot = path.resolve(__dirname, '..');
  const dataDir = path.join(projectRoot, 'data');
  const dbPath = path.join(dataDir, 'e2e.duckdb');
  const dockerDbPath = '/root/motherduck-duckdb-datasource/data/e2e.duckdb';

  fs.mkdirSync(dataDir, { recursive: true });
  if (fs.existsSync(dbPath)) {
    fs.rmSync(dbPath);
  }

  // Create DB and seed initial data
  execSync(`duckdb "${dbPath}" -c "CREATE OR REPLACE TABLE numbers(val INTEGER); INSERT INTO numbers VALUES (10),(20);"`);

  // Configure the datasource to point at this DuckDB file
  const ds = await readProvisionedDataSource({ fileName: 'datasources.yml' });
  const configPage = await createDataSourceConfigPage({ type: ds.type });
  await page.getByRole('textbox', { name: 'Database name' }).fill(dockerDbPath);
  await page.getByRole('textbox', { name: 'MotherDuck Token' }).fill('');
  await expect(configPage.saveAndTest()).toBeOK();

});

// motherduck database test
test('motherduck database test', async ({ panelEditPage, readProvisionedDataSource, createDataSourceConfigPage, page }) => {
  const motherDuckToken = process.env.MOTHERDUCK_TOKEN;
  if (!motherDuckToken) {
    console.log('Skipping MotherDuck test: MOTHERDUCK_TOKEN environment variable not set');
    return;
  }

  const ds = await readProvisionedDataSource({ fileName: 'datasources.yml' });
  const configPage = await createDataSourceConfigPage({ type: ds.type });
  await page.getByRole('textbox', { name: 'Database name' }).fill('md:sample_data');
  await page.getByRole('textbox', { name: 'MotherDuck Token' }).fill(motherDuckToken);
  await expect(configPage.saveAndTest()).toBeOK();
});
