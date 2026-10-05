import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { QueryEditorProps } from '@grafana/data';
import { Alert, Combobox, InlineField, InlineSwitch, Stack } from '@grafana/ui';

import { MerakiDS } from '../datasource';
import {
  DEVICE_PRODUCT_FILTER,
  DEVICE_PRODUCT_TYPES,
  MerakiDevice,
  MerakiDSOpts,
  MerakiNetwork,
  MerakiQuery,
  NEEDS_DEVICE,
  NEEDS_NETWORK,
  NETWORK_EVENT_PRODUCT_TYPES,
  NETWORK_PRODUCT_FILTER,
  OPTIONAL_DEVICE,
  OPTIONAL_NETWORK,
  QUERY_TYPE_OPTIONS,
  QueryType,
  SENSOR_METRICS,
} from '../types';

type Props = QueryEditorProps<MerakiDS, MerakiQuery, MerakiDSOpts>;

/** Pick a default event log product type from a network's list. Prefers 'appliance'. */
function defaultProductType(types: string[]): string {
  const logged = types.filter(t => NETWORK_EVENT_PRODUCT_TYPES.some(o => o.value === t));
  return logged.find(t => t === 'appliance') ?? logged[0] ?? '';
}

/** Filter networks to those compatible with a query type. */
function compatibleNetworks(all: MerakiNetwork[], qt: QueryType): MerakiNetwork[] {
  const required = NETWORK_PRODUCT_FILTER[qt];
  if (!required) return all;
  return all.filter(n => (n.productTypes ?? []).some(p => required.includes(p)));
}

