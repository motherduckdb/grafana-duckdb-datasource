import React, { ChangeEvent } from 'react';
import { InlineField, Input, SecretInput } from '@grafana/ui';
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
        ...options.secureJsonFields,
        motherDuckToken: false,
      },
      secureJsonData: {
        ...options.secureJsonData,
        motherDuckToken: '',
      },
    });
  };

  return (
    <>
      <InlineField label="DB Path" labelWidth={14} interactive tooltip={'Json field returned to frontend'}>
        <Input
          id="config-editor-path"
          onChange={onPathChange}
          value={jsonData.path}
          placeholder="Enter the path to the duckdb file, or :memory: for in-memory database."
          width={40}
        />
      </InlineField>
      <InlineField label="MotherDuck Token" labelWidth={14} interactive tooltip={'Secure json field (backend only)'}>
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
