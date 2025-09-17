import React, { ChangeEvent } from 'react';
import { InlineField, Input, SecretInput, TextArea } from '@grafana/ui';
import { DataSourcePluginOptionsEditorProps } from '@grafana/data';
import { DuckDBDataSourceOptions, SecureJsonData } from '../types';

interface Props extends DataSourcePluginOptionsEditorProps<DuckDBDataSourceOptions, SecureJsonData> {}

export function ConfigEditor(props: Props) {
  const { onOptionsChange, options } = props;
  const { jsonData, secureJsonFields, secureJsonData } = options;

  const onPathChange = (event: ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({
      ...options,
      jsonData: {
        ...jsonData,
        path: event.target.value,
      },
    });
  };

  const onInitSqlChange = (event: ChangeEvent<HTMLTextAreaElement>) => {
    onOptionsChange({
      ...options,
      jsonData: {
        ...jsonData,
        initSql: event.target.value,
      },
    });
  };


  // Secure field (only sent to the backend)
  const onMotherDuckTokenChange = (event: ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({
      ...options,
      secureJsonData: {
        motherDuckToken: event.target.value,
      },
    });
  };

  const onResetMotherDuckToken = () => {
    onOptionsChange({
      ...options,
      secureJsonFields: {
        motherDuckToken: false,
      },
      secureJsonData: {
        motherDuckToken: '',
      },
    });
  };

  return (
    <>
      <InlineField label="Database name" labelWidth={20} interactive tooltip={'path to DuckDB file or MotherDuck database string'}>
        <Input
          id="config-editor-path"
          onChange={onPathChange}
          value={jsonData.path}
          placeholder="Enter the MotherDuck database string or path to the duckdb file"
          width={40}
        />
      </InlineField>
      <InlineField label="Init SQL" labelWidth={20} interactive
                     tooltip={'(Optional) SQL to run when connection is established'}>
          <TextArea
              id="config-editor-init-sql"
              onChange={onInitSqlChange}
              value={jsonData.initSql || ''}
              placeholder="e.g. INSTALL 'httpfs'; LOAD 'httpfs';"
              width={60}
              rows={5}
          />
      </InlineField>
      <InlineField label="MotherDuck Token" labelWidth={20} interactive tooltip={'MotherDuck Token'}>
        <SecretInput
          required
          id="config-editor-md-token"
          isConfigured={secureJsonFields.motherDuckToken}
          value={secureJsonData?.motherDuckToken}
          placeholder="Enter your MotherDuck token"
          width={40}
          onReset={onResetMotherDuckToken}
          onChange={onMotherDuckTokenChange}
        />
      </InlineField>


    </>
  );
}
