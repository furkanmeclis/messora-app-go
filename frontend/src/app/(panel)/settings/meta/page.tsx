import type { Metadata } from 'next';
import { getT } from '@/i18n/server';
import { api } from '@/lib/server/api';
import { requirePermission } from '@/lib/server/auth';
import type { MetaConnection, WhatsAppAccount } from '@/lib/types';
import { MetaConnectionView } from './meta-connection-client';

export async function generateMetadata(): Promise<Metadata> {
    const t = await getT();
    return { title: t('channels.meta.title') };
}

/** Settings → WhatsApp / Meta: the company's own Meta app (phase 17 WP5). */
export default async function MetaConnectionPage() {
    await requirePermission('channels.manage');
    const [connection, accounts] = await Promise.all([
        api<MetaConnection>('/channels/meta-connection').then((r) => r.data),
        api<WhatsAppAccount[]>('/channels/accounts').then((r) => r.data),
    ]);
    return <MetaConnectionView connection={connection} accounts={accounts} />;
}
