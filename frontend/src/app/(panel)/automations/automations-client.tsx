'use client';

import { useState, useTransition } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { History, Pencil, Trash2, Workflow } from 'lucide-react';
import { useConfirm } from '@/components/confirm';
import { notify } from '@/components/notify';
import { Button } from '@/components/ui/button';
import {
    Empty,
    EmptyContent,
    EmptyDescription,
    EmptyHeader,
    EmptyMedia,
    EmptyTitle,
} from '@/components/ui/empty';
import { Switch } from '@/components/ui/switch';
import { useFormatDateTime } from '@/components/display-preferences';
import { useT } from '@/i18n/client';
import { NUMBER_LOCALE } from '@/lib/format';
import { ClientDataTable } from '@/components/client-data-table';
import type { Automation } from '@/lib/types';
import { deleteAutomationAction, setActiveAction } from './actions';
import { describeTrigger } from './model';

type Option = { id: string; label: string };

/** "Loading…" for screen readers while a screen loads. */
export function LoadingLabel() {
    const t = useT();
    return <span className="sr-only">{t('common.loading')}</span>;
}

export function AutomationTable({
    automations,
    phones,
    campaigns,
    canManage,
}: {
    automations: Automation[];
    phones: Option[];
    campaigns: Option[];
    canManage: boolean;
}) {
    const t = useT();
    const router = useRouter();
    const formatDateTime = useFormatDateTime();
    const names = {
        phones: new Map(phones.map((p) => [p.id, p.label])),
        campaigns: new Map(campaigns.map((c) => [c.id, c.label])),
    };
    if (automations.length === 0) {
        return (
            <Empty className="border py-12">
                <EmptyHeader>
                    <EmptyMedia variant="icon">
                        <Workflow />
                    </EmptyMedia>
                    <EmptyTitle>{t('automations.list.emptyTitle')}</EmptyTitle>
                    <EmptyDescription>
                        {t('automations.list.emptyDescription')}
                    </EmptyDescription>
                </EmptyHeader>
                {canManage && (
                    <EmptyContent>
                        <Button asChild>
                            <Link href="/automations/new">
                                {t('automations.list.createFirst')}
                            </Link>
                        </Button>
                    </EmptyContent>
                )}
            </Empty>
        );
    }
    return (
        <ClientDataTable
            rows={automations}
            rowId={(a) => a.id}
            storageKey="automations"
            rowTestId="automation-row"
            initialSort="name"
            searchText={(a) =>
                `${a.name} ${a.description ?? ''} ${describeTrigger(
                    a.trigger_type,
                    a.trigger,
                    t,
                    names,
                )}`
            }
            searchPlaceholder={t('table.search')}
            noMatch={
                <p className="p-8 text-center text-sm text-muted-foreground">
                    {t('table.noMatch')}
                </p>
            }
            columns={[
                {
                    id: 'name',
                    header: t('automations.list.columns.name'),
                    hideable: false,
                    sortValue: (a) => a.name,
                    cell: (a) => (
                        <div className="max-w-56 min-w-0">
                            <Link
                                href={`/automations/${a.id}`}
                                className="font-medium hover:underline"
                            >
                                {a.name}
                            </Link>
                            {a.description && (
                                <div className="truncate text-xs text-muted-foreground">
                                    {a.description}
                                </div>
                            )}
                        </div>
                    ),
                },
                {
                    id: 'trigger',
                    header: t('automations.list.columns.trigger'),
                    sortValue: (a) =>
                        describeTrigger(a.trigger_type, a.trigger, t, names),
                    cell: (a) => (
                        <div className="max-w-80 text-sm whitespace-normal">
                            {describeTrigger(
                                a.trigger_type,
                                a.trigger,
                                t,
                                names,
                            )}
                        </div>
                    ),
                },
                {
                    id: 'active',
                    header: t('automations.list.columns.active'),
                    sortValue: (a) => (a.active ? 1 : 0),
                    cell: (a) => (
                        <ActiveSwitch automation={a} disabled={!canManage} />
                    ),
                },
                {
                    id: 'runs',
                    header: t('automations.list.columns.runs'),
                    className: 'text-right',
                    sortValue: (a) => a.run_count,
                    cell: (a) => (
                        <span className="tabular-nums">
                            {a.run_count.toLocaleString(NUMBER_LOCALE)}
                        </span>
                    ),
                },
                {
                    id: 'lastRun',
                    header: t('automations.list.columns.lastRun'),
                    sortValue: (a) => a.last_run_at ?? null,
                    cell: (a) => (
                        <span className="text-sm whitespace-nowrap text-muted-foreground">
                            {a.last_run_at
                                ? formatDateTime(a.last_run_at)
                                : t('automations.list.never')}
                        </span>
                    ),
                },
            ]}
            rowActions={(a) => (
                <div className="flex justify-end gap-1">
                    <Button
                        asChild
                        size="icon-sm"
                        variant="ghost"
                        aria-label={t('automations.list.runsOf', {
                            name: a.name,
                        })}
                    >
                        <Link href={`/automations/${a.id}/runs`}>
                            <History />
                        </Link>
                    </Button>
                    <Button
                        asChild
                        size="icon-sm"
                        variant="ghost"
                        aria-label={
                            canManage
                                ? t('automations.list.edit', { name: a.name })
                                : t('automations.list.view', { name: a.name })
                        }
                    >
                        <Link href={`/automations/${a.id}`}>
                            <Pencil />
                        </Link>
                    </Button>
                    {canManage && <DeleteButton automation={a} />}
                </div>
            )}
            rowMenu={(a) => [
                [
                    {
                        id: 'open',
                        label: canManage
                            ? t('automations.list.edit', { name: a.name })
                            : t('automations.list.view', { name: a.name }),
                        icon: Pencil,
                        run: () => router.push(`/automations/${a.id}`),
                    },
                    {
                        id: 'runs',
                        label: t('automations.list.runsOf', { name: a.name }),
                        icon: History,
                        run: () => router.push(`/automations/${a.id}/runs`),
                    },
                ],
            ]}
            onRowClick={(a) => router.push(`/automations/${a.id}`)}
        />
    );
}

