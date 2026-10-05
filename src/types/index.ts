import { DataQuery, DataSourceJsonData } from '@grafana/schema';

// ── Query types ───────────────────────────────────────────────────────────────
// These map directly to Meraki Dashboard API v1 endpoints.

export type QueryType =
  | 'deviceAvailabilities'
  | 'deviceStatuses'
  | 'networkEvents'
  | 'securityEvents'
  | 'networkClients'
  | 'deviceClients'
  | 'wirelessLatencyStats'
  | 'wirelessConnectionStats'
  | 'wirelessClientCount'
  | 'switchPortStatuses'
  | 'applianceUplinkStatuses'
  | 'applianceLanPorts'
  | 'vpnStats'
  | 'sensorReadingsLatest'
  | 'sensorReadingsHistory';

export interface QueryTypeOption {
  label: string;
  value: QueryType;
  description: string;
}

export const QUERY_TYPE_OPTIONS: QueryTypeOption[] = [
  { value: 'deviceAvailabilities',     label: 'Device Availabilities',      description: 'Live device online/offline status' },
  { value: 'deviceStatuses',           label: 'Device Statuses',             description: 'Live status with model, LAN IP and gateway' },
  { value: 'networkEvents',            label: 'Network Events',              description: 'DHCP, 802.11, VPN event logs' },
  { value: 'securityEvents',           label: 'Security Events',             description: 'IDS/IPS and appliance security alerts' },
  { value: 'networkClients',           label: 'Network Clients',             description: 'Clients seen on a network' },
  { value: 'deviceClients',            label: 'Device Clients',              description: 'Clients seen on a specific device' },
  { value: 'wirelessLatencyStats',     label: 'Wireless Latency Stats',      description: 'Per-traffic-class latency' },
  { value: 'wirelessConnectionStats',  label: 'Wireless Connection Stats',   description: 'Auth, DHCP, DNS success counts' },
  { value: 'wirelessClientCount',      label: 'Wireless Client Count',       description: 'Historical client count time-series' },
  { value: 'switchPortStatuses',       label: 'Switch Port Statuses',        description: 'Live port status per switch' },
  { value: 'applianceUplinkStatuses',  label: 'Appliance Uplink Statuses',   description: 'Live WAN uplink status' },
  { value: 'applianceLanPorts',        label: 'Appliance LAN Ports',         description: 'MX LAN port settings (VLAN, access or trunk)' },
  { value: 'vpnStats',                 label: 'VPN Stats',                   description: 'Site-to-site VPN latency, loss, jitter' },
  { value: 'sensorReadingsLatest',     label: 'Sensor Readings',             description: 'Newest MT sensor reading per metric' },
  { value: 'sensorReadingsHistory',    label: 'Sensor Readings History',     description: 'MT sensor readings as time series' },
];

// Query types that are live snapshots (time range has no effect)
export const LIVE_QUERY_TYPES: QueryType[] = [
  'deviceAvailabilities',
  'deviceStatuses',
  'networkClients',
  'deviceClients',
  'switchPortStatuses',
  'applianceUplinkStatuses',
  'applianceLanPorts',
  'sensorReadingsLatest',
];

// Query types that need a network selected
export const NEEDS_NETWORK: QueryType[] = [
  'networkEvents',
  'securityEvents',
  'networkClients',
  'wirelessLatencyStats',
  'wirelessConnectionStats',
  'wirelessClientCount',
  'switchPortStatuses',
  'applianceLanPorts',
];

// Query types that can be narrowed to a network but default to the whole org
export const OPTIONAL_NETWORK: QueryType[] = [
  'deviceAvailabilities',
  'deviceStatuses',
  'applianceUplinkStatuses',
  'vpnStats',
  'sensorReadingsLatest',
  'sensorReadingsHistory',
];

// Query types that need a device serial selected
export const NEEDS_DEVICE: QueryType[] = [
  'deviceClients',
  'switchPortStatuses',
];

// Query types that can be narrowed to a device but default to all of them
export const OPTIONAL_DEVICE: QueryType[] = [
  'sensorReadingsLatest',
  'sensorReadingsHistory',
];

// For each query type, what product type must a device have to appear in the dropdown
export const DEVICE_PRODUCT_FILTER: Partial<Record<QueryType, string>> = {
  switchPortStatuses:      'switch',
  applianceUplinkStatuses: 'appliance',
  wirelessLatencyStats:    'wireless',
  wirelessConnectionStats: 'wireless',
  wirelessClientCount:     'wireless',
  sensorReadingsLatest:    'sensor',
  sensorReadingsHistory:   'sensor',
};

