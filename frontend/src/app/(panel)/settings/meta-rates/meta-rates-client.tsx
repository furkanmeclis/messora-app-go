'use client';

import { useState, useTransition } from 'react';
import { useRouter } from 'next/navigation';
import { toast } from 'sonner';
import { ClientDataTable } from '@/components/client-data-table';
import { useFormatDateTime } from '@/components/display-preferences';
import { Badge } from '@/components/ui/badge';
import { FileDrop } from '@/components/ui/file-drop';
import { useT } from '@/i18n/client';
import type { Schemas } from '@/lib/types';
import { Section } from '../section';

type Rate = Schemas['MetaRate'];

const EXAMPLE = `market,countries,category,price,currency,effective_from,tier_lower,tier_upper
Turkey,TR,marketing,0.0000,USD,2026-10-01,,
Rest of Western Europe,AT;BE;CH,marketing,0.0000,USD,2026-10-01,,
Turkey,TR,utility,0.0000,USD,2026-10-01,1,100000`;

/** Uploads Meta's rate card (CSV) and lists the imported rates. */
export function MetaRates({
    rates,
    readOnly,
}: {
    rates: Rate[];
    /** Workspaces only read the card; the platform admin panel imports it. */
    readOnly: boolean;
}) {
    const t = useT();
    const router = useRouter();
    const formatDateTime = useFormatDateTime();
    const [busy, start] = useTransition();
    const [key, setKey] = useState(0);
    const [problems, setProblems] = useState<string | null>(null);

    const upload = (file: File | null) => {
        if (!file) return;
        start(async () => {
            const form = new FormData();
            form.append('file', file);
            const res = await fetch('/bff/meta-rates', {
                method: 'POST',
                body: form,
            });
            const body = (await res.json().catch(() => null)) as {
                data?: Schemas['MetaRateImport'];
                error?: { message?: string };
            } | null;
            setKey((k) => k + 1);
            if (!res.ok || !body?.data) {
                const msg =
                    body?.error?.message ?? t('common.somethingWentWrong');
                setProblems(msg);
                toast.error(t('settings.metaRates.importFailed'));
                return;
            }
            setProblems(null);
            toast.success(
                t('settings.metaRates.imported', {
                    rates: body.data.rates,
                    markets: body.data.markets,
                }),
            );
            router.refresh();
        });
    };

    const tier = (r: Rate) =>
        r.tier_lower == null && r.tier_upper == null
            ? '—'
            : `${r.tier_lower ?? 1} – ${r.tier_upper ?? '∞'}`;

    return (
        <div className="space-y-6">
            <Section
                title={t('settings.metaRates.title')}
                description={t('settings.metaRates.description')}
            >
                <div className="space-y-4">
                    {readOnly && (
                        <p
                            role="note"
                            className="rounded-lg border bg-muted/40 p-3 text-sm text-muted-foreground"
                        >
                            {t('settings.metaRates.managedByPlatform')}
                        </p>
                    )}
                    {!readOnly && (
                        <FileDrop
                            key={key}
                            compact
                            accept=".csv,text/csv"
                            onFile={upload}
                            disabled={busy}
                            hint={t('settings.metaRates.hint')}
                        />
                    )}
                    {problems && (
                        <p
                            role="alert"
                            className="rounded-lg border border-destructive/40 bg-destructive/5 p-3 text-sm whitespace-pre-wrap text-destructive"
                        >
                            {problems}
                        </p>
                    )}
                    <details className="text-sm">
                        <summary className="cursor-pointer text-muted-foreground">
                            {t('settings.metaRates.format')}
                        </summary>
                        <p className="mt-2 text-muted-foreground">
                            {t('settings.metaRates.formatHelp')}
                        </p>
                        <pre className="mt-2 overflow-x-auto rounded-lg bg-muted p-3 font-mono text-xs">
                            {EXAMPLE}
                        </pre>
                    </details>
                </div>
            </Section>
            <ClientDataTable
                rows={rates}
                rowId={(r) => r.id}
                storageKey="meta-rates"
                initialSort="market"
                searchText={(r) =>
                    `${r.market} ${r.countries.join(' ')} ${r.category} ${r.currency}`
                }
                searchPlaceholder={t('table.search')}
                empty={
                    <p className="p-6 text-sm text-muted-foreground">
                        {t('settings.metaRates.empty')}
                    </p>
                }
                noMatch={
                    <p className="p-8 text-center text-sm text-muted-foreground">
                        {t('table.noMatch')}
                    </p>
                }
                columns={[
                    {
                        id: 'market',
                        header: t('settings.metaRates.market'),
                        hideable: false,
                        sortValue: (r) => r.market,
                        cell: (r) => (
                            <div>
                                <div className="font-medium">{r.market}</div>
                                {r.countries.length > 0 && (
                                    <div className="max-w-64 truncate text-xs text-muted-foreground">
                                        {r.countries.join(', ')}
                                    </div>
                                )}
                            </div>
                        ),
                    },
                    {
                        id: 'category',
                        header: t('settings.metaRates.category'),
                        sortValue: (r) => r.category,
                        cell: (r) => (
                            <Badge variant="outline">{r.category}</Badge>
                        ),
                    },
                    {
                        id: 'price',
                        header: t('settings.metaRates.price'),
                        sortValue: (r) => Number(r.price),
                        cell: (r) => (
                            <span className="font-mono whitespace-nowrap">
                                {r.price} {r.currency}
                            </span>
                        ),
                    },
                    {
                        id: 'tier',
                        header: t('settings.metaRates.tier'),
                        sortValue: (r) => r.tier_lower ?? 0,
                        cell: (r) => (
                            <span className="whitespace-nowrap">{tier(r)}</span>
                        ),
                    },
                    {
                        id: 'validity',
                        header: t('settings.metaRates.validity'),
                        sortValue: (r) => r.effective_from,
                        cell: (r) => (
                            <span className="whitespace-nowrap">
                                {r.effective_to
                                    ? t('settings.metaRates.between', {
                                          from: r.effective_from,
                                          to: r.effective_to,
                                      })
                                    : t('settings.metaRates.since', {
                                          from: r.effective_from,
                                      })}
                            </span>
                        ),
                    },
                    {
                        id: 'imported',
                        header: t('settings.metaRates.importedAt'),
                        sortValue: (r) => r.imported_at,
                        cell: (r) => (
                            <span className="text-sm whitespace-nowrap text-muted-foreground">
                                {formatDateTime(r.imported_at)}
                            </span>
                        ),
                    },
                ]}
            />
        </div>
    );
}
