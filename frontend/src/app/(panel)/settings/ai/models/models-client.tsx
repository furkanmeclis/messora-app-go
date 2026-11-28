'use client';

import { useState, useTransition } from 'react';
import { useRouter } from 'next/navigation';
import { Check, Cpu, Pencil, Plus, Trash2 } from 'lucide-react';
import { toast } from 'sonner';
import { ClientDataTable } from '@/components/client-data-table';
import { useConfirm } from '@/components/confirm';
import { notify } from '@/components/notify';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
    Empty,
    EmptyDescription,
    EmptyHeader,
    EmptyMedia,
    EmptyTitle,
} from '@/components/ui/empty';
import { FileDrop } from '@/components/ui/file-drop';
import { DatePicker } from '@/components/ui/date-picker';
import { useLocale, useT } from '@/i18n/client';
import { intlLocale } from '@/i18n/config';
import type { Schemas } from '@/lib/types';
import { deletePriceAction } from '@/app/platform-admin/price-actions';
import { PriceDialog } from './price-dialog';

type Model = Schemas['AIModel'];
type Price = Schemas['AIPrice'];
type Caps = Schemas['AICapabilities'];

const CAPS: (keyof Caps)[] = [
    'tools',
    'structured_output',
    'vision',
    'thinking',
];

