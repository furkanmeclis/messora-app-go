import type { Metadata } from 'next';
import Link from 'next/link';
import { Upload } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { getT } from '@/i18n/server';
import { api } from '@/lib/server/api';
import { can, requirePermission } from '@/lib/server/auth';
import { isConsentStatus } from '@/lib/consent';
import type { Contact, ContactGroup, Segment, Tag } from '@/lib/types';
import { PageHeading } from '../panel-header';
import { ContactsTable } from './contacts-table';
import {
    ContactFilters,
    ExportMenu,
    NewContactDialog,
} from './contacts-client';

export async function generateMetadata(): Promise<Metadata> {
    const t = await getT();
    return { title: t('contacts.title') };
}

const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;

export default async function ContactsPage({
    searchParams,
}: {
    searchParams: Promise<Record<string, string | undefined>>;
}) {
    const me = await requirePermission('contacts.read');
    const t = await getT();
    const sp = await searchParams;
    const filters = {
        q: (sp.q ?? '').trim().slice(0, 100),
        tag_id: sp.tag_id && UUID.test(sp.tag_id) ? sp.tag_id : '',
        group_id: sp.group_id && UUID.test(sp.group_id) ? sp.group_id : '',
        segment_id:
            sp.segment_id && UUID.test(sp.segment_id) ? sp.segment_id : '',
        opted_out:
            sp.opted_out === 'true' || sp.opted_out === 'false'
                ? sp.opted_out
                : '',
        marketing_consent: isConsentStatus(sp.marketing_consent)
            ? sp.marketing_consent
            : '',
        service_consent: isConsentStatus(sp.service_consent)
            ? sp.service_consent
            : '',
        reachability:
            sp.reachability === 'meta_marketing_stopped' ||
            sp.reachability === 'undeliverable'
                ? sp.reachability
                : '',
        ai_profile: sp.ai_profile === 'true' ? 'true' : '',
    };
    const query = new URLSearchParams();
    for (const [k, v] of Object.entries(filters)) if (v) query.set(k, v);
    const canSegments = can(me, 'segments.read');
    const SORTS = ['newest', 'oldest', 'name_asc', 'name_desc', 'last_message'];
    const sort = SORTS.includes(sp.sort ?? '') ? sp.sort! : 'newest';
    const pageSize = [25, 50, 100, 200].includes(Number(sp.limit))
        ? Number(sp.limit)
        : 50;
    const page = Math.max(
        1,
        Math.min(100000, Number.parseInt(sp.page ?? '1', 10) || 1),
    );
    const listQuery = new URLSearchParams(query);
    listQuery.set('page', String(page));
    listQuery.set('limit', String(pageSize));
    listQuery.set('sort', sort);

    const [list, tags, groups, segments] = await Promise.all([
        api<Contact[]>(`/contacts?${listQuery}`),
        api<Tag[]>('/tags').then((r) => r.data),
        api<ContactGroup[]>('/groups').then((r) => r.data),
        canSegments
            ? api<Segment[]>('/segments').then((r) => r.data)
            : Promise.resolve([] as Segment[]),
    ]);

    return (
        <>
            <PageHeading
                title={t('contacts.title')}
                description={t('contacts.description')}
                actions={
                    <div className="flex flex-wrap gap-2">
                        {can(me, 'contacts.import') && (
                            <Button variant="outline" asChild shortcut="i">
                                <Link href="/contacts/import">
                                    <Upload /> {t('contacts.import')}
                                </Link>
                            </Button>
                        )}
                        {can(me, 'contacts.export') && (
                            <ExportMenu query={query.toString()} />
                        )}
                        {can(me, 'contacts.create') && <NewContactDialog />}
                    </div>
                }
            />
            <div className="space-y-4">
                <ContactFilters
                    key={query.toString()}
                    filters={filters}
                    tags={tags.map((tag) => ({ id: tag.id, name: tag.name }))}
                    groups={groups.map((g) => ({ id: g.id, name: g.name }))}
                    segments={segments.map((s) => ({
                        id: s.id,
                        name: s.name,
                    }))}
                />
                <ContactsTable
                    contacts={list.data}
                    total={list.total ?? list.data.length}
                    page={page}
                    pageSize={pageSize}
                    sort={sort}
                    filters={filters}
                    filtered={query.size > 0}
                    tags={tags.map((tag) => ({ id: tag.id, name: tag.name }))}
                    groups={groups.map((g) => ({ id: g.id, name: g.name }))}
                    canUpdate={can(me, 'contacts.update')}
                    canDelete={can(me, 'contacts.delete')}
                />
            </div>
        </>
    );
}
