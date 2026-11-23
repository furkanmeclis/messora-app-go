import type { Metadata } from 'next';
import { Card, CardContent } from '@/components/ui/card';
import { api } from '@/lib/server/api';
import { can, requirePermission } from '@/lib/server/auth';
import type { Tag } from '@/lib/types';
import { PageHeading } from '../../panel-header';
import { TagDialog, TagTable } from './tags-client';

export const metadata: Metadata = { title: 'Tags' };

export default async function TagsPage() {
    const me = await requirePermission('contacts.read');
    const tags = (await api<Tag[]>('/tags')).data;
    const canManage = can(me, 'contacts.update');
    return (
        <>
            <PageHeading
                title="Tags"
                description="Labels on contacts. Automations and segments can use them too."
                actions={canManage && <TagDialog />}
            />
            <Card>
                <CardContent className="p-0">
                    <TagTable tags={tags} canManage={canManage} />
                </CardContent>
            </Card>
        </>
    );
}
