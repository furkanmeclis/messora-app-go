import type { Metadata } from 'next';
import Link from 'next/link';
import { FileX } from 'lucide-react';
import { Button } from '@/components/ui/button';
import {
    Empty,
    EmptyContent,
    EmptyDescription,
    EmptyHeader,
    EmptyMedia,
    EmptyTitle,
} from '@/components/ui/empty';
import { api, ApiError } from '@/lib/server/api';
import { requirePermission } from '@/lib/server/auth';
import type { ContactGroup, ContactImport, Tag } from '@/lib/types';
import { PageHeading } from '../../../panel-header';
import { UUID_RE } from '../../shared';
import { ImportWizard } from '../import-client';

export const metadata: Metadata = { title: 'Import contacts' };

async function loadImport(id: string): Promise<ContactImport | null> {
    if (!UUID_RE.test(id)) return null;
    try {
        return (await api<ContactImport>(`/contact-imports/${id}`)).data;
    } catch (err) {
        if (err instanceof ApiError && err.status === 404) return null;
        throw err;
    }
}

export default async function ImportDetailPage({
    params,
}: {
    params: Promise<{ importId: string }>;
}) {
    await requirePermission('contacts.import');
    const { importId } = await params;
    const imp = await loadImport(importId);
    if (!imp) {
        return (
            <Empty className="py-16">
                <EmptyHeader>
                    <EmptyMedia variant="icon">
                        <FileX />
                    </EmptyMedia>
                    <EmptyTitle>Import not found</EmptyTitle>
                    <EmptyDescription>
                        It may have been removed, or the link is wrong.
                    </EmptyDescription>
                </EmptyHeader>
                <EmptyContent>
                    <Button variant="outline" asChild>
                        <Link href="/contacts/import">Back to imports</Link>
                    </Button>
                </EmptyContent>
            </Empty>
        );
    }
    const [groups, tags] = await Promise.all([
        api<ContactGroup[]>('/groups').then((r) => r.data),
        api<Tag[]>('/tags').then((r) => r.data),
    ]);
    return (
        <>
            <PageHeading title="Import contacts" description={imp.filename} />
            <ImportWizard
                initial={imp}
                groups={groups.map((g) => ({ id: g.id, name: g.name }))}
                tags={tags.map((t) => ({ name: t.name, color: t.color }))}
            />
        </>
    );
}