// For each query type, what network product types are relevant
export const NETWORK_PRODUCT_FILTER: Partial<Record<QueryType, string[]>> = {
  securityEvents:          ['appliance'],
  switchPortStatuses:      ['switch'],
  applianceUplinkStatuses: ['appliance'],
  applianceLanPorts:       ['appliance'],
  vpnStats:                ['appliance'],
  networkClients:          ['wireless', 'appliance', 'switch', 'cellularGateway'],
  wirelessLatencyStats:    ['wireless'],
  wirelessConnectionStats: ['wireless'],
  wirelessClientCount:     ['wireless'],
  sensorReadingsLatest:    ['sensor'],
  sensorReadingsHistory:   ['sensor'],
};

// ── Product type dropdowns ────────────────────────────────────────────────────

export const DEVICE_PRODUCT_TYPES = [
  { label: 'All',                  value: '' },
  { label: 'Wireless',             value: 'wireless' },
  { label: 'Appliance',            value: 'appliance' },
  { label: 'Switch',               value: 'switch' },
  { label: 'Camera',               value: 'camera' },
  { label: 'Systems Manager',      value: 'systemsManager' },
  { label: 'Cellular Gateway',     value: 'cellularGateway' },
  { label: 'Wireless Controller',  value: 'wirelessController' },
  { label: 'Campus Gateway',       value: 'campusGateway' },
  { label: 'Secure Connect',       value: 'secureConnect' },
  { label: 'Sensor',               value: 'sensor' },
];

export const NETWORK_EVENT_PRODUCT_TYPES = [
  { label: 'Wireless',             value: 'wireless' },
  { label: 'Appliance',            value: 'appliance' },
  { label: 'Switch',               value: 'switch' },
  { label: 'Camera',               value: 'camera' },
  { label: 'Systems Manager',      value: 'systemsManager' },
  { label: 'Cellular Gateway',     value: 'cellularGateway' },
  { label: 'Wireless Controller',  value: 'wirelessController' },
  { label: 'Campus Gateway',       value: 'campusGateway' },
  { label: 'Secure Connect',       value: 'secureConnect' },
];

// MT sensor metrics, as the readings endpoints name them
export const SENSOR_METRICS = [
  { label: 'All',                   value: '' },
  { label: 'Temperature',           value: 'temperature' },
  { label: 'Humidity',              value: 'humidity' },
  { label: 'Door',                  value: 'door' },
  { label: 'Water',                 value: 'water' },
  { label: 'CO2',                   value: 'co2' },
  { label: 'PM2.5',                 value: 'pm25' },
  { label: 'TVOC',                  value: 'tvoc' },
  { label: 'Indoor Air Quality',    value: 'indoorAirQuality' },
  { label: 'Noise',                 value: 'noise' },
  { label: 'Battery',               value: 'battery' },
  { label: 'Real Power',            value: 'realPower' },
  { label: 'Apparent Power',        value: 'apparentPower' },
  { label: 'Current',               value: 'current' },
  { label: 'Voltage',               value: 'voltage' },
  { label: 'Frequency',             value: 'frequency' },
  { label: 'Power Factor',          value: 'powerFactor' },
  { label: 'Downstream Power',      value: 'downstreamPower' },
  { label: 'Remote Lockout Switch', value: 'remoteLockoutSwitch' },
];

// ── Query model ───────────────────────────────────────────────────────────────

export interface MerakiQuery extends DataQuery {
  queryType: QueryType;
  networkId: string;
  deviceSerial: string;
  productType: string;
  /** deviceAvailabilities only: query change history instead of live status */
  historical: boolean;
  /** sensor queries only: one metric, or empty for all */
  metric?: string;
  /** sensor queries only: temperature in °F instead of °C */
  fahrenheit?: boolean;
}

export const DEFAULT_QUERY: Partial<MerakiQuery> = {
  queryType:    'deviceAvailabilities',
  networkId:    '',
  deviceSerial: '',
  productType:  '',
  historical:   false,
};

// ── Datasource config ─────────────────────────────────────────────────────────

export interface MerakiDSOpts extends DataSourceJsonData {
  baseUrl?: string;
  organizationId?: string;
}

export interface MerakiSecureOpts {
  apiKey?: string;
}

// ── API shapes (used by dropdowns) ───────────────────────────────────────────

export interface MerakiNetwork {
  id: string;
  name: string;
  productTypes?: string[];
}

export interface MerakiDevice {
  serial: string;
  name?: string;
  model?: string;
  productType?: string;
  networkId?: string;
}
