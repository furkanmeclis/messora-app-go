import type { Metadata } from 'next';
import Link from 'next/link';
import { Plus } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { getT } from '@/i18n/server';
import { api } from '@/lib/server/api';
import { can, requirePermission } from '@/lib/server/auth';
import type { Segment } from '@/lib/types';
import { PageHeading } from '../../panel-header';
import { SegmentTable } from './segments-client';

export async function generateMetadata(): Promise<Metadata> {
    const t = await getT();
    return { title: t('contacts.nav.segments') };
}

export default async function SegmentsPage() {
    const me = await requirePermission('segments.read');
    const segments = (await api<Segment[]>('/segments')).data;
    const canManage = can(me, 'segments.manage');
    const canContacts = can(me, 'contacts.read');
    const t = await getT();
    return (
        <>
            <PageHeading
                title={t('contacts.nav.segments')}
                description={t('contacts.segments.description')}
                actions={
                    canManage && (
                        <Button asChild shortcut="n">
                            <Link href="/contacts/segments/new">
                                <Plus /> {t('contacts.segments.new')}
                            </Link>
                        </Button>
                    )
                }
            />
            <SegmentTable segments={segments} canContacts={canContacts} />
        </>
    );
}
