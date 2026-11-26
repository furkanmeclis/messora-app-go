import type { Metadata } from 'next';
import { getT } from '@/i18n/server';
import { api } from '@/lib/server/api';
import { requirePermission } from '@/lib/server/auth';
import type { Schemas } from '@/lib/types';
import { RANGES, type Range } from './ranges';
import { UsageClient } from './usage-client';

export async function generateMetadata(): Promise<Metadata> {
    const t = await getT();
    return { title: t('ai.usage.title') };
}

type Summary = Schemas['AIUsageSummary'];

/** The instants of a preset period, ending now. */
function period(range: Range, now: Date): { from: Date; to: Date } {
    const to = new Date(now.getTime() + 60_000);
    if (range === 'month')
        return {
            from: new Date(
                Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), 1),
            ),
            to,
        };
    const days = Number(range);
    const from = new Date(now);
    from.setUTCHours(0, 0, 0, 0);
    from.setUTCDate(from.getUTCDate() - (days - 1));
    return { from, to };
}

export default async function AIUsagePage({
    searchParams,
}: {
    searchParams: Promise<Record<string, string | undefined>>;
}) {
    await requirePermission('ai.read');
    const sp = await searchParams;
    const range: Range = (RANGES as readonly string[]).includes(sp.range ?? '')
        ? (sp.range as Range)
        : '30';
    const { from, to } = period(range, new Date());
    const load = (group: string) =>
        api<Summary>(
            `/ai/usage/summary?${new URLSearchParams({
                from: from.toISOString(),
                to: to.toISOString(),
                group_by: group,
            })}`,
        ).then((r) => r.data);
    const [day, model, provider, campaign, purpose] = await Promise.all([
        load('day'),
        load('model'),
        load('provider'),
        load('campaign'),
        load('purpose'),
    ]);
    return (
        <UsageClient
            range={range}
            from={from.toISOString()}
            summary={day}
            byModel={model.groups}
            byProvider={provider.groups}
            byCampaign={campaign.groups}
            byPurpose={purpose.groups}
        />
    );
}