export function ActiveSwitch({
    automation,
    disabled,
}: {
    automation: Pick<Automation, 'id' | 'name' | 'active'>;
    disabled?: boolean;
}) {
    const t = useT();
    const [pending, startTransition] = useTransition();
    const [checked, setChecked] = useState(automation.active);
    const [seen, setSeen] = useState(automation.active);
    // Follow the server value after a refresh.
    if (seen !== automation.active) {
        setSeen(automation.active);
        setChecked(automation.active);
    }
    return (
        <Switch
            checked={checked}
            disabled={disabled || pending}
            aria-label={t('automations.list.activeLabel', {
                name: automation.name,
            })}
            onCheckedChange={(v) => {
                setChecked(v);
                startTransition(async () => {
                    if (!notify(await setActiveAction(automation.id, v))) {
                        setChecked(!v);
                    }
                });
            }}
        />
    );
}

export function DeleteButton({
    automation,
    redirectTo,
    withLabel,
}: {
    automation: Pick<Automation, 'id' | 'name'>;
    redirectTo?: string;
    withLabel?: boolean;
}) {
    const t = useT();
    const ask = useConfirm();
    const router = useRouter();
    const [pending, startTransition] = useTransition();
    const onClick = async () => {
        const ok = await ask({
            title: t('automations.delete.title', { name: automation.name }),
            description: t('automations.delete.description'),
            confirmLabel: t('common.delete'),
            destructive: true,
        });
        if (!ok) return;
        startTransition(async () => {
            const done = notify(await deleteAutomationAction(automation.id));
            if (done && redirectTo) router.push(redirectTo);
        });
    };
    return withLabel ? (
        <Button
            type="button"
            variant="outline"
            disabled={pending}
            onClick={onClick}
        >
            <Trash2 /> {t('common.delete')}
        </Button>
    ) : (
        <Button
            type="button"
            size="icon-sm"
            variant="ghost"
            disabled={pending}
            aria-label={t('automations.delete.label', {
                name: automation.name,
            })}
            onClick={onClick}
        >
            <Trash2 />
        </Button>
    );
}
