import React, { ChangeEvent } from 'react';
import { DataSourcePluginOptionsEditorProps } from '@grafana/data';
import { Combobox, Field, Input, SecretInput, FieldSet } from '@grafana/ui';

import { MerakiDSOpts, MerakiSecureOpts } from '../types';

type Props = DataSourcePluginOptionsEditorProps<MerakiDSOpts, MerakiSecureOpts>;

const DEFAULT_URL = 'https://api.meraki.com/api/v1';

// Each Meraki region has its own API host, and the global host refuses
// organizations from other regions.
const REGIONS = [
  { label: 'Global (api.meraki.com)',             value: DEFAULT_URL },
  { label: 'Canada (api.meraki.ca)',              value: 'https://api.meraki.ca/api/v1' },
  { label: 'China (api.meraki.cn)',               value: 'https://api.meraki.cn/api/v1' },
  { label: 'India (api.meraki.in)',               value: 'https://api.meraki.in/api/v1' },
  { label: 'US Government (api.gov-meraki.com)',  value: 'https://api.gov-meraki.com/api/v1' },
];

export function ConfigEditor({ options, onOptionsChange }: Props) {
  const { jsonData, secureJsonFields, secureJsonData } = options;

  function patchJson(key: keyof MerakiDSOpts, value: string) {
    onOptionsChange({ ...options, jsonData: { ...jsonData, [key]: value } });
  }

  return (
    <FieldSet label="Connection">
      <Field
        label="Region"
        description="The Meraki region your dashboard is in. Pick a custom URL for a proxy or a test server."
      >
        <Combobox
          id="cfg-base-url"
          width={50}
          options={REGIONS}
          value={jsonData?.baseUrl || DEFAULT_URL}
          createCustomValue
          onChange={opt => patchJson('baseUrl', opt?.value || DEFAULT_URL)}
        />
      </Field>

      <Field label="Default organization ID" description="Queries use this organization unless they pick another one the API key can see. Find it under Organization → Settings." required>
        <Input
          id="cfg-org-id"
          width={50}
          value={jsonData?.organizationId ?? ''}
          placeholder="123456"
          onChange={(e: ChangeEvent<HTMLInputElement>) => patchJson('organizationId', e.target.value)}
        />
      </Field>

      <Field label="API Key" description="Meraki Dashboard API key. Generate one under My Profile → API access." required>
        <SecretInput
          id="cfg-api-key"
          width={50}
          isConfigured={Boolean(secureJsonFields?.apiKey)}
          value={secureJsonData?.apiKey ?? ''}
          placeholder="••••••••••••••••••••••••••••••••••••••••"
          onReset={() =>
            onOptionsChange({
              ...options,
              secureJsonFields: { ...secureJsonFields, apiKey: false },
              secureJsonData:   { ...secureJsonData,   apiKey: '' },
            })
          }
          onChange={(e: ChangeEvent<HTMLInputElement>) =>
            onOptionsChange({ ...options, secureJsonData: { ...secureJsonData, apiKey: e.target.value } })
          }
        />
      </Field>
    </FieldSet>
  );
}
