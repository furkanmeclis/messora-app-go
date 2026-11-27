import type { Metadata } from 'next';
import { notFound } from 'next/navigation';
import { api, ApiError } from '@/lib/server/api';
import { can, requirePermission } from '@/lib/server/auth';
import { getT } from '@/i18n/server';
import type {
    Campaign,
    CampaignEventItem,
    PhoneNumber,
    Schemas,
} from '@/lib/types';
import { CampaignDashboard } from './dashboard';

export async function generateMetadata(): Promise<Metadata> {
    const t = await getT();
    return { title: t('campaigns.campaign') };
}

const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;

export default async function CampaignPage({
    params,
}: {
    params: Promise<{ id: string }>;
}) {
    const { id } = await params;
    if (!UUID.test(id)) notFound();
    const me = await requirePermission('campaigns.read');
    let campaign: Campaign;
    let events: CampaignEventItem[];
    let pauses: Schemas['CampaignPause'][];
    try {
        const [c, e, p] = await Promise.all([
            api<Campaign>(`/campaigns/${id}`),
            api<CampaignEventItem[]>(`/campaigns/${id}/events`),
            api<Schemas['CampaignPause'][]>(`/campaigns/${id}/pauses`),
        ]);
        pauses = p.data;
        campaign = c.data;
        events = e.data;
    } catch (err) {
        if (err instanceof ApiError && err.status === 404) notFound();
        throw err;
    }
    const sender = can(me, 'channels.read')
        ? await api<PhoneNumber[]>('/channels/phone-numbers')
              .then((r) =>
                  r.data.find((n) => n.id === campaign.phone_number_id),
              )
              .catch(() => undefined)
        : undefined;

    return (
        <CampaignDashboard
            campaign={campaign}
            pauses={pauses}
            events={events}
            sender={sender?.display_phone_number}
            timezone={me.user.timezone || 'UTC'}
            perms={{
                edit: can(me, 'campaigns.create'),
                start: can(me, 'campaigns.start'),
                pause: can(me, 'campaigns.pause'),
                del: can(me, 'campaigns.delete'),
            }}
        />
    );
}
