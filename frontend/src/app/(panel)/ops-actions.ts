'use server';

import { api } from '@/lib/server/api';
import { runAction } from '@/lib/server/actions';
import type { ActionResult, Schemas } from '@/lib/types';

export type OpsOverview = Schemas['OpsOverview'];
export type OpsSection =
    'campaigns' | 'conversations' | 'activity' | 'automations' | 'alerts';

const SECTIONS = new Set<OpsSection>([
    'campaigns',
    'conversations',
    'activity',
    'automations',
    'alerts',
]);

/**
 * The operations center (GET /ops/overview), limited to sections; the API
 * leaves out what the user may not read.
 */
export async function loadOpsAction(
    sections: OpsSection[],
    tz: string,
): Promise<ActionResult<OpsOverview>> {
    const wanted = sections.filter((s) => SECTIONS.has(s));
    const q = new URLSearchParams();
    if (wanted.length) q.set('sections', wanted.join(','));
    if (tz && tz.length <= 64) q.set('tz', tz);
    return runAction(
        async () => (await api<OpsOverview>(`/ops/overview?${q}`)).data,
    );
}
