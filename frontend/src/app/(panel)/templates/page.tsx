import type { Metadata } from 'next';
import Link from 'next/link';
import { RefreshCw } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { api } from '@/lib/server/api';
import { can, requirePermission } from '@/lib/server/auth';
import type { Template, WhatsAppAccount } from '@/lib/types';
import { TEMPLATE_STATUSES } from '@/lib/templates';
import { PageHeading } from '../panel-header';
import {
    CreateTemplateDialog,
    TemplateFilters,
    TemplateList,
} from './templates-client';

export const metadata: Metadata = { title: 'Templates' };

const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;

export default async function TemplatesPage({
    searchParams,
}: {
    searchParams: Promise<Record<string, string | undefined>>;
}) {
    const me = await requirePermission('templates.read');
    const sp = await searchParams;
    const account = sp.account && UUID.test(sp.account) ? sp.account : '';
    const status = TEMPLATE_STATUSES.find((s) => s === sp.status) ?? '';
    const query = new URLSearchParams();
    if (account) query.set('account_id', account);
    if (status) query.set('status', status);

    const [templates, accounts] = await Promise.all([
        api<Template[]>(`/templates?${query}`).then((r) => r.data),
        can(me, 'channels.read')
            ? api<WhatsAppAccount[]>('/channels/accounts')
                  .then((r) => r.data.map((a) => ({ id: a.id, name: a.name })))
                  .catch(() => null)
            : Promise.resolve(null),
    ]);
    // Without channels.read the accounts are known from the templates.
    const accountOptions =
        accounts ??
        Array.from(
            new Map(
                templates.map((t) => [
                    t.account_id,
                    { id: t.account_id, name: t.account_name ?? t.account_id },
                ]),
            ).values(),
        );
    const manage = can(me, 'templates.manage');

    return (
        <>
            <PageHeading
                title="Templates"
                description="Message templates approved by Meta can be sent outside the 24-hour window and in campaigns."
                actions={
                    <div className="flex flex-wrap gap-2">
                        {can(me, 'channels.read') && (
                            <Button variant="outline" asChild>
                                <Link href="/channels">
                                    <RefreshCw /> Sync from Meta
                                </Link>
                            </Button>
                        )}
                        {manage && accounts && accounts.length > 0 && (
                            <CreateTemplateDialog accounts={accounts} />
                        )}
                    </div>
                }
            />
            <div className="space-y-4">
                <TemplateFilters
                    accounts={accountOptions}
                    account={account}
                    status={status}
                />
                <TemplateList
                    templates={templates}
                    manage={manage}
                    filtered={!!(account || status)}
                    hasAccounts={accountOptions.length > 0 || !!account}
                />
            </div>
        </>
    );
}
