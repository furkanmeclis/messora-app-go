'use client';

import { useState, useTransition } from 'react';
import Link from 'next/link';
import { notify } from '@/components/notify';
import { useFormatDateTime } from '@/components/display-preferences';
import { Button } from '@/components/ui/button';
import {
    Table,
    TableBody,
    TableCell,
    TableHead,
    TableHeader,
    TableRow,
} from '@/components/ui/table';
import { useT } from '@/i18n/client';
import type { Schemas } from '@/lib/types';
import { loadAuditAction } from '../../actions';

type Entry = Schemas['PlatformAuditLog'];

function details(e: Entry): string {
    const parts = Object.entries(e.changes).map(
        ([k, v]) => `${k}: ${typeof v === 'string' ? v : JSON.stringify(v)}`,
    );
    return parts.join(' · ');
}

export function AuditList({
    initial,
    initialCursor,
}: {
    initial: Entry[];
    initialCursor?: string;
}) {
    const t = useT();
    const fmt = useFormatDateTime();
    const [rows, setRows] = useState(initial);
    const [cursor, setCursor] = useState(initialCursor);
    const [pending, start] = useTransition();
    return (
        <div className="space-y-4">
            <div className="overflow-x-auto rounded-xl border bg-card">
                <Table>
                    <TableHeader>
                        <TableRow>
                            <TableHead>
                                {t('platform.audit.columns.when')}
                            </TableHead>
                            <TableHead>
                                {t('platform.audit.columns.who')}
                            </TableHead>
                            <TableHead>
                                {t('platform.audit.columns.action')}
                            </TableHead>
                            <TableHead>
                                {t('platform.audit.columns.company')}
                            </TableHead>
                            <TableHead>
                                {t('platform.audit.columns.details')}
                            </TableHead>
                        </TableRow>
                    </TableHeader>
                    <TableBody>
                        {rows.length === 0 && (
                            <TableRow>
                                <TableCell
                                    colSpan={5}
                                    className="py-10 text-center text-muted-foreground"
                                >
                                    {t('platform.audit.empty')}
                                </TableCell>
                            </TableRow>
                        )}
                        {rows.map((e) => (
                            <TableRow
                                key={e.id}
                                data-testid="platform-audit-row"
                            >
                                <TableCell className="text-sm whitespace-nowrap text-muted-foreground">
                                    {fmt(e.occurred_at)}
                                </TableCell>
                                <TableCell className="text-sm">
                                    {e.actor_email ?? '—'}
                                </TableCell>
                                <TableCell className="font-mono text-xs">
                                    {e.action}
                                </TableCell>
                                <TableCell className="text-sm">
                                    {e.tenant ? (
                                        <Link
                                            href={`/tenants/${e.tenant.id}`}
                                            className="hover:underline"
                                        >
                                            {e.tenant.slug}
                                        </Link>
                                    ) : (
                                        '—'
                                    )}
                                </TableCell>
                                <TableCell className="max-w-96 truncate text-xs text-muted-foreground">
                                    {details(e)}
                                </TableCell>
                            </TableRow>
                        ))}
                    </TableBody>
                </Table>
            </div>
            {cursor && (
                <Button
                    variant="outline"
                    disabled={pending}
                    onClick={() =>
                        start(async () => {
                            const r = await loadAuditAction(cursor);
                            if (!notify(r)) return;
                            setRows((x) => [...x, ...r.data.rows]);
                            setCursor(r.data.next);
                        })
                    }
                >
                    {t('platform.audit.more')}
                </Button>
            )}
        </div>
    );
}
