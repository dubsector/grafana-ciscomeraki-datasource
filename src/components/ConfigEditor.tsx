import React, { ChangeEvent, FocusEvent } from 'react';
import { DataSourcePluginOptionsEditorProps } from '@grafana/data';
import { Field, Input, SecretInput, FieldSet } from '@grafana/ui';

import { MerakiDSOpts, MerakiSecureOpts } from '../types';

type Props = DataSourcePluginOptionsEditorProps<MerakiDSOpts, MerakiSecureOpts>;

const DEFAULT_URL = 'https://api.meraki.com/api/v1';

const REGIONAL_URLS = [
  'Global:  https://api.meraki.com/api/v1',
  'India:   https://api.in.meraki.com/api/v1',
  'FedRAMP: https://api.gov.meraki.com/api/v1',
];

export function ConfigEditor({ options, onOptionsChange }: Props) {
  const { jsonData, secureJsonFields, secureJsonData } = options;

  function patchJson(key: keyof MerakiDSOpts, value: string) {
    onOptionsChange({ ...options, jsonData: { ...jsonData, [key]: value } });
  }

  return (
    <FieldSet label="Connection">
      <Field
        label="Base URL"
        description={`Meraki Dashboard API base URL. Regional options: ${REGIONAL_URLS.join(' | ')}`}
      >
        <Input
          id="cfg-base-url"
          width={50}
          value={jsonData?.baseUrl ?? ''}
          placeholder={DEFAULT_URL}
          onChange={(e: ChangeEvent<HTMLInputElement>) => patchJson('baseUrl', e.target.value)}
          onBlur={(e: FocusEvent<HTMLInputElement>) => {
            if (!e.target.value.trim()) patchJson('baseUrl', DEFAULT_URL);
          }}
        />
      </Field>

      <Field label="Organization ID" description="Your Meraki org ID (find it under Organization → Settings)." required>
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
