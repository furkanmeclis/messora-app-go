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
import {
    Table,
    TableBody,
    TableCell,
    TableHead,
    TableHeader,
    TableRow,
} from '@/components/ui/table';
import { useFormatDateTime } from '@/components/display-preferences';
import { useLocale, useT } from '@/i18n/client';
import { intlLocale } from '@/i18n/config';
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
    const locale = useLocale();
    const formatDateTime = useFormatDateTime();
    const names = {
        phones: new Map(phones.map((p) => [p.id, p.label])),
        campaigns: new Map(campaigns.map((c) => [c.id, c.label])),
    };
    if (automations.length === 0) {
        return (
            <Empty className="py-12">
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
        <Table>
            <TableHeader>
                <TableRow>
                    <TableHead>{t('automations.list.columns.name')}</TableHead>
                    <TableHead>
                        {t('automations.list.columns.trigger')}
                    </TableHead>
                    <TableHead>
                        {t('automations.list.columns.active')}
                    </TableHead>
                    <TableHead className="text-right">
                        {t('automations.list.columns.runs')}
                    </TableHead>
                    <TableHead>
                        {t('automations.list.columns.lastRun')}
                    </TableHead>
                    <TableHead>
                        <span className="sr-only">
                            {t('automations.list.columns.actions')}
                        </span>
                    </TableHead>
                </TableRow>
            </TableHeader>
            <TableBody>
                {automations.map((a) => (
                    <TableRow key={a.id} data-testid="automation-row">
                        <TableCell className="max-w-56">
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
                        </TableCell>
                        <TableCell className="max-w-80 text-sm whitespace-normal">
                            {describeTrigger(
                                a.trigger_type,
                                a.trigger,
                                t,
                                names,
                            )}
                        </TableCell>
                        <TableCell>
                            <ActiveSwitch
                                automation={a}
                                disabled={!canManage}
                            />
                        </TableCell>
                        <TableCell className="text-right tabular-nums">
                            {a.run_count.toLocaleString(intlLocale(locale))}
                        </TableCell>
                        <TableCell className="text-sm text-muted-foreground">
                            {a.last_run_at
                                ? formatDateTime(a.last_run_at)
                                : t('automations.list.never')}
                        </TableCell>
                        <TableCell>
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
                                            ? t('automations.list.edit', {
                                                  name: a.name,
                                              })
                                            : t('automations.list.view', {
                                                  name: a.name,
                                              })
                                    }
                                >
                                    <Link href={`/automations/${a.id}`}>
                                        <Pencil />
                                    </Link>
                                </Button>
                                {canManage && <DeleteButton automation={a} />}
                            </div>
                        </TableCell>
                    </TableRow>
                ))}
            </TableBody>
        </Table>
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
