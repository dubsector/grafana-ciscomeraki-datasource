import { CoreApp, MetricFindValue, ScopedVars } from '@grafana/data';
import { DataSourceWithBackend, getTemplateSrv } from '@grafana/runtime';

import {
  DEFAULT_QUERY,
  MerakiDevice,
  MerakiDSOpts,
  MerakiNetwork,
  MerakiQuery,
  NETWORK_EVENT_PRODUCT_TYPES,
  NETWORK_PRODUCT_FILTER,
} from './types';

const CACHE_MS = 60_000;

interface TTLCache<T> { data: T; exp: number }

export class MerakiDS extends DataSourceWithBackend<MerakiQuery, MerakiDSOpts> {
  private _networks: TTLCache<MerakiNetwork[]> | null = null;
  private _devices:  TTLCache<MerakiDevice[]>  | null = null;

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
   * Pass a JSON string: { "queryType": "devices", "networkId": "${network}", "productType": "switch" }
   * or { "queryType": "eventProductTypes", "networkId": "${network}" },
   * or just rely on default (returns all networks).
   */
  async metricFindQuery(raw: unknown): Promise<MetricFindValue[]> {
    let q: { queryType?: string; networkId?: string; productType?: string } | undefined;
    if (typeof raw === 'string') {
      try { q = JSON.parse(getTemplateSrv().replace(raw)); } catch { /* use default */ }
    } else if (typeof raw === 'object' && raw !== null) {
      q = raw as typeof q;
    }

    const networks = await this.networks();
    const nid = q?.networkId ? getTemplateSrv().replace(q.networkId) : '';

    if (q?.queryType === 'devices') {
      const devs = await this.devices();
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