/** Model catalogue with today's prices, and the price table. */
export function ModelsClient({
    models,
    prices,
    fakeAvailable,
    canManage,
    pricesOnly = false,
}: {
    models: Model[];
    prices: Price[];
    fakeAvailable: boolean;
    /** Prices can be changed: only in the platform admin panel. */
    canManage: boolean;
    /** Show only the price table (the platform admin panel). */
    pricesOnly?: boolean;
}) {
    const t = useT();
    const router = useRouter();
    const ask = useConfirm();
    const locale = intlLocale(useLocale());
    const [editing, setEditing] = useState<
        | { price: Price; model?: undefined }
        | { price?: undefined; model?: Model }
        | null
    >(null);
    const [busy, start] = useTransition();
    const [effective, setEffective] = useState('');
    const [fileKey, setFileKey] = useState(0);
    const [problem, setProblem] = useState<string | null>(null);
    const num = (n: number | null | undefined) =>
        n == null ? (
            <span className="text-muted-foreground">{t('ai.unknown')}</span>
        ) : (
            n.toLocaleString(locale)
        );

    const upload = (file: File | null) => {
        if (!file) return;
        start(async () => {
            const form = new FormData();
            if (effective) form.append('effective_from', effective);
            form.append('file', file);
            const res = await fetch('/bff/ai-prices', {
                method: 'POST',
                body: form,
            });
            const body = (await res.json().catch(() => null)) as {
                data?: Schemas['AIPriceImport'];
                error?: { message?: string };
            } | null;
            setFileKey((k) => k + 1);
            if (!res.ok || !body?.data) {
                setProblem(
                    body?.error?.message ?? t('common.somethingWentWrong'),
                );
                toast.error(t('ai.prices.importFailed'));
                return;
            }
            setProblem(null);
            toast.success(
                t('ai.prices.imported', {
                    prices: body.data.prices,
                    skipped: body.data.skipped,
                }),
            );
            router.refresh();
        });
    };

    const removePrice = async (p: Price) => {
        const ok = await ask({
            title: t('ai.prices.deleteTitle'),
            description: t('ai.prices.deleteBody'),
            confirmLabel: t('ai.prices.delete'),
            destructive: true,
        });
        if (!ok) return;
        start(async () => {
            if (notify(await deletePriceAction(p.id))) router.refresh();
        });
    };

    const priceCell = (p: Price | null) =>
        p ? (
            <div className="space-y-1 whitespace-nowrap">
                <span className="font-mono text-sm">
                    {p.input} / {p.output} {p.currency}
                </span>
                <div>
                    <Badge
                        variant="outline"
                        className={
                            p.confirmed
                                ? 'border-success/40 bg-success/10 text-success'
                                : 'border-warning/40 bg-warning/10 text-warning'
                        }
                    >
                        {p.confirmed
                            ? t('ai.models.confirmed')
                            : t('ai.models.unconfirmed')}
                    </Badge>
                </div>
            </div>
        ) : (
            <span className="text-sm text-muted-foreground">
                {t('ai.models.noPrice')}
            </span>
        );

    return (
        <div className="space-y-10">
            {!canManage && (
                <p
                    role="note"
                    className="rounded-lg border bg-muted/40 p-3 text-sm text-muted-foreground"
                >
                    {t('ai.prices.managedByPlatform')}
                </p>
            )}
            {!pricesOnly && (
                <section className="space-y-3">
                    <div>
                        <h2 className="font-semibold tracking-tight">
                            {t('ai.models.title')}
                        </h2>
                        <p className="text-sm text-muted-foreground">
                            {t('ai.models.description')}
                        </p>
                    </div>
                    <ClientDataTable
                        rows={models}
                        rowId={(m) => `${m.provider_id}/${m.model_id}`}
                        storageKey="ai-models"
                        initialSort="model"
                        rowTestId="ai-model-row"
                        searchText={(m) =>
                            `${m.model_id} ${m.display_name} ${m.provider_name}`
                        }
                        searchPlaceholder={t('table.search')}
                        empty={
                            <Empty className="py-12">
                                <EmptyHeader>
                                    <EmptyMedia variant="icon">
                                        <Cpu />
                                    </EmptyMedia>
                                    <EmptyTitle>
                                        {t('ai.models.empty')}
                                    </EmptyTitle>
                                    <EmptyDescription>
                                        {t('ai.models.emptyHint')}
                                    </EmptyDescription>
                                </EmptyHeader>
                            </Empty>
                        }
                        noMatch={
                            <p className="p-8 text-center text-sm text-muted-foreground">
                                {t('table.noMatch')}
                            </p>
                        }
                        columns={[
                            {
                                id: 'model',
                                header: t('ai.models.columns.model'),
                                hideable: false,
                                sortValue: (m) => m.model_id,
                                cell: (m) => (
                                    <div className="min-w-0">
                                        <div className="font-medium">
                                            {m.display_name}
                                        </div>
                                        <div className="font-mono text-xs text-muted-foreground">
                                            {m.model_id}
                                            {m.lifecycle &&
                                                m.lifecycle !== 'active' &&
                                                ` · ${m.lifecycle}`}
                                        </div>
                                    </div>
                                ),
                            },
                            {
                                id: 'provider',
                                header: t('ai.models.columns.provider'),
                                sortValue: (m) => m.provider_name,
                                cell: (m) => (
                                    <span className="text-sm whitespace-nowrap">
                                        {m.provider_name}
                                        <span className="ml-1.5 text-xs text-muted-foreground">
                                            {m.key_source === 'platform'
                                                ? t('plans.ai.sourcePlatform')
                                                : t('plans.ai.sourceOwn')}
                                        </span>
                                    </span>
                                ),
                            },
                            {
                                id: 'context',
                                header: t('ai.models.columns.context'),
                                sortValue: (m) => m.context_window ?? -1,
                                cell: (m) => (
                                    <span className="text-sm tabular-nums">
                                        {num(m.context_window)}
                                    </span>
                                ),
                            },
                            {
                                id: 'output',
                                header: t('ai.models.columns.output'),
                                sortValue: (m) => m.max_output ?? -1,
                                cell: (m) => (
                                    <span className="text-sm tabular-nums">
                                        {num(m.max_output)}
                                    </span>
                                ),
                            },
                            {
                                id: 'capabilities',
                                header: t('ai.models.columns.capabilities'),
                                cell: (m) => (
                                    <div className="flex max-w-xs flex-wrap gap-1">
                                        {CAPS.map((c) => {
                                            const v = m.capabilities[c];
                                            return (
                                                <Badge
                                                    key={c}
                                                    variant="outline"
                                                    className={
                                                        v === true
                                                            ? 'border-success/40 text-success'
                                                            : 'text-muted-foreground'
                                                    }
                                                    title={
                                                        v == null
                                                            ? t('ai.unknown')
                                                            : undefined
                                                    }
                                                >
                                                    {t(
                                                        `ai.models.capabilities.${c}`,
                                                    )}
                                                    {v === false
                                                        ? ' ✕'
                                                        : v == null
                                                          ? ` (${t('ai.unknown')})`
                                                          : ''}
                                                </Badge>
                                            );
                                        })}
                                    </div>
                                ),
                            },
                            {
                                id: 'price',
                                header: t('ai.models.columns.price'),
                                sortValue: (m) =>
                                    m.price ? Number(m.price.input) : -1,
                                cell: (m) => priceCell(m.price),
                            },
                        ]}
                        rowActions={
                            canManage
                                ? (m) => (
                                      <Button
                                          size="sm"
                                          variant="outline"
                                          onClick={() =>
                                              setEditing(
                                                  m.price
                                                      ? { price: m.price }
                                                      : { model: m },
                                              )
                                          }
                                      >
                                          {m.price ? <Pencil /> : <Plus />}
                                          {m.price
                                              ? t('ai.models.editPrice')
                                              : t('ai.models.addPrice')}
                                      </Button>
                                  )
                                : undefined
                        }
                    />
                </section>
            )}

            <section className="space-y-3">
                <div className="flex flex-wrap items-start justify-between gap-4">
                    <div>
                        <h2 className="font-semibold tracking-tight">
                            {t('ai.prices.title')}
                        </h2>
                        <p className="max-w-3xl text-sm text-muted-foreground">
                            {t('ai.prices.description')}
                        </p>
                    </div>
                    {canManage && (
                        <Button
                            variant="outline"
                            onClick={() => setEditing({})}
                        >
                            <Plus /> {t('ai.models.addPrice')}
                        </Button>
                    )}
                </div>
                {canManage && (
                    <div className="grid gap-3 sm:grid-cols-[1fr_auto] sm:items-end">
                        <FileDrop
                            key={fileKey}
                            compact
                            accept=".json,application/json"
                            onFile={upload}
                            disabled={busy}
                            hint={t('ai.prices.importHint')}
                        />
                        <label className="space-y-1 text-sm">
                            <span className="text-muted-foreground">
                                {t('ai.prices.effectiveFrom')}
                            </span>
                            <DatePicker
                                value={effective}
                                onValueChange={setEffective}
                            />
                        </label>
                    </div>
                )}
                {problem && (
                    <p
                        role="alert"
                        className="rounded-lg border border-destructive/40 bg-destructive/5 p-3 text-sm whitespace-pre-wrap text-destructive"
                    >
                        {problem}
                    </p>
                )}
                <ClientDataTable
                    rows={prices}
                    rowId={(p) => p.id}
                    storageKey="ai-prices"
                    initialSort="pattern"
                    rowTestId="ai-price-row"
                    searchText={(p) =>
                        `${p.model_pattern} ${p.provider_kind} ${p.source}`
                    }
                    searchPlaceholder={t('table.search')}
                    noMatch={
                        <p className="p-8 text-center text-sm text-muted-foreground">
                            {t('table.noMatch')}
                        </p>
                    }
                    columns={[
                        {
                            id: 'pattern',
                            header: t('ai.prices.pattern'),
                            hideable: false,
                            sortValue: (p) => p.model_pattern,
                            cell: (p) => (
                                <div>
                                    <div className="font-mono text-sm">
                                        {p.model_pattern}
                                    </div>
                                    <div className="text-xs text-muted-foreground">
                                        {t(`ai.kinds.${p.provider_kind}`)} ·{' '}
                                        {t(`ai.prices.source.${p.source}`)}
                                    </div>
                                </div>
                            ),
                        },
                        {
                            id: 'rates',
                            header: `${t('ai.prices.input')} / ${t('ai.prices.output')}`,
                            sortValue: (p) => Number(p.input),
                            cell: (p) => priceCell(p),
                        },
                        {
                            id: 'cache',
                            header: t('ai.prices.cachedInput'),
                            cell: (p) => (
                                <span className="font-mono text-xs whitespace-nowrap text-muted-foreground">
                                    {p.cached_input ?? '—'}
                                    {(p.cache_write_5m || p.cache_write_1h) &&
                                        ` · ${p.cache_write_5m ?? '—'} / ${p.cache_write_1h ?? '—'}`}
                                </span>
                            ),
                        },
                        {
                            id: 'rules',
                            header: t('ai.prices.rules'),
                            cell: (p) => {
                                const n =
                                    (p.rules.thresholds?.length ?? 0) +
                                    (p.rules.windows?.length ?? 0);
                                return n ? (
                                    <Badge variant="outline">
                                        {t('ai.prices.rulesCount', {
                                            count: n,
                                        })}
                                    </Badge>
                                ) : (
                                    <span className="text-muted-foreground">
                                        —
                                    </span>
                                );
                            },
                        },
                        {
                            id: 'validity',
                            header: t('ai.prices.effectiveFrom'),
                            sortValue: (p) => p.effective_from,
                            cell: (p) => (
                                <span className="text-sm whitespace-nowrap">
                                    {p.effective_from}
                                    {p.effective_to && ` → ${p.effective_to}`}
                                </span>
                            ),
                        },
                    ]}
                    rowActions={
                        canManage
                            ? (p) => (
                                  <Button
                                      size="sm"
                                      variant="outline"
                                      onClick={() => setEditing({ price: p })}
                                  >
                                      {p.confirmed ? <Pencil /> : <Check />}
                                      {t('ai.models.editPrice')}
                                  </Button>
                              )
                            : undefined
                    }
                    rowMenu={
                        canManage
                            ? (p) => [
                                  [
                                      {
                                          id: 'edit',
                                          label: t('ai.models.editPrice'),
                                          icon: Pencil,
                                          run: () => setEditing({ price: p }),
                                      },
                                  ],
                                  [
                                      {
                                          id: 'delete',
                                          label: t('ai.prices.delete'),
                                          icon: Trash2,
                                          danger: true,
                                          run: () => void removePrice(p),
                                      },
                                  ],
                              ]
                            : undefined
                    }
                />
            </section>
            {editing && (
                <PriceDialog
                    price={editing.price}
                    model={editing.model}
                    fakeAvailable={fakeAvailable}
                    onClose={() => setEditing(null)}
                    onDone={() => {
                        setEditing(null);
                        router.refresh();
                    }}
                />
            )}
        </div>
    );
}
