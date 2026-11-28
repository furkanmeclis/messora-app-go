'use client';

import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { FileDown } from 'lucide-react';
import { ProformaStatusBadge, useMoney } from '@/components/billing';
import { useFormatDate } from '@/components/display-preferences';
import { notify } from '@/components/notify';
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
import { voidProformaAction } from '../../billing-actions';
import { ReasonButton } from '../billing-ui';

/** Proformas with their company, PDF and void (admin). */
export function ProformasTable({
    rows,
    showTenant = true,
}: {
    rows: Schemas['Proforma'][];
    showTenant?: boolean;
}) {
    const t = useT();
    const money = useMoney();
    const fmtDate = useFormatDate();
    const router = useRouter();
    return (
        <div className="overflow-x-auto rounded-xl border bg-card">
            <Table data-testid="admin-proformas">
                <TableHeader>
                    <TableRow>
                        <TableHead>{t('billing.docs.document')}</TableHead>
                        {showTenant && (
                            <TableHead>
                                {t('billing.admin.subs.company')}
                            </TableHead>
                        )}
                        <TableHead>{t('billing.docs.kind')}</TableHead>
                        <TableHead>{t('billing.docs.total')}</TableHead>
                        <TableHead>{t('billing.docs.status')}</TableHead>
                        <TableHead />
                    </TableRow>
                </TableHeader>
                <TableBody>
                    {rows.length === 0 && (
                        <TableRow>
                            <TableCell
                                colSpan={6}
                                className="py-10 text-center text-muted-foreground"
                            >
                                {t('billing.admin.proformas.empty')}
                            </TableCell>
                        </TableRow>
                    )}
                    {rows.map((f) => (
                        <TableRow key={f.id} data-reference={f.reference}>
                            <TableCell>
                                <div className="font-medium">{f.number}</div>
                                <div className="font-mono text-xs text-muted-foreground">
                                    {f.reference_display} ·{' '}
                                    {fmtDate(f.created_at)}
                                </div>
                            </TableCell>
                            {showTenant && (
                                <TableCell>
                                    {f.tenant ? (
                                        <Link
                                            href={`/tenants/${f.tenant.id}`}
                                            className="hover:underline"
                                        >
                                            {f.tenant.name}
                                        </Link>
                                    ) : (
                                        '—'
                                    )}
                                </TableCell>
                            )}
                            <TableCell>
                                {t(`billing.proforma.kind.${f.kind}`)}
                                {f.renewal &&
                                    ` · ${t('billing.proforma.renewal')}`}
                            </TableCell>
                            <TableCell>
                                {money(f.total)}
                                {Number(f.paid) > 0 && f.status !== 'paid' && (
                                    <div className="text-xs text-muted-foreground">
                                        {t('billing.proforma.paid')}{' '}
                                        {money(f.paid)}
                                    </div>
                                )}
                            </TableCell>
                            <TableCell>
                                <ProformaStatusBadge status={f.status} />
                            </TableCell>
                            <TableCell className="text-right">
                                <div className="flex justify-end gap-1">
                                    {f.status === 'open' &&
                                        Number(f.paid) === 0 && (
                                            <ReasonButton
                                                label={t(
                                                    'billing.admin.proformas.void',
                                                )}
                                                title={t(
                                                    'billing.admin.proformas.voidReason',
                                                )}
                                                variant="ghost"
                                                onConfirm={async (reason) => {
                                                    const ok = notify(
                                                        await voidProformaAction(
                                                            f.id,
                                                            reason,
                                                        ),
                                                    );
                                                    if (ok) router.refresh();
                                                    return ok;
                                                }}
                                            />
                                        )}
                                    <Button
                                        asChild
                                        size="icon-xs"
                                        variant="ghost"
                                    >
                                        <a
                                            href={`/bff/billing/proformas/${f.id}/pdf`}
                                            download
                                            aria-label={t(
                                                'billing.docs.download',
                                            )}
                                        >
                                            <FileDown />
                                        </a>
                                    </Button>
                                </div>
                            </TableCell>
                        </TableRow>
                    ))}
                </TableBody>
            </Table>
        </div>
    );
}
