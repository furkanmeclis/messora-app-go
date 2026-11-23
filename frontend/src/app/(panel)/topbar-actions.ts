'use server';

import { api } from '@/lib/server/api';
import type { Campaign } from '@/lib/types';

/** Name and size of a campaign that started after the page loaded. */
export async function campaignSummaryAction(
    id: string,
): Promise<{ name: string; total: number } | null> {
    if (!/^[0-9a-f-]{36}$/i.test(id)) return null;
    try {
        const c = (await api<Campaign>(`/campaigns/${id}`)).data;
        return { name: c.name, total: Number(c.total_recipients) };
    } catch {
        return null;
    }
}
