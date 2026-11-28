// Build-time plan data. Order: PLANS_API_URL (default: the live app API;
// `file:<path>` reads a local fixture) → src/data/plans-fallback.json.
// The result is also written to /plans-snapshot.json, which the Node server
// uses as its fallback while it keeps the live data fresh (server.mjs).
import { readFile } from 'node:fs/promises';
// @ts-ignore plain ES module shared with the server
import * as core from '../../server/plans-core.mjs';
import { fallbackPlans } from '../data/plans';

export type Plan = {
  key: string; custom: boolean; vat: number;
  name: { tr: string; en: string }; desc: { tr: string; en: string };
  monthly: number | null; yearly: number | null;
  limits: Record<string, number | null>; features: Record<string, boolean>;
};

async function load(): Promise<{ plans: Plan[]; source: string; updatedAt: string }> {
  const url = process.env.PLANS_API_URL || core.PLANS_API_URL;
  try {
    const json = url.startsWith('file:')
      ? JSON.parse(await readFile(url.slice(5), 'utf8'))
      : await (await fetch(url, { signal: AbortSignal.timeout(6000), headers: { Accept: 'application/json' } })).json();
    const plans = core.normalize(json);
    if (plans) return { plans, source: url, updatedAt: json.updated_at ?? new Date().toISOString() };
    console.warn(`[plans] unexpected response from ${url}; using the fallback`);
  } catch (e) {
    console.warn(`[plans] ${url} unreachable (${(e as Error).message}); using the fallback`);
  }
  return { plans: core.normalize(fallbackPlans)!, source: 'fallback', updatedAt: fallbackPlans.updated_at };
}

export const planData = await load();
export const plans: Plan[] = planData.plans;
export const P = core;
