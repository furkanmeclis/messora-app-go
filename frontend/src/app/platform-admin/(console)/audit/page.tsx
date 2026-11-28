import type { Metadata } from 'next';
import { ScrollText } from 'lucide-react';
import { getT } from '@/i18n/server';
import { papi } from '@/lib/server/platform';
import type { Schemas } from '@/lib/types';
import { Heading } from '../heading';
import { AuditList } from './audit-list';

export async function generateMetadata(): Promise<Metadata> {
    const t = await getT();
    return { title: t('platform.audit.title') };
}

export default async function PlatformAuditPage() {
    const [t, first] = await Promise.all([
        getT(),
        papi<Schemas['PlatformAuditLog'][]>('/platform/audit-logs?limit=50'),
    ]);
    return (
        <>
            <Heading
                icon={<ScrollText />}
                title={t('platform.audit.title')}
                description={t('platform.audit.description')}
            />
            <AuditList initial={first.data} initialCursor={first.nextCursor} />
        </>
    );
}
