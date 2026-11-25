'use server';

import { revalidatePath } from 'next/cache';
import { getT } from '@/i18n/server';
import { api } from '@/lib/server/api';
import { runAction } from '@/lib/server/actions';
import type { ActionResult, Offer, Schemas } from '@/lib/types';

const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;

export type OfferForm = {
    title: string;
    description: string;
    discount_text: string;
    product: string;
    starts_at: string;
    ends_at: string;
    image_url: string;
    cta_url: string;
    coupon_code: string;
    template_id: string;
    publish: boolean;
};

/** Creates (id null) or edits an offer. */
export async function saveOfferAction(
    id: string | null,
    f: OfferForm,
): Promise<ActionResult<Offer>> {
    const t = await getT();
    if (id !== null && !UUID.test(id)) throw new Error('invalid id');
    const template = UUID.test(f.template_id) ? f.template_id : undefined;
    const res = await runAction(
        async () => {
            if (id === null) {
                const body: Schemas['OfferInput'] = {
                    title: f.title,
                    description: f.description,
                    discount_text: f.discount_text,
                    product: f.product,
                    starts_at: f.starts_at,
                    ends_at: f.ends_at,
                    status: f.publish ? 'scheduled' : 'draft',
                    ...(f.image_url.trim() ? { image_url: f.image_url } : {}),
                    ...(f.cta_url.trim() ? { cta_url: f.cta_url } : {}),
                    ...(f.coupon_code.trim()
                        ? { coupon_code: f.coupon_code }
                        : {}),
                    ...(template ? { template_id: template } : {}),
                };
                return (await api<Offer>('/offers', { method: 'POST', body }))
                    .data;
            }
            const body: Schemas['OfferUpdate'] = {
                title: f.title,
                description: f.description,
                discount_text: f.discount_text,
                product: f.product,
                starts_at: f.starts_at,
                ends_at: f.ends_at,
                image_url: f.image_url,
                cta_url: f.cta_url,
                coupon_code: f.coupon_code,
                status: f.publish ? 'scheduled' : 'draft',
                ...(template
                    ? { template_id: template }
                    : { clear_template: true }),
            };
            return (
                await api<Offer>(`/offers/${id}`, { method: 'PATCH', body })
            ).data;
        },
        id === null ? t('offers.created') : t('offers.saved'),
    );
    revalidatePath('/templates/offers');
    return res;
}

/** Ends an offer now: campaigns using it pause. */
export async function endOfferAction(id: string): Promise<ActionResult<Offer>> {
    if (!UUID.test(id)) throw new Error('invalid id');
    const t = await getT();
    const res = await runAction(
        async () =>
            (
                await api<Offer>(`/offers/${id}`, {
                    method: 'PATCH',
                    body: { status: 'expired' },
                })
            ).data,
        t('offers.ended'),
    );
    revalidatePath('/templates/offers');
    return res;
}

export async function deleteOfferAction(id: string): Promise<ActionResult> {
    if (!UUID.test(id)) throw new Error('invalid id');
    const t = await getT();
    const res = await runAction(
        () => api(`/offers/${id}`, { method: 'DELETE' }).then(() => undefined),
        t('offers.deleted'),
    );
    revalidatePath('/templates/offers');
    return res;
}
