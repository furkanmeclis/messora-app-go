'use server';

import { revalidatePath } from 'next/cache';
import { api } from '@/lib/server/api';
import { runAction } from '@/lib/server/actions';
import { getT } from '@/i18n/server';
import { invalidateAppearance } from '@/lib/server/appearance';
import type {
    ActionResult,
    Schemas,
    Settings,
    SmtpSettings,
} from '@/lib/types';

type Patch = Schemas['UpdateSettingsRequest'];

/** Changes workspace settings (only the fields given). */
export async function saveSettingsAction(
    patch: Patch,
    successKey:
        'settings.general.saved' | 'common.saved' = 'settings.general.saved',
): Promise<ActionResult<Settings>> {
    const t = await getT();
    const res = await runAction(
        async () =>
            (await api<Settings>('/settings', { method: 'PATCH', body: patch }))
                .data,
        t(successKey),
    );
    invalidateAppearance();
    revalidatePath('/', 'layout');
    return res;
}

/** Marks wizard steps as done (merged with the stored ones). */
export async function completeSetupStepAction(
    step: Schemas['SetupStep'],
    done: Schemas['SetupStep'][],
    finish = false,
    extra: Patch = {},
): Promise<ActionResult<Settings>> {
    const steps = Array.from(new Set([...done, step]));
    const res = await runAction(
        async () =>
            (
                await api<Settings>('/settings', {
                    method: 'PATCH',
                    body: {
                        ...extra,
                        setup_steps: steps,
                        ...(finish ? { setup_completed: true } : {}),
                    },
                })
            ).data,
    );
    invalidateAppearance();
    revalidatePath('/', 'layout');
    return res;
}

export async function removeLogoAction(): Promise<ActionResult<Settings>> {
    const t = await getT();
    const res = await runAction(
        async () =>
            (await api<Settings>('/settings/logo', { method: 'DELETE' })).data,
        t('settings.logo.removed'),
    );
    invalidateAppearance();
    revalidatePath('/', 'layout');
    return res;
}

export async function saveSmtpAction(
    input: Schemas['SmtpSettingsInput'],
): Promise<ActionResult<SmtpSettings>> {
    const t = await getT();
    return runAction(
        async () =>
            (
                await api<SmtpSettings>('/settings/smtp', {
                    method: 'PUT',
                    body: input,
                })
            ).data,
        t('settings.email.saved'),
    );
}

/** Changes the reply-to address of the workspace's e-mails. */
export async function saveEmailSettingsAction(
    replyTo: string | null,
): Promise<ActionResult<Schemas['EmailSettings']>> {
    const t = await getT();
    const res = await runAction(
        async () =>
            (
                await api<Schemas['EmailSettings']>('/settings/email', {
                    method: 'PATCH',
                    body: { reply_to: replyTo },
                })
            ).data,
        t('plans.email.saved'),
    );
    if (res.ok) revalidatePath('/settings/email');
    return res;
}

export async function testSmtpAction(
    to: string,
    config?: Schemas['SmtpSettingsInput'],
): Promise<ActionResult> {
    const t = await getT();
    return runAction(
        () =>
            api('/settings/smtp/test', {
                method: 'POST',
                body: config ? { to, config } : { to },
            }).then(() => undefined),
        t('settings.email.testSent', { to }),
    );
}

/** Replaces the consent settings (policy, keywords, confirmations). */
export async function saveConsentSettingsAction(
    input: Schemas['ConsentSettings'],
): Promise<ActionResult<Schemas['ConsentSettings']>> {
    const t = await getT();
    const res = await runAction(
        async () =>
            (
                await api<Schemas['ConsentSettings']>('/settings/consent', {
                    method: 'PUT',
                    body: input,
                })
            ).data,
        t('consent.settings.saved'),
    );
    if (res.ok) revalidatePath('/settings/consent');
    return res;
}

/** Replaces the campaign protection settings (spam thresholds, attribution). */
export async function saveCampaignProtectionAction(
    input: Schemas['CampaignProtectionSettings'],
): Promise<ActionResult<Schemas['CampaignProtectionSettings']>> {
    const t = await getT();
    const res = await runAction(
        async () =>
            (
                await api<Schemas['CampaignProtectionSettings']>(
                    '/settings/campaign-protection',
                    { method: 'PUT', body: input },
                )
            ).data,
        t('analytics.settings.saved'),
    );
    if (res.ok) revalidatePath('/settings/campaign-protection');
    return res;
}
