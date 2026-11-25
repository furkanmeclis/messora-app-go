'use client';

import { useState, useTransition } from 'react';
import { useRouter } from 'next/navigation';
import { Filter, X } from 'lucide-react';
import { ClientDataTable } from '@/components/client-data-table';
import { notify } from '@/components/notify';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Spinner } from '@/components/ui/spinner';
import { useFormatDateTime } from '@/components/display-preferences';
import { useT } from '@/i18n/client';
import type { AuditLog } from '@/lib/types';
import { loadAuditPage } from '../actions';

function summarize(changes: Record<string, unknown>): string {
    return Object.entries(changes)
        .map(([k, v]) => {
            if (Array.isArray(v) && v.length === 2)
                return `${k}: ${String(v[0])} → ${String(v[1])}`;
            return `${k}: ${typeof v === 'object' ? JSON.stringify(v) : String(v)}`;
        })
        .join(' · ');
}

export function AuditTable({
    initial,
    initialCursor,
    action,
}: {
    initial: AuditLog[];
    initialCursor?: string;
    action?: string;
}) {
    const t = useT();
    const formatDateTime = useFormatDateTime();
    const router = useRouter();
    const [entries, setEntries] = useState(initial);
    const [cursor, setCursor] = useState(initialCursor);
    const [pending, startTransition] = useTransition();

    const more = () =>
        startTransition(async () => {
            const r = await loadAuditPage(cursor, action);
            if (notify(r)) {
                setEntries((e) => [...e, ...r.data.entries]);
                setCursor(r.data.next);
            }
        });
    const filterBy = (a: string) =>
        router.push(`/audit?action=${encodeURIComponent(a)}`);
    const who = (e: AuditLog) =>
        e.actor.name ??
        (e.actor.type === 'system' ? t('audit.system') : t('audit.unknown'));

    return (
        <div className="space-y-3">
            <ClientDataTable
                rows={entries}
                rowId={(e) => e.id}
                storageKey="audit"
                initialSort="-when"
                pageSize={50}
                searchText={(e) =>
                    `${who(e)} ${e.ip ?? ''} ${e.action} ${summarize(e.changes)}`
                }
                searchPlaceholder={t('table.search')}
                toolbar={
                    action && (
                        <div className="flex items-center gap-2 text-sm">
                            {t('audit.filteredBy')}
                            <Badge variant="secondary" className="font-mono">
                                {action}
                            </Badge>
                            <Button
                                size="icon"
                                variant="ghost"
                                aria-label={t('audit.clearFilter')}
                                onClick={() => router.push('/audit')}
                            >
                                <X />
                            </Button>
                        </div>
                    )
                }
                empty={
                    <p className="py-10 text-center text-sm text-muted-foreground">
                        {t('audit.empty')}
                    </p>
                }
                noMatch={
                    <p className="p-8 text-center text-sm text-muted-foreground">
                        {t('table.noMatch')}
                    </p>
                }
                columns={[
                    {
                        id: 'when',
                        header: t('audit.columns.when'),
                        sortValue: (e) => e.occurred_at,
                        cell: (e) => (
                            <span className="text-sm whitespace-nowrap text-muted-foreground">
                                {formatDateTime(e.occurred_at)}
                            </span>
                        ),
                    },
                    {
                        id: 'who',
                        header: t('audit.columns.who'),
                        sortValue: (e) => who(e),
                        cell: (e) => (
                            <div className="text-sm">
                                {who(e)}
                                {e.ip && (
                                    <div className="text-xs text-muted-foreground">
                                        {e.ip}
                                    </div>
                                )}
                            </div>
                        ),
                    },
                    {
                        id: 'action',
                        header: t('audit.columns.action'),
                        hideable: false,
                        sortValue: (e) => e.action,
                        cell: (e) => (
                            <button
                                type="button"
                                className="font-mono text-xs hover:underline"
                                onClick={() => filterBy(e.action)}
                            >
                                {e.action}
                            </button>
                        ),
                    },
                    {
                        id: 'details',
                        header: t('audit.columns.details'),
                        cell: (e) => (
                            <div
                                className="max-w-md truncate text-xs text-muted-foreground"
                                title={summarize(e.changes)}
                            >
                                {summarize(e.changes)}
                            </div>
                        ),
                    },
                ]}
                rowMenu={(e) => [
                    [
                        {
                            id: 'filter',
                            label: t('audit.filterBy', { action: e.action }),
                            icon: Filter,
                            disabled: e.action === action,
                            run: () => filterBy(e.action),
                        },
                    ],
                ]}
            />
            {cursor && (
                <div className="text-center">
                    <Button variant="outline" onClick={more} disabled={pending}>
                        {pending && <Spinner />} {t('audit.loadMore')}
                    </Button>
                </div>
            )}
        </div>
    );
}
