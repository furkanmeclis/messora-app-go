'use server';

import { revalidatePath } from 'next/cache';
import { api } from '@/lib/server/api';
import { runAction } from '@/lib/server/actions';
import { getT } from '@/i18n/server';
import type { ActionResult, Campaign, Schemas, Segment } from '@/lib/types';

const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;
const STATUSES = new Set([
    'draft',
    'scheduled',
    'preparing',
    'running',
    'paused',
    'paused_limit',
    'completed',
    'cancelled',
    'failed',
]);
const PRIORITIES = new Set(['high', 'normal', 'low']);

function check(id: string) {
    if (!UUID.test(id)) throw new Error('invalid id');
}

export type CampaignInput = Schemas['CreateCampaignRequest'];

/** More campaigns for the list ("Load more"). */
export async function loadCampaignsAction(
    status: string,
    cursor: string,
): Promise<ActionResult<{ items: Campaign[]; next?: string }>> {
    check(cursor);
    const q = new URLSearchParams({ limit: '25', cursor });
    if (STATUSES.has(status)) q.set('status', status);
    return runAction(async () => {
        const r = await api<Campaign[]>(`/campaigns?${q}`);
        return { items: r.data, next: r.nextCursor };
    });
}

/** Creates a draft (id null) or edits a draft or scheduled campaign. */
export async function saveCampaignAction(
    id: string | null,
    input: CampaignInput,
    idempotencyKey: string,
): Promise<ActionResult<Campaign>> {
    if (id) check(id);
    const res = await runAction(async () => {
        const r = id
            ? await api<Campaign>(`/campaigns/${id}`, {
                  method: 'PATCH',
                  body: input,
              })
            : await api<Campaign>('/campaigns', {
                  method: 'POST',
                  body: input,
                  idempotencyKey,
              });
        return r.data;
    });
    revalidatePath('/campaigns');
    return res;
}

/** Starts now, or schedules when scheduledAt (ISO, UTC) is given. */
export async function startCampaignAction(
    id: string,
    scheduledAt: string | null,
    idempotencyKey: string,
): Promise<ActionResult<Campaign>> {
    check(id);
    const t = await getT();
    if (scheduledAt && Number.isNaN(Date.parse(scheduledAt))) {
        const message = t('campaigns.actions.invalidDate');
        return {
            ok: false,
            code: 'VALIDATION_ERROR',
            message,
            fieldErrors: { scheduled_at: message },
        };
    }
    const res = await runAction(
        async () =>
            (
                await api<Campaign>(`/campaigns/${id}/start`, {
                    method: 'POST',
                    body: scheduledAt ? { scheduled_at: scheduledAt } : {},
                    idempotencyKey,
                })
            ).data,
        scheduledAt
            ? t('campaigns.actions.scheduled')
            : t('campaigns.actions.started'),
    );
    revalidatePath('/campaigns');
    return res;
}

const CONTROL_MESSAGES = {
    pause: 'campaigns.actions.paused',
    resume: 'campaigns.actions.resumed',
    cancel: 'campaigns.actions.cancelled',
} as const;

export async function controlCampaignAction(
    id: string,
    op: keyof typeof CONTROL_MESSAGES,
): Promise<ActionResult<Campaign>> {
    check(id);
    if (!(op in CONTROL_MESSAGES)) throw new Error('invalid operation');
    const t = await getT();
    const res = await runAction(
        async () =>
            (
                await api<Campaign>(`/campaigns/${id}/${op}`, {
                    method: 'POST',
                })
            ).data,
        t(CONTROL_MESSAGES[op]),
    );
    revalidatePath('/campaigns');
    return res;
}

export async function deleteCampaignAction(id: string): Promise<ActionResult> {
    check(id);
    const t = await getT();
    const res = await runAction(
        () =>
            api(`/campaigns/${id}`, { method: 'DELETE' }).then(() => undefined),
        t('campaigns.actions.deleted'),
    );
    revalidatePath('/campaigns');
    return res;
}

export type Preflight = Schemas['CampaignPreflight'];

/**
 * Who in the audience may receive the template under the consent rules
 * (POST /campaigns/preflight): totals and blocked or warned counts by
 * reason.
 */
export async function preflightAction(
    input: Schemas['CampaignPreflightRequest'],
): Promise<ActionResult<Preflight>> {
    if (input.phone_number_id) check(input.phone_number_id);
    for (const k of ['segment_id', 'group_id', 'tag_id'] as const) {
        const v = input.audience[k];
        if (v) check(v);
    }
    if (input.audience.previous_campaign)
        check(input.audience.previous_campaign.campaign_id);
    return runAction(
        async () =>
            (
                await api<Preflight>('/campaigns/preflight', {
                    method: 'POST',
                    body: input,
                })
            ).data,
    );
}

/**
 * Roughly how many contacts an audience reaches now. Opted-out contacts are
 * left out for "all contacts" because the campaign skips them anyway.
 */
export async function audienceEstimateAction(
    kind: 'segment' | 'all',
    id?: string,
): Promise<ActionResult<number>> {
    if (kind === 'segment') {
        check(id ?? '');
        return runAction(async () => {
            const r = await api<Segment>(`/segments/${id}`);
            return r.data.contact_count ?? 0;
        });
    }
    return runAction(async () => {
        const r = await api<Schemas['SegmentPreview']>('/segments/preview', {
            method: 'POST',
            body: {
                definition: { field: 'opted_out', operator: 'is_false' },
            },
        });
        return r.data.count;
    });
}

export type CampaignSettings = Schemas['CampaignSettingsRequest'];
export type ResumeRequest = Schemas['CampaignResumeRequest'];
export type ResumePreview = Schemas['CampaignResumePreview'];
export type CampaignPause = Schemas['CampaignPause'];

function checkSettings(s: CampaignSettings) {
    if (s.priority && !PRIORITIES.has(s.priority))
        throw new Error('invalid priority');
}

/** Changes name, priority or rate cap of a campaign in progress. */
export async function updateCampaignSettingsAction(
    id: string,
    settings: CampaignSettings,
): Promise<ActionResult<Campaign>> {
    check(id);
    checkSettings(settings);
    const t = await getT();
    const res = await runAction(
        async () =>
            (
                await api<Campaign>(`/campaigns/${id}/settings`, {
                    method: 'PATCH',
                    body: settings,
                })
            ).data,
        settings.priority ? t('campaigns.engine.priorityChanged') : undefined,
    );
    revalidatePath('/campaigns');
    return res;
}

/** What resuming would do (consent checked again, estimated time). */
export async function resumePreviewAction(
    id: string,
    settings: CampaignSettings,
): Promise<ActionResult<ResumePreview>> {
    check(id);
    checkSettings(settings);
    return runAction(
        async () =>
            (
                await api<ResumePreview>(`/campaigns/${id}/resume-preview`, {
                    method: 'POST',
                    body: settings,
                })
            ).data,
    );
}

/** Resumes now or at resume_at, with optional changes. */
export async function resumeCampaignAction(
    id: string,
    body: ResumeRequest,
): Promise<ActionResult<Campaign>> {
    check(id);
    checkSettings(body);
    if (body.resume_at && Number.isNaN(Date.parse(body.resume_at)))
        throw new Error('invalid date');
    const t = await getT();
    const res = await runAction(
        async () =>
            (
                await api<Campaign>(`/campaigns/${id}/resume`, {
                    method: 'POST',
                    body,
                })
            ).data,
        t('campaigns.actions.resumed'),
    );
    revalidatePath('/campaigns');
    return res;
}