export function QueryEditor({ query, onChange, onRunQuery, datasource }: Props) {
  const [networks, setNetworks] = useState<MerakiNetwork[]>([]);
  const [devices,  setDevices]  = useState<MerakiDevice[]>([]);
  const [nLoading, setNLoading] = useState(false);
  const [dLoading, setDLoading] = useState(false);

  // Load networks once
  useEffect(() => {
    setNLoading(true);
    datasource.networks()
      .then(setNetworks)
      .catch(() => setNetworks([]))
      .finally(() => setNLoading(false));
  }, [datasource]);

  // Load devices once
  useEffect(() => {
    setDLoading(true);
    datasource.devices()
      .then(setDevices)
      .catch(() => setDevices([]))
      .finally(() => setDLoading(false));
  }, [datasource]);

  // Networks visible for current query type
  const visibleNetworks = useMemo(
    () => compatibleNetworks(networks, query.queryType),
    [networks, query.queryType]
  );

  // Auto-select first compatible network when needed
  useEffect(() => {
    if (nLoading || visibleNetworks.length === 0) return;
    if (!NEEDS_NETWORK.includes(query.queryType)) return;
    if (query.networkId && visibleNetworks.some(n => n.id === query.networkId)) return;
    onChange({ ...query, networkId: visibleNetworks[0].id, deviceSerial: '' });
    onRunQuery();
  }, [nLoading, visibleNetworks, query, onChange, onRunQuery]);

  const networkOpts = useMemo(
    () => visibleNetworks.map(n => ({ label: n.name ?? n.id, value: n.id })),
    [visibleNetworks]
  );

  // Devices visible for current query type + selected network
  const visibleDevices = useMemo(() => {
    let list = devices;
    if (query.networkId) list = list.filter(d => d.networkId === query.networkId);
    const requiredPT = DEVICE_PRODUCT_FILTER[query.queryType];
    if (requiredPT) list = list.filter(d => d.productType?.toLowerCase() === requiredPT);
    return list;
  }, [devices, query.networkId, query.queryType]);

  // Auto-select first compatible device when needed
  useEffect(() => {
    if (dLoading || visibleDevices.length === 0) return;
    if (!NEEDS_DEVICE.includes(query.queryType)) return;
    if (query.deviceSerial && visibleDevices.some(d => d.serial === query.deviceSerial)) return;
    onChange({ ...query, deviceSerial: visibleDevices[0].serial });
    onRunQuery();
  }, [dLoading, visibleDevices, query, onChange, onRunQuery]);

  const deviceOpts = useMemo(
    () => visibleDevices.map(d => ({
      label: d.name ? `${d.name} (${d.serial})` : d.serial,
      value: d.serial,
    })),
    [visibleDevices]
  );

  // Product types on the currently selected network (for networkEvents)
  const selectedNetworkProductTypes = useMemo(() => {
    if (!query.networkId) return [];
    return networks.find(n => n.id === query.networkId)?.productTypes ?? [];
  }, [networks, query.networkId]);

  // Keep productType valid when switching networks in networkEvents
  useEffect(() => {
    if (query.queryType !== 'networkEvents') return;
    if (query.productType) {
      if (selectedNetworkProductTypes.length > 0 && !selectedNetworkProductTypes.includes(query.productType)) {
        onChange({ ...query, productType: '' });
      }
      return;
    }
    const pt = defaultProductType(selectedNetworkProductTypes);
    if (pt) {
      onChange({ ...query, productType: pt });
      onRunQuery();
    }
  }, [query, selectedNetworkProductTypes, onChange, onRunQuery]);

  // Product type options for networkEvents filtered to what the network supports
  const eventProductTypeOpts = useMemo(() => {
    if (selectedNetworkProductTypes.length === 0) return NETWORK_EVENT_PRODUCT_TYPES;
    const set = new Set(selectedNetworkProductTypes);
    const filtered = NETWORK_EVENT_PRODUCT_TYPES.filter(o => set.has(o.value));
    return filtered.length > 0 ? filtered : NETWORK_EVENT_PRODUCT_TYPES;
  }, [selectedNetworkProductTypes]);

  // ── Handlers ──────────────────────────────────────────────────────────────

  const onQueryTypeChange = useCallback((opt: { value?: QueryType } | null) => {
    if (!opt?.value) return;
    const qt = opt.value;

    let networkId = '';
    let productType = '';

    if (NEEDS_NETWORK.includes(qt)) {
      const compat = compatibleNetworks(networks, qt);
      if (compat.length > 0) networkId = compat[0].id;
    }
    if (qt === 'networkEvents' && networkId) {
      const net = networks.find(n => n.id === networkId);
      const types = net?.productTypes ?? [];
      if (types.length > 0) productType = defaultProductType(types);
    }

    onChange({ ...query, queryType: qt, networkId, deviceSerial: '', productType, historical: false });
    onRunQuery();
  }, [networks, query, onChange, onRunQuery]);

  const onNetworkChange = useCallback((opt: { value?: string } | null) => {
    const networkId = opt?.value ?? '';
    let productType = '';
    if (query.queryType === 'networkEvents' && networkId) {
      const net = networks.find(n => n.id === networkId);
      const types = net?.productTypes ?? [];
      if (types.length > 0) productType = defaultProductType(types);
    }
    onChange({ ...query, networkId, deviceSerial: '', productType });
    onRunQuery();
  }, [networks, query, onChange, onRunQuery]);

  const onDeviceChange = useCallback((opt: { value?: string } | null) => {
    onChange({ ...query, deviceSerial: opt?.value ?? '' });
    onRunQuery();
  }, [query, onChange, onRunQuery]);

  const onProductTypeChange = useCallback((opt: { value?: string } | null) => {
    onChange({ ...query, productType: opt?.value ?? '' });
    onRunQuery();
  }, [query, onChange, onRunQuery]);

  const onMetricChange = useCallback((opt: { value?: string } | null) => {
    onChange({ ...query, metric: opt?.value ?? '' });
    onRunQuery();
  }, [query, onChange, onRunQuery]);

  const onFahrenheitToggle = useCallback(() => {
    onChange({ ...query, fahrenheit: !query.fahrenheit });
    onRunQuery();
  }, [query, onChange, onRunQuery]);

  const onHistoricalToggle = useCallback(() => {
    onChange({ ...query, historical: !query.historical });
    onRunQuery();
  }, [query, onChange, onRunQuery]);

  // ── Derived flags ─────────────────────────────────────────────────────────

  const qt = query.queryType;
  const optionalNetwork  = OPTIONAL_NETWORK.includes(qt);
  const showNetwork      = NEEDS_NETWORK.includes(qt) || optionalNetwork;
  const optionalDevice   = OPTIONAL_DEVICE.includes(qt);
  const showDevice       = NEEDS_DEVICE.includes(qt) || optionalDevice;
  const showSensor       = qt === 'sensorReadingsLatest' || qt === 'sensorReadingsHistory';
  const showHistorical   = qt === 'deviceAvailabilities';
  const showEventPT      = qt === 'networkEvents';
  const showDeviceFilter = qt === 'deviceAvailabilities' || qt === 'deviceStatuses';
  const isLive           = qt !== 'deviceAvailabilities' || !query.historical;

  const liveMessages: Partial<Record<QueryType, string>> = {
    deviceAvailabilities:   'Showing a live snapshot — enable Historical Data to use the time range picker.',
    deviceStatuses:         'Live device status snapshot. Time range has no effect.',
    applianceUplinkStatuses:'Live WAN uplink snapshot. Time range has no effect.',
    switchPortStatuses:     'Live port state snapshot. Time range has no effect.',
    applianceLanPorts:      'Current LAN port settings. Time range has no effect.',
    networkClients:         'Live client snapshot. Time range has no effect.',
    deviceClients:          'Live client snapshot. Time range has no effect.',
    sensorReadingsLatest:   'Newest reading per sensor and metric. Time range has no effect.',
  };

  const liveMsg = isLive && liveMessages[qt];

  // ── Render ────────────────────────────────────────────────────────────────

  return (
    <Stack gap={1} direction="column">
      <InlineField label="Query Type" labelWidth={18}>
        <Combobox
          id="qe-type"
          options={QUERY_TYPE_OPTIONS}
          value={qt}
          onChange={onQueryTypeChange}
          width={32}
        />
      </InlineField>

      {showHistorical && (
        <InlineField
          label="Historical Data"
          labelWidth={18}
          tooltip="Query change history as a time series instead of a live snapshot"
        >
          <InlineSwitch id="qe-historical" value={!!query.historical} onChange={onHistoricalToggle} />
        </InlineField>
      )}

      {liveMsg && <Alert title="Live data" severity="info">{liveMsg}</Alert>}

      {showNetwork && (
        <InlineField label="Network" labelWidth={18} tooltip="Target Meraki network">
          <Combobox
            id="qe-network"
            options={networkOpts}
            value={query.networkId || null}
            onChange={onNetworkChange}
            placeholder={nLoading ? 'Loading networks…' : optionalNetwork ? 'All networks' : 'Select a network'}
            isClearable
            width={40}
          />
        </InlineField>
      )}

      {showDevice && (
        <InlineField label="Device" labelWidth={18} tooltip="Target device serial (filtered by network and product type)">
          <Combobox
            id="qe-device"
            options={deviceOpts}
            value={query.deviceSerial || null}
            onChange={onDeviceChange}
            placeholder={dLoading ? 'Loading devices…' : optionalDevice ? 'All sensors' : 'Select a device'}
            isClearable
            width={40}
          />
        </InlineField>
      )}

      {showEventPT && (
        <InlineField label="Product Type" labelWidth={18} tooltip="Required for mixed-product networks">
          <Combobox
            id="qe-event-pt"
            options={eventProductTypeOpts}
            value={query.productType || null}
            onChange={onProductTypeChange}
            placeholder="Select product type"
            width={26}
          />
        </InlineField>
      )}

      {showSensor && (
        <InlineField label="Metric" labelWidth={18} tooltip="Only this metric (optional)">
          <Combobox
            id="qe-metric"
            options={SENSOR_METRICS}
            value={query.metric ?? ''}
            onChange={onMetricChange}
            isClearable
            width={26}
          />
        </InlineField>
      )}

      {showSensor && (
        <InlineField label="Fahrenheit" labelWidth={18} tooltip="Temperature in °F instead of °C">
          <InlineSwitch id="qe-fahrenheit" value={!!query.fahrenheit} onChange={onFahrenheitToggle} />
        </InlineField>
      )}

      {showDeviceFilter && (
        <InlineField label="Product Type" labelWidth={18} tooltip="Filter devices by product type (optional)">
          <Combobox
            id="qe-device-pt"
            options={DEVICE_PRODUCT_TYPES}
            value={query.productType ?? ''}
            onChange={onProductTypeChange}
            isClearable
            width={26}
          />
        </InlineField>
      )}
    </Stack>
  );
}
