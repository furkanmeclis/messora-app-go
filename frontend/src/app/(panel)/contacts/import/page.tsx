import type { Metadata } from 'next';
import {
    Card,
    CardContent,
    CardDescription,
    CardHeader,
    CardTitle,
} from '@/components/ui/card';
import { api } from '@/lib/server/api';
import { requirePermission } from '@/lib/server/auth';
import type { ContactImport } from '@/lib/types';
import { PageHeading } from '../../panel-header';
import { ImportList, UploadCard } from './import-client';

export const metadata: Metadata = { title: 'Import contacts' };

export default async function ImportPage() {
    await requirePermission('contacts.import');
    const imports = (await api<ContactImport[]>('/contact-imports')).data;
    return (
        <>
            <PageHeading
                title="Import contacts"
                description="Add or update contacts from a CSV or Excel file of up to 50 MB."
            />
            <div className="space-y-6">
                <UploadCard />
                <Card>
                    <CardHeader>
                        <CardTitle>Recent imports</CardTitle>
                        <CardDescription>
                            Open one to follow its progress or download the rows
                            that failed.
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
