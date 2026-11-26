'use server';

import { revalidatePath } from 'next/cache';
import { api } from '@/lib/server/api';
import { runAction } from '@/lib/server/actions';
import type { ActionResult, Campaign, Schemas } from '@/lib/types';

const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;

function check(id: string) {
    if (!UUID.test(id)) throw new Error('invalid id');
}

export type CampaignForecast = Schemas['CampaignForecast'];
export type CampaignAnalytics = Schemas['CampaignAnalytics'];
export type ForecastVsActual = Schemas['CampaignForecastVsActual'];
export type BreakdownBucket = Schemas['CampaignBreakdownBucket'];
export type RecipientRow = Schemas['CampaignRecipientRow'];
export type RecipientFilter = Schemas['CampaignRecipientFilter'];
export type CampaignExport = Schemas['CampaignExport'];

function checkFilter(f?: RecipientFilter) {
    if (!f) return;
    for (const k of ['tag_id', 'group_id', 'segment_id'] as const) {
        const v = f[k];
        if (v) check(v);
    }
    for (const id of f.ids ?? []) check(id);
}

/** Pre-launch analysis for the wizard's review step. */
export async function forecastAction(
    input: Schemas['CampaignForecastRequest'],
): Promise<ActionResult<CampaignForecast>> {
    if (input.phone_number_id) check(input.phone_number_id);
    return runAction(
        async () =>
            (
                await api<CampaignForecast>('/campaigns/forecast', {
                    method: 'POST',
                    body: input,
                })
            ).data,
    );
}

/** Everything the analysis center shows, in one round trip. */
export async function loadAnalysisAction(id: string): Promise<
    ActionResult<{
        analytics: CampaignAnalytics;
        forecast: ForecastVsActual;
    }>
> {
    check(id);
    return runAction(async () => {
        const [a, f] = await Promise.all([
            api<CampaignAnalytics>(`/campaigns/${id}/analytics`),
            api<ForecastVsActual>(`/campaigns/${id}/forecast`),
        ]);
        return { analytics: a.data, forecast: f.data };
    });
}

export async function loadBreakdownAction(
    id: string,
    dimension: 'group' | 'segment' | 'tag' | 'contact_type' | 'attribute',
    attrKey?: string,
): Promise<ActionResult<BreakdownBucket[]>> {
    check(id);
    const q = new URLSearchParams({ dimension });
    if (dimension === 'attribute' && attrKey)
        q.set('attr_key', attrKey.slice(0, 100));
    return runAction(
        async () =>
            (await api<BreakdownBucket[]>(`/campaigns/${id}/breakdown?${q}`))
                .data,
    );
}

export async function searchRecipientsAction(
    id: string,
    body: Schemas['CampaignRecipientSearchRequest'],
): Promise<ActionResult<{ rows: RecipientRow[]; total: number }>> {
    check(id);
    checkFilter(body.filter);
    return runAction(async () => {
        const r = await api<RecipientRow[]>(
            `/campaigns/${id}/recipients/search`,
            { method: 'POST', body },
        );
        return { rows: r.data, total: r.total ?? r.data.length };
    });
}

export async function bulkRecipientsAction(
    id: string,
    body: Schemas['CampaignRecipientBulkRequest'],
): Promise<ActionResult<{ affected: number; campaign?: Campaign }>> {
    check(id);
    checkFilter(body.filter);
    if (body.target_id) check(body.target_id);
    return runAction(
        async () =>
            (
                await api<{ affected: number; campaign?: Campaign }>(
                    `/campaigns/${id}/recipients/bulk`,
                    { method: 'POST', body },
                )
            ).data,
    );
}

export async function setReactionAction(
    id: string,
    recipientId: string,
    reaction: 'positive' | 'negative' | 'neutral' | null,
): Promise<ActionResult> {
    check(id);
    check(recipientId);
    return runAction(async () => {
        await api(`/campaigns/${id}/recipients/${recipientId}/reaction`, {
            method: 'PUT',
            body: { reaction },
        });
        return undefined;
    });
}

export async function setReactionMapAction(
    id: string,
    reactionMap: Record<string, 'positive' | 'negative' | 'neutral'>,
): Promise<ActionResult<Campaign>> {
    check(id);
    const res = await runAction(
        async () =>
            (
                await api<Campaign>(`/campaigns/${id}/reaction-map`, {
                    method: 'PUT',
                    body: { reaction_map: reactionMap },
                })
            ).data,
    );
    if (res.ok) revalidatePath(`/campaigns/${id}`);
    return res;
}

export async function listExportsAction(
    id: string,
): Promise<ActionResult<CampaignExport[]>> {
    check(id);
    return runAction(
        async () =>
            (await api<CampaignExport[]>(`/campaigns/${id}/exports`)).data,
    );
}

export async function createExportAction(
    id: string,
    body: Schemas['CampaignExportRequest'],
): Promise<ActionResult<CampaignExport>> {
    check(id);
    checkFilter(body.filter);
    return runAction(
        async () =>
            (
                await api<CampaignExport>(`/campaigns/${id}/exports`, {
                    method: 'POST',
                    body,
                })
            ).data,
    );
}

/** Tags and groups for the bulk actions and filters. */
export async function loadAudienceRefsAction(): Promise<
    ActionResult<{
        tags: Schemas['Tag'][];
        groups: Schemas['Group'][];
        segments: Schemas['Segment'][];
    }>
> {
    return runAction(async () => {
        const [t, g, s] = await Promise.all([
            api<Schemas['Tag'][]>('/tags'),
            api<Schemas['Group'][]>('/groups'),
            api<Schemas['Segment'][]>('/segments'),
        ]);
        return { tags: t.data, groups: g.data, segments: s.data };
    });
}
