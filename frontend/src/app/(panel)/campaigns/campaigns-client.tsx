'use client';

import { useState, useTransition } from 'react';
import Link from 'next/link';
import { Megaphone } from 'lucide-react';
import { notify } from '@/components/notify';
import { useRealtimeEvents } from '@/components/realtime';
import { Button } from '@/components/ui/button';
import {
    Empty,
    EmptyContent,
    EmptyDescription,
    EmptyHeader,
    EmptyMedia,
    EmptyTitle,
} from '@/components/ui/empty';
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
import type { Campaign } from '@/lib/types';
import { loadCampaignsAction } from './actions';
import {
    CAMPAIGN_STATUSES,
    CampaignProgress,
    CampaignStatusBadge,
    statusLabel,
} from './campaign-ui';

export function StatusFilter({ status }: { status: string }) {
    const items = ['', ...CAMPAIGN_STATUSES];
    return (
        <nav aria-label="Filter by status">
            <ul className="flex flex-wrap gap-1">
                {items.map((s) => (
                    <li key={s || 'all'}>
                        <Button
                            asChild
                            size="sm"
                            variant={s === status ? 'secondary' : 'ghost'}
                        >
                            <Link
                                href={
                                    s ? `/campaigns?status=${s}` : '/campaigns'
                                }
                                aria-current={s === status ? 'page' : undefined}
                            >
                                {s ? statusLabel(s) : 'All'}
                            </Link>
                        </Button>
                    </li>
                ))}
            </ul>
        </nav>
    );
}

/** Patches a campaign row from a `campaign.updated` event. */
export function applyEvent<
    T extends Pick<Campaign, 'id' | 'status' | 'counts'>,
>(c: T, e: Record<string, unknown>): T {
    if (e.campaign_id !== c.id) return c;
    const next = { ...c };
    if (typeof e.status === 'string') {
        next.status = e.status as Campaign['status'];
    }
    if (e.counts && typeof e.counts === 'object') {
        next.counts = e.counts as Record<string, number>;
    }
    return next;
}

export function CampaignList({
    initial,
    initialCursor,
    status,
    canCreate,
}: {
    initial: Campaign[];
    initialCursor?: string;
    status: string;
    canCreate: boolean;
}) {
    const formatDateTime = useFormatDateTime();
    const [items, setItems] = useState(initial);
    const [cursor, setCursor] = useState(initialCursor);
    const [pending, startTransition] = useTransition();

    // Live progress without polling: events carry status and counts.
    useRealtimeEvents((channel, e) => {
        if (channel !== 'campaign:all' || e.type !== 'campaign.updated') return;
        setItems((list) => list.map((c) => applyEvent(c, e)));
    });

    const more = () =>
        startTransition(async () => {
            if (!cursor) return;
            const r = await loadCampaignsAction(status, cursor);
            if (notify(r)) {
                setItems((l) => [
                    ...l,
                    ...r.data.items.filter(
                        (x) => !l.some((y) => y.id === x.id),
                    ),
                ]);
                setCursor(r.data.next);
            }
        });

    if (items.length === 0) {
        return (
            <Empty className="py-12">
                <EmptyHeader>
                    <EmptyMedia variant="icon">
                        <Megaphone />
                    </EmptyMedia>
                    <EmptyTitle>
                        {status
                            ? `No ${statusLabel(status).toLowerCase()} campaigns`
                            : 'No campaigns yet'}
                    </EmptyTitle>
                    <EmptyDescription>
                        {status
                            ? 'Campaigns in other states are under “All”.'
                            : 'A campaign sends an approved template to a segment, group or tag of your contacts.'}
                    </EmptyDescription>
                </EmptyHeader>
                {canCreate && !status && (
                    <EmptyContent>
                        <Button asChild>
                            <Link href="/campaigns/new">Create a campaign</Link>
                        </Button>
                    </EmptyContent>
                )}
            </Empty>
        );
    }

    return (
        <div>
            <Table>
                <TableHeader>
                    <TableRow>
                        <TableHead>Name</TableHead>
                        <TableHead>Status</TableHead>
                        <TableHead className="w-64">Progress</TableHead>
                        <TableHead>Template</TableHead>
                        <TableHead>When</TableHead>
                    </TableRow>
                </TableHeader>
                <TableBody>
                    {items.map((c) => (
                        <TableRow key={c.id} data-testid="campaign-row">
                            <TableCell className="font-medium">
                                <Link
                                    href={`/campaigns/${c.id}`}
                                    className="hover:underline"
                                >
                                    {c.name}
                                </Link>
                            </TableCell>
                            <TableCell>
                                <CampaignStatusBadge status={c.status} />
                            </TableCell>
                            <TableCell>
                                {c.status === 'draft' ||
                                c.status === 'scheduled' ? (
                                    <span className="text-xs text-muted-foreground">
                                        Not started
                                    </span>
                                ) : (
                                    <CampaignProgress
                                        compact
                                        counts={c.counts}
                                        total={c.total_recipients}
                                        label={`Progress of ${c.name}`}
                                    />
                                )}
                            </TableCell>
                            <TableCell className="font-mono text-xs">
                                {c.template_name}
                                <span className="text-muted-foreground">
                                    {' '}
                                    ({c.template_language})
                                </span>
                            </TableCell>
                            <TableCell className="text-sm text-muted-foreground">
                                {c.status === 'scheduled'
                                    ? `Starts ${formatDateTime(c.scheduled_at)}`
                                    : c.finished_at
                                      ? `Finished ${formatDateTime(c.finished_at)}`
                                      : c.started_at
                                        ? `Started ${formatDateTime(c.started_at)}`
                                        : `Created ${formatDateTime(c.created_at)}`}
                            </TableCell>
                        </TableRow>
                    ))}
                </TableBody>
            </Table>
            {cursor && (
                <div className="border-t p-3 text-center">
                    <Button variant="outline" onClick={more} disabled={pending}>
                        {pending && <Spinner />} Load more
                    </Button>
                </div>
            )}
        </div>
    );
}
