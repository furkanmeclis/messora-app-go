'use client';

import { useState, useTransition } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { History, Pencil, Trash2, Workflow } from 'lucide-react';
import { notify } from '@/components/notify';
import {
    AlertDialog,
    AlertDialogAction,
    AlertDialogCancel,
    AlertDialogContent,
    AlertDialogDescription,
    AlertDialogFooter,
    AlertDialogHeader,
    AlertDialogTitle,
    AlertDialogTrigger,
} from '@/components/ui/alert-dialog';
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
import type { Automation } from '@/lib/types';
import { deleteAutomationAction, setActiveAction } from './actions';
import { describeTrigger } from './model';
import { NUMBER_LOCALE } from '@/lib/format';

type Option = { id: string; label: string };

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
                    <EmptyTitle>No automations yet</EmptyTitle>
                    <EmptyDescription>
                        Answer keywords, tag contacts or follow up after a delay
                        without lifting a finger.
                    </EmptyDescription>
                </EmptyHeader>
                {canManage && (
                    <EmptyContent>
                        <Button asChild>
                            <Link href="/automations/new">
                                Create your first automation
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
                    <TableHead>Name</TableHead>
                    <TableHead>Trigger</TableHead>
                    <TableHead>Active</TableHead>
                    <TableHead className="text-right">Runs</TableHead>
                    <TableHead>Last run</TableHead>
                    <TableHead>
                        <span className="sr-only">Actions</span>
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
                            {describeTrigger(a.trigger_type, a.trigger, names)}
                        </TableCell>
                        <TableCell>
                            <ActiveSwitch
                                automation={a}
                                disabled={!canManage}
                            />
                        </TableCell>
                        <TableCell className="text-right tabular-nums">
                            {a.run_count.toLocaleString(NUMBER_LOCALE)}
                        </TableCell>
                        <TableCell className="text-sm text-muted-foreground">
                            {a.last_run_at
                                ? formatDateTime(a.last_run_at)
                                : 'Never'}
                        </TableCell>
                        <TableCell>
                            <div className="flex justify-end gap-1">
                                <Button
                                    asChild
                                    size="icon-sm"
                                    variant="ghost"
                                    aria-label={`Runs of ${a.name}`}
                                >
                                    <Link href={`/automations/${a.id}/runs`}>
                                        <History />
                                    </Link>
                                </Button>
                                <Button
                                    asChild
                                    size="icon-sm"
                                    variant="ghost"
                                    aria-label={`${canManage ? 'Edit' : 'View'} ${a.name}`}
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
            aria-label={`Active: ${automation.name}`}
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
    const router = useRouter();
    const [pending, startTransition] = useTransition();
    return (
        <AlertDialog>
            <AlertDialogTrigger asChild>
                {withLabel ? (
                    <Button variant="outline" disabled={pending}>
                        <Trash2 /> Delete
                    </Button>
                ) : (
                    <Button
                        size="icon-sm"
                        variant="ghost"
                        disabled={pending}
                        aria-label={`Delete ${automation.name}`}
                    >
                        <Trash2 />
                    </Button>
                )}
            </AlertDialogTrigger>
            <AlertDialogContent>
                <AlertDialogHeader>
                    <AlertDialogTitle>
                        Delete “{automation.name}”?
                    </AlertDialogTitle>
                    <AlertDialogDescription>
                        Runs in progress stop and the run history is deleted
                        too. This cannot be undone.
                    </AlertDialogDescription>
                </AlertDialogHeader>
                <AlertDialogFooter>
                    <AlertDialogCancel>Cancel</AlertDialogCancel>
                    <AlertDialogAction
                        variant="destructive"
                        onClick={() =>
                            startTransition(async () => {
                                const ok = notify(
                                    await deleteAutomationAction(automation.id),
                                );
                                if (ok && redirectTo) router.push(redirectTo);
                            })
                        }
                    >
                        Delete
                    </AlertDialogAction>
                </AlertDialogFooter>
            </AlertDialogContent>
        </AlertDialog>
    );
}
