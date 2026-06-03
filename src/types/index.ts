import { DataQuery, DataSourceJsonData } from '@grafana/schema';

// ── Query types ───────────────────────────────────────────────────────────────
// These map directly to Meraki Dashboard API v1 endpoints.

export type QueryType =
  | 'deviceAvailabilities'
  | 'networkEvents'
  | 'securityEvents'
  | 'networkClients'
  | 'deviceClients'
  | 'wirelessLatencyStats'
  | 'wirelessConnectionStats'
  | 'wirelessClientCount'
  | 'switchPortStatuses'
  | 'applianceUplinkStatuses'
  | 'vpnStats';

export interface QueryTypeOption {
  label: string;
  value: QueryType;
  description: string;
}

export const QUERY_TYPE_OPTIONS: QueryTypeOption[] = [
  { value: 'deviceAvailabilities',     label: 'Device Availabilities',      description: 'Live device online/offline status' },
  { value: 'networkEvents',            label: 'Network Events',              description: 'DHCP, 802.11, VPN event logs' },
  { value: 'securityEvents',           label: 'Security Events',             description: 'IDS/IPS and appliance security alerts' },
  { value: 'networkClients',           label: 'Network Clients',             description: 'Clients seen on a network' },
  { value: 'deviceClients',            label: 'Device Clients',              description: 'Clients seen on a specific device' },
  { value: 'wirelessLatencyStats',     label: 'Wireless Latency Stats',      description: 'Per-traffic-class latency' },
  { value: 'wirelessConnectionStats',  label: 'Wireless Connection Stats',   description: 'Auth, DHCP, DNS success counts' },
  { value: 'wirelessClientCount',      label: 'Wireless Client Count',       description: 'Historical client count time-series' },
  { value: 'switchPortStatuses',       label: 'Switch Port Statuses',        description: 'Live port status per switch' },
  { value: 'applianceUplinkStatuses',  label: 'Appliance Uplink Statuses',   description: 'Live WAN uplink status' },
  { value: 'vpnStats',                 label: 'VPN Stats',                   description: 'Site-to-site VPN latency, loss, jitter' },
];

// Query types that are live snapshots (time range has no effect)
export const LIVE_QUERY_TYPES: QueryType[] = [
  'deviceAvailabilities',
  'networkClients',
  'deviceClients',
  'switchPortStatuses',
  'applianceUplinkStatuses',
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
  'applianceUplinkStatuses',
];

// Query types that need a device serial selected
export const NEEDS_DEVICE: QueryType[] = [
  'deviceClients',
  'switchPortStatuses',
];

// For each query type, what product type must a device have to appear in the dropdown
export const DEVICE_PRODUCT_FILTER: Partial<Record<QueryType, string>> = {
  switchPortStatuses:      'switch',
  applianceUplinkStatuses: 'appliance',
  wirelessLatencyStats:    'wireless',
  wirelessConnectionStats: 'wireless',
  wirelessClientCount:     'wireless',
};

// For each query type, what network product types are relevant
export const NETWORK_PRODUCT_FILTER: Partial<Record<QueryType, string[]>> = {
  securityEvents:          ['appliance'],
  networkClients:          ['wireless', 'appliance', 'switch', 'cellularGateway'],
  wirelessLatencyStats:    ['wireless'],
  wirelessConnectionStats: ['wireless'],
  wirelessClientCount:     ['wireless'],
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
];

export const NETWORK_EVENT_PRODUCT_TYPES = [
  { label: 'Wireless',             value: 'wireless' },
  { label: 'Appliance',            value: 'appliance' },
  { label: 'Switch',               value: 'switch' },
  { label: 'Camera',               value: 'camera' },
  { label: 'Systems Manager',      value: 'systemsManager' },
  { label: 'Cellular Gateway',     value: 'cellularGateway' },
  { label: 'Wireless Controller',  value: 'wirelessController' },
  { label: 'Secure Connect',       value: 'secureConnect' },
];

// ── Query model ───────────────────────────────────────────────────────────────

export interface MerakiQuery extends DataQuery {
  queryType: QueryType;
  networkId: string;
  deviceSerial: string;
  productType: string;
  /** deviceAvailabilities only: query change history instead of live status */
  historical: boolean;
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
