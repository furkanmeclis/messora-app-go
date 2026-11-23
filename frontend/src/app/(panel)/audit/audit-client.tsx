'use client';

import { useState, useTransition } from 'react';
import { useRouter } from 'next/navigation';
import { X } from 'lucide-react';
import { notify } from '@/components/notify';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Spinner } from '@/components/ui/spinner';
import {
    Table,
    TableBody,
    TableCell,
    TableHead,
    TableHeader,
    TableRow,
} from '@/components/ui/table';
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

    return (
        <div>
            {action && (
                <div className="flex items-center gap-2 border-b p-3 text-sm">
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
            )}
            <Table>
                <TableHeader>
                    <TableRow>
                        <TableHead>{t('audit.columns.when')}</TableHead>
                        <TableHead>{t('audit.columns.who')}</TableHead>
                        <TableHead>{t('audit.columns.action')}</TableHead>
                        <TableHead>{t('audit.columns.details')}</TableHead>
                    </TableRow>
                </TableHeader>
                <TableBody>
                    {entries.length === 0 && (
                        <TableRow>
                            <TableCell
                                colSpan={4}
                                className="py-10 text-center text-muted-foreground"
                            >
                                {t('audit.empty')}
                            </TableCell>
                        </TableRow>
                    )}
                    {entries.map((e) => (
                        <TableRow key={e.id}>
                            <TableCell className="text-sm whitespace-nowrap text-muted-foreground">
                                {formatDateTime(e.occurred_at)}
                            </TableCell>
                            <TableCell className="text-sm">
                                {e.actor.name ??
                                    (e.actor.type === 'system'
                                        ? t('audit.system')
                                        : t('audit.unknown'))}
                                {e.ip && (
                                    <div className="text-xs text-muted-foreground">
                                        {e.ip}
                                    </div>
                                )}
                            </TableCell>
                            <TableCell>
                                <button
                                    type="button"
                                    className="font-mono text-xs hover:underline"
                                    onClick={() =>
                                        router.push(
                                            `/audit?action=${encodeURIComponent(e.action)}`,
                                        )
                                    }
                                >
                                    {e.action}
                                </button>
                            </TableCell>
                            <TableCell
                                className="max-w-md truncate text-xs text-muted-foreground"
                                title={summarize(e.changes)}
                            >
                                {summarize(e.changes)}
                            </TableCell>
                        </TableRow>
                    ))}
                </TableBody>
            </Table>
            {cursor && (
                <div className="border-t p-3 text-center">
                    <Button variant="outline" onClick={more} disabled={pending}>
                        {pending && <Spinner />} {t('audit.loadMore')}
                    </Button>
                </div>
            )}
        </div>
    );
}
