import { CoreApp, DataSourceInstanceSettings, MetricFindValue, ScopedVars } from '@grafana/data';
import { DataSourceWithBackend, getTemplateSrv } from '@grafana/runtime';

import {
  DEFAULT_QUERY,
  MerakiDevice,
  MerakiDSOpts,
  MerakiNetwork,
  MerakiQuery,
  NETWORK_PRODUCT_FILTER,
} from './types';

const CACHE_MS = 60_000;

interface TTLCache<T> { data: T; exp: number }

export class MerakiDS extends DataSourceWithBackend<MerakiQuery, MerakiDSOpts> {
  private _networks: TTLCache<MerakiNetwork[]> | null = null;
  private _devices:  TTLCache<MerakiDevice[]>  | null = null;

  constructor(settings: DataSourceInstanceSettings<MerakiDSOpts>) {
    super(settings);
  }

  getDefaultQuery(_: CoreApp): Partial<MerakiQuery> {
    return DEFAULT_QUERY;
  }

  filterQuery(q: MerakiQuery): boolean {
    return !!q.queryType;
  }

  applyTemplateVariables(q: MerakiQuery, vars: ScopedVars): MerakiQuery {
    const s = getTemplateSrv();
    const pt = q.productType ? s.replace(q.productType, vars) : '';
    // Treat Grafana "All" special values as empty (no filter)
    const ptClean = pt === '$__all' || pt === 'All' || pt.startsWith('{') ? '' : pt;
    return {
      ...q,
      networkId:    q.networkId    ? s.replace(q.networkId,    vars) : '',
      deviceSerial: q.deviceSerial ? s.replace(q.deviceSerial, vars) : '',
      productType:  ptClean,
    };
  }

  /**
   * Template variable support.
   * Pass a JSON string: { "queryType": "devices", "networkId": "${network}" }
   * or just rely on default (returns all networks).
   */
  async metricFindQuery(raw: unknown): Promise<MetricFindValue[]> {
    let q: { queryType?: string; networkId?: string } | undefined;
    if (typeof raw === 'string') {
      try { q = JSON.parse(getTemplateSrv().replace(raw)); } catch { /* use default */ }
    } else if (typeof raw === 'object' && raw !== null) {
      q = raw as typeof q;
    }

    const networks = await this.networks();

    if (q?.queryType === 'devices') {
      const nid = q.networkId ? getTemplateSrv().replace(q.networkId) : '';
      const devs = await this.devices();
      return (nid ? devs.filter(d => d.networkId === nid) : devs)
        .map(d => ({ text: d.name ?? d.serial, value: d.serial }));
    }

    // Filter networks by query type compatibility if requested
    if (q?.queryType) {
      const allowed = NETWORK_PRODUCT_FILTER[q.queryType as keyof typeof NETWORK_PRODUCT_FILTER];
      if (allowed) {
        return networks
          .filter(n => (n.productTypes ?? []).some(p => allowed.includes(p)))
          .map(n => ({ text: n.name ?? n.id, value: n.id }));
      }
    }

    return networks.map(n => ({ text: n.name ?? n.id, value: n.id }));
  }

  async networks(): Promise<MerakiNetwork[]> {
    if (this._networks && Date.now() < this._networks.exp) return this._networks.data;
    const data: MerakiNetwork[] = (await this.getResource('networks')) ?? [];
    this._networks = { data, exp: Date.now() + CACHE_MS };
    return data;
  }

  async devices(): Promise<MerakiDevice[]> {
    if (this._devices && Date.now() < this._devices.exp) return this._devices.data;
    const data: MerakiDevice[] = (await this.getResource('devices')) ?? [];
    this._devices = { data, exp: Date.now() + CACHE_MS };
    return data;
  }
}
