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
import { getT } from '@/i18n/server';
import { api, ApiError } from '@/lib/server/api';
import { requirePermission } from '@/lib/server/auth';
import type { ContactGroup, ContactImport, Tag } from '@/lib/types';
import { PageHeading } from '../../../panel-header';
import { UUID_RE } from '../../shared';
import { ImportWizard } from '../import-client';

export async function generateMetadata(): Promise<Metadata> {
    const t = await getT();
    return { title: t('contacts.imports.title') };
}

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
    const t = await getT();
    if (!imp) {
        return (
            <Empty className="py-16">
                <EmptyHeader>
                    <EmptyMedia variant="icon">
                        <FileX />
                    </EmptyMedia>
                    <EmptyTitle>{t('contacts.imports.notFound')}</EmptyTitle>
                    <EmptyDescription>
                        {t('contacts.imports.notFoundHint')}
                    </EmptyDescription>
                </EmptyHeader>
                <EmptyContent>
                    <Button variant="outline" asChild>
                        <Link href="/contacts/import">
                            {t('contacts.imports.back')}
                        </Link>
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
            <PageHeading
                title={t('contacts.imports.title')}
                description={imp.filename}
            />
            <ImportWizard
                initial={imp}
                groups={groups.map((g) => ({ id: g.id, name: g.name }))}
                tags={tags.map((tag) => ({ name: tag.name, color: tag.color }))}
            />
        </>
    );
}
