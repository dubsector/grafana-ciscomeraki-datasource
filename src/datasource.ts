import { CoreApp, DataSourceInstanceSettings, MetricFindValue, ScopedVars } from '@grafana/data';
import { DataSourceWithBackend, getTemplateSrv } from '@grafana/runtime';

import {
  DEFAULT_QUERY,
  MerakiDevice,
  MerakiDSOpts,
  MerakiNetwork,
  MerakiOrganization,
  MerakiQuery,
  NEEDS_DEVICE,
  NETWORK_EVENT_PRODUCT_TYPES,
  NETWORK_PRODUCT_FILTER,
} from './types';

const CACHE_MS = 60_000;

interface TTLCache<T> { data: T; exp: number }

export class MerakiDS extends DataSourceWithBackend<MerakiQuery, MerakiDSOpts> {
  private cache = new Map<string, TTLCache<unknown>>();
  readonly defaultOrg: string;

  constructor(settings: DataSourceInstanceSettings<MerakiDSOpts>) {
    super(settings);
    this.defaultOrg = String(settings.jsonData.organizationId ?? '');
  }

  getDefaultQuery(_: CoreApp): Partial<MerakiQuery> {
    return DEFAULT_QUERY;
  }

  filterQuery(q: MerakiQuery): boolean {
    if (!q.queryType) return false;
    // Skip instead of erroring when a device picker has nothing in it,
    // e.g. a network without switches, so the panel shows its empty text.
    if (NEEDS_DEVICE.includes(q.queryType) && !getTemplateSrv().replace(q.deviceSerial ?? '')) return false;
    return true;
  }

  applyTemplateVariables(q: MerakiQuery, vars: ScopedVars): MerakiQuery {
    const s = getTemplateSrv();
    const pt = q.productType ? s.replace(q.productType, vars) : '';
    // Treat Grafana "All" special values as empty (no filter)
    const ptClean = pt === '$__all' || pt === 'All' || pt.startsWith('{') ? '' : pt;
    return {
      ...q,
      organizationId: orgOrDefault(q.organizationId ? s.replace(q.organizationId, vars) : ''),
      networkId:    q.networkId    ? s.replace(q.networkId,    vars) : '',
      deviceSerial: q.deviceSerial ? s.replace(q.deviceSerial, vars) : '',
      productType:  ptClean,
      ...(q.uplink ? { uplink: s.replace(q.uplink, vars) } : {}),
    };
  }

  /**
   * Template variable support.
   * Pass a JSON string: { "queryType": "devices", "networkId": "${network}", "productType": "switch" },
   * { "queryType": "eventProductTypes", "networkId": "${network}" },
   * or { "queryType": "networks", "productTypes": ["switch", "appliance"] }.
   * { "queryType": "organizations" } lists the organizations the API key can
   * see, the data source's default first. Network and device queries take an
   * optional "organizationId".
   * With no query it returns all networks.
   */
  async metricFindQuery(raw: unknown): Promise<MetricFindValue[]> {
    let q: { queryType?: string; organizationId?: string; networkId?: string; productType?: string; productTypes?: string[] } | undefined;
    if (typeof raw === 'string') {
      try { q = JSON.parse(getTemplateSrv().replace(raw)); } catch { /* use default */ }
    } else if (typeof raw === 'object' && raw !== null) {
      q = raw as typeof q;
    }

    if (q?.queryType === 'organizations') {
      return (await this.organizations()).map(o => ({ text: o.name ?? o.id, value: o.id }));
    }

    const org = q?.organizationId ?? '';
    const networks = await this.networks(org);
    const nid = q?.networkId ? getTemplateSrv().replace(q.networkId) : '';

    if (q?.queryType === 'devices') {
      const devs = await this.devices(org);
      return devs
        .filter(d => !nid || d.networkId === nid)
        .filter(d => !q.productType || d.productType === q.productType)
        .map(d => ({ text: d.name ?? d.serial, value: d.serial }));
    }

    // Multi-product networks need a productType for the event log
    if (q?.queryType === 'eventProductTypes') {
      const types = networks.find(n => n.id === nid)?.productTypes ?? [];
      return NETWORK_EVENT_PRODUCT_TYPES
        .filter(t => types.includes(t.value))
        .map(t => ({ text: t.label, value: t.value }));
    }

    // Networks, limited to the given product types or the query type's
    const allowed = q?.productTypes ?? NETWORK_PRODUCT_FILTER[q?.queryType as keyof typeof NETWORK_PRODUCT_FILTER];
    return networks
      .filter(n => !allowed || (n.productTypes ?? []).some(p => allowed.includes(p)))
      .map(n => ({ text: n.name ?? n.id, value: n.id }));
  }

  /** Organizations the API key can see, the default one first. */
  async organizations(): Promise<MerakiOrganization[]> {
    const orgs = await this.cached<MerakiOrganization[]>('organizations', '');
    const isDefault = (o: MerakiOrganization) => String(o.id) === this.defaultOrg;
    return [...orgs.filter(isDefault), ...orgs.filter(o => !isDefault(o))];
  }

  /** Networks in org (a variable is fine), or in the default organization. */
  networks(org = ''): Promise<MerakiNetwork[]> {
    return this.cached('networks', org);
  }

  devices(org = ''): Promise<MerakiDevice[]> {
    return this.cached('devices', org);
  }

  private async cached<T>(path: string, org: string): Promise<T> {
    const id = orgOrDefault(org ? getTemplateSrv().replace(org) : '');
    const key = `${path}:${id}`;
    const hit = this.cache.get(key);
    if (hit && Date.now() < hit.exp) return hit.data as T;
    const data = (await this.getResource(path, id ? { organizationId: id } : {})) ?? [];
    this.cache.set(key, { data, exp: Date.now() + CACHE_MS });
    return data as T;
  }
}

/** An unresolved or "All" variable means the data source's default organization. */
function orgOrDefault(org: string): string {
  return org === '$__all' || org.startsWith('$') ? '' : org;
}
