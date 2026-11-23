import type { Metadata } from 'next';
import {
    Card,
    CardContent,
    CardDescription,
    CardHeader,
    CardTitle,
} from '@/components/ui/card';
import { getT } from '@/i18n/server';
import { api } from '@/lib/server/api';
import { requirePermission } from '@/lib/server/auth';
import type { ContactImport } from '@/lib/types';
import { PageHeading } from '../../panel-header';
import { ImportList, UploadCard } from './import-client';

export async function generateMetadata(): Promise<Metadata> {
    const t = await getT();
    return { title: t('contacts.imports.title') };
}

export default async function ImportPage() {
    await requirePermission('contacts.import');
    const imports = (await api<ContactImport[]>('/contact-imports')).data;
    const t = await getT();
    return (
        <>
            <PageHeading
                title={t('contacts.imports.title')}
                description={t('contacts.imports.description')}
            />
            <div className="space-y-6">
                <UploadCard />
                <Card>
                    <CardHeader>
                        <CardTitle>{t('contacts.imports.recent')}</CardTitle>
                        <CardDescription>
                            {t('contacts.imports.recentHint')}
                        </CardDescription>
                    </CardHeader>
                    <CardContent className="p-0">
                        <ImportList imports={imports} />
                    </CardContent>
                </Card>
            </div>
        </>
    );
}
