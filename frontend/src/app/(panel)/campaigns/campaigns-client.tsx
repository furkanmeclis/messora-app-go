'use client';

import { useState, useTransition } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { ClientDataTable } from '@/components/client-data-table';
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
import { useFormatDateTime } from '@/components/display-preferences';
import { useT } from '@/i18n/client';
import { TabNav } from '@/components/tab-nav';
import { CAMPAIGN_STATUS_ICON, STATUS_TONE } from '@/lib/status-icons';
import type { Campaign } from '@/lib/types';
import { loadCampaignsAction } from './actions';
import {
    CAMPAIGN_STATUSES,
    CampaignProgress,
    CampaignStatusBadge,
    statusLabel,
    summarize,
} from './campaign-ui';

export function StatusFilter({ status }: { status: string }) {
    const t = useT();
    const items = ['', ...CAMPAIGN_STATUSES];
    return (
        <TabNav
            label={t('campaigns.list.filterByStatus')}
            items={items.map((s) => ({
                href: s ? `/campaigns?status=${s}` : '/campaigns',
                label: s ? statusLabel(t, s) : t('campaigns.list.all'),
                icon: CAMPAIGN_STATUS_ICON[s || 'all'],
                tone: s ? STATUS_TONE[s] : undefined,
                active: s === status,
            }))}
        />
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
    const t = useT();
    const formatDateTime = useFormatDateTime();
    const router = useRouter();
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
            <Empty className="border py-12">
                <EmptyHeader>
                    <EmptyMedia variant="icon">
                        <Megaphone />
                    </EmptyMedia>
                    <EmptyTitle>
                        {status &&
                        CAMPAIGN_STATUSES.includes(status as Campaign['status'])
                            ? t(
                                  `campaigns.list.emptyByStatus.${status as Campaign['status']}`,
                              )
                            : t('campaigns.list.emptyTitle')}
                    </EmptyTitle>
                    <EmptyDescription>
                        {status
                            ? t('campaigns.list.emptyFilteredHint')
                            : t('campaigns.list.emptyHint')}
                    </EmptyDescription>
                </EmptyHeader>
                {canCreate && !status && (
                    <EmptyContent>
                        <Button asChild>
                            <Link href="/campaigns/new">
                                {t('campaigns.list.create')}
                            </Link>
                        </Button>
                    </EmptyContent>
                )}
            </Empty>
        );
    }

    const when = (c: Campaign) =>
        c.status === 'scheduled'
            ? c.scheduled_at
            : (c.finished_at ?? c.started_at ?? c.created_at);

    return (
        <div className="space-y-3">
            <ClientDataTable
                rows={items}
                rowId={(c) => c.id}
                storageKey="campaigns"
                rowTestId="campaign-row"
                searchText={(c) =>
                    `${c.name} ${c.template_name} ${statusLabel(t, c.status)}`
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
                        header: t('campaigns.list.name'),
                        hideable: false,
                        sortValue: (c) => c.name,
                        cell: (c) => (
                            <Link
                                href={`/campaigns/${c.id}`}
                                className="font-medium hover:underline"
                            >
                                {c.name}
                            </Link>
                        ),
                    },
                    {
                        id: 'status',
                        header: t('campaigns.list.status'),
                        sortValue: (c) => statusLabel(t, c.status),
                        cell: (c) => <CampaignStatusBadge status={c.status} />,
                    },
                    {
                        id: 'progress',
                        header: t('campaigns.list.progress'),
                        className: 'w-64',
                        sortValue: (c) => {
                            const s = summarize(c.counts, c.total_recipients);
                            return s.recipients ? s.done / s.recipients : null;
                        },
                        cell: (c) =>
                            c.status === 'draft' || c.status === 'scheduled' ? (
                                <span className="text-xs text-muted-foreground">
                                    {t('campaigns.list.notStarted')}
                                </span>
                            ) : (
                                <CampaignProgress
                                    compact
                                    counts={c.counts}
                                    total={c.total_recipients}
                                    label={t('campaigns.list.progressOf', {
                                        name: c.name,
                                    })}
                                />
                            ),
                    },
                    {
                        id: 'template',
                        header: t('campaigns.list.template'),
                        sortValue: (c) => c.template_name,
                        cell: (c) => (
                            <span className="font-mono text-xs">
                                {c.template_name}
                                <span className="text-muted-foreground">
                                    {' '}
                                    ({c.template_language})
                                </span>
                            </span>
                        ),
                    },
                    {
                        id: 'when',
                        header: t('campaigns.list.when'),
                        sortValue: when,
                        cell: (c) => (
                            <span className="text-sm whitespace-nowrap text-muted-foreground">
                                {c.status === 'scheduled'
                                    ? t('campaigns.list.starts', {
                                          date: formatDateTime(c.scheduled_at),
                                      })
                                    : c.finished_at
                                      ? t('campaigns.list.finished', {
                                            date: formatDateTime(c.finished_at),
                                        })
                                      : c.started_at
                                        ? t('campaigns.list.started', {
                                              date: formatDateTime(
                                                  c.started_at,
                                              ),
                                          })
                                        : t('campaigns.list.created', {
                                              date: formatDateTime(
                                                  c.created_at,
                                              ),
                                          })}
                            </span>
                        ),
                    },
                ]}
                rowMenu={(c) => [
                    [
                        {
                            id: 'open',
                            label: c.name,
                            icon: Megaphone,
                            run: () => router.push(`/campaigns/${c.id}`),
                        },
                    ],
                ]}
                onRowClick={(c) => router.push(`/campaigns/${c.id}`)}
            />
            {cursor && (
                <div className="text-center">
                    <Button variant="outline" onClick={more} disabled={pending}>
                        {pending && <Spinner />} {t('campaigns.list.loadMore')}
                    </Button>
                </div>
            )}
        </div>
    );
}
