'use client';

import { useEffect, useMemo, useRef, useState, useTransition } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import {
    ChevronDown,
    Download,
    Filter,
    Megaphone,
    RefreshCw,
    Search,
    Tag as TagIcon,
    Users,
} from 'lucide-react';
import { toast } from 'sonner';
import {
    DataTable,
    type Column,
    type Selection,
} from '@/components/data-table';
import { notify } from '@/components/notify';
import { useReauth } from '@/components/reauth';
import { useFormatDateTime } from '@/components/display-preferences';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
    DropdownMenu,
    DropdownMenuCheckboxItem,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuLabel,
    DropdownMenuSeparator,
    DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { Input } from '@/components/ui/input';
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from '@/components/ui/select';
import {
    Tooltip,
    TooltipContent,
    TooltipTrigger,
} from '@/components/ui/tooltip';
import { useT } from '@/i18n/client';
import type { MessageKey } from '@/i18n/translate';
import { NUMBER_LOCALE } from '@/lib/format';
import type { Campaign, Schemas } from '@/lib/types';
import {
    bulkRecipientsAction,
    createExportAction,
    loadAudienceRefsAction,
    searchRecipientsAction,
    setReactionAction,
    type RecipientFilter,
    type RecipientRow,
} from '../analytics-actions';
import {
    failureClassLabel,
    skipReasonLabel,
    unreachedLabel,
} from '../campaign-ui';

const PAGE = 50;
const DEFAULT_SORT = '-created';
/** How often the shown page is re-read at most while the counts move. */
const LIVE_REFRESH_MS = 3000;
const SYNC_EXPORT_LIMIT = 20000;
const ANY = '__any';

export const STATUS_FILTERS = [
    'sent',
    'delivered',
    'read',
    'failed',
    'pending',
    'replied',
    'positive',
    'negative',
    'neutral',
    'opted_out',
    'rejected',
    'blocked',
    'unknown',
    'cancelled',
    'temporary',
    'ecosystem_limit',
    'undeliverable',
    'permanent',
    'never_sent',
    'user_stopped_marketing',
    'blocked_by_consent',
] as const;

type Refs = {
    tags: Schemas['Tag'][];
    groups: Schemas['Group'][];
    segments: Schemas['Segment'][];
};

function exportQuery(f: RecipientFilter, format: 'csv' | 'xlsx') {
    const q = new URLSearchParams({ format });
    if (f.q) q.set('q', f.q);
    if (f.status?.length) q.set('status', f.status.join(','));
    if (f.tag_id) q.set('tag_id', f.tag_id);
    if (f.group_id) q.set('group_id', f.group_id);
    if (f.segment_id) q.set('segment_id', f.segment_id);
    if (f.contact_type !== undefined) q.set('contact_type', f.contact_type);
    if (f.attr_key) {
        q.set('attr_key', f.attr_key);
        q.set('attr_value', f.attr_value ?? '');
    }
    return q.toString();
}

/** A table selection as a recipient filter (bulk actions, exports). */
function selectionFilter(f: RecipientFilter, s: Selection): RecipientFilter {
    if (s.mode === 'ids') return { ...f, ids: s.ids };
    return s.exclude.length ? { ...f, exclude_ids: s.exclude } : f;
}

/**
 * The campaign's recipients: server-side search, filters, sort and pages,
 * bulk actions on a selection, exports. Refreshes itself while the
 * campaign's counters move (realtime progress events).
 */
export function RecipientsPanel({
    campaign,
    canEdit,
}: {
    campaign: Campaign;
    canEdit: boolean;
}) {
    const t = useT();
    const router = useRouter();
    const formatDateTime = useFormatDateTime();
    const { guard } = useReauth();
    const [q, setQ] = useState('');
    const [statuses, setStatuses] = useState<string[]>([]);
    const [adv, setAdv] = useState<RecipientFilter>({});
    const [showAdv, setShowAdv] = useState(false);
    const [sort, setSort] = useState(DEFAULT_SORT);
    const [page, setPage] = useState(1);
    const [pageSize, setPageSize] = useState(PAGE);
    const [data, setData] = useState<{
        rows: RecipientRow[];
        total: number;
    } | null>(null);
    const [loading, setLoading] = useState(false);
    const [tick, setTick] = useState(0);
    const [refs, setRefs] = useState<Refs | null>(null);
    const [pending, startTransition] = useTransition();

    const filter = useMemo<RecipientFilter>(() => {
        const f: RecipientFilter = { ...adv };
        if (q.trim()) f.q = q.trim().slice(0, 100);
        if (statuses.length) f.status = statuses;
        return f;
    }, [adv, q, statuses]);
    const filterKey = JSON.stringify(filter);
    // Another filter, order or page size starts at the first page.
    const viewKey = `${filterKey}|${sort}|${pageSize}`;
    const [view, setView] = useState(viewKey);
    if (view !== viewKey) {
        setView(viewKey);
        setPage(1);
    }

    const request = useRef(0);
    useEffect(() => {
        const id = ++request.current;
        const timer = setTimeout(async () => {
            setLoading(true);
            const r = await searchRecipientsAction(campaign.id, {
                filter,
                sort,
                offset: (page - 1) * pageSize,
                limit: pageSize,
            });
            if (id !== request.current) return; // a newer request runs
            setLoading(false);
            if (notify(r)) setData(r.data);
        }, 250);
        return () => clearTimeout(timer);
        // filterKey stands for filter.
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [campaign.id, filterKey, sort, page, pageSize, tick]);

    // Live: progress events change the counts; re-read the shown page at
    // most every few seconds while they do.
    const countsKey = JSON.stringify(campaign.counts ?? {});
    const seenCounts = useRef(countsKey);
    const liveTimer = useRef<ReturnType<typeof setTimeout> | undefined>(
        undefined,
    );
    useEffect(() => {
        if (seenCounts.current === countsKey) return;
        seenCounts.current = countsKey;
        if (liveTimer.current) return;
        liveTimer.current = setTimeout(() => {
            liveTimer.current = undefined;
            setTick((n) => n + 1);
        }, LIVE_REFRESH_MS);
    }, [countsKey]);
    useEffect(() => () => clearTimeout(liveTimer.current), []);

    useEffect(() => {
        if (!showAdv && !canEdit) return;
        if (refs) return;
        void loadAudienceRefsAction().then((r) => r.ok && setRefs(r.data));
    }, [showAdv, canEdit, refs]);

    const total = data?.total ?? 0;

    const bulk = (
        action: 'add_tag' | 'add_to_group' | 'follow_up',
        selection: Selection,
        clear: () => void,
        target?: string,
    ) =>
        startTransition(async () => {
            const r = await guard(() =>
                bulkRecipientsAction(campaign.id, {
                    action,
                    target_id: target,
                    filter: selectionFilter(filter, selection),
                }),
            );
            if (!notify(r)) return;
            clear();
            if (action === 'follow_up' && r.data.campaign) {
                toast.success(
                    t('analytics.recipients.followUpCreated', {
                        count: r.data.affected,
                    }),
                );
                router.push(`/campaigns/${r.data.campaign.id}/edit`);
                return;
            }
            toast.success(
                t('analytics.recipients.bulkDone', { count: r.data.affected }),
            );
        });

    /** Large sets and selections are prepared in the background. */
    const exportAs = (format: 'csv' | 'xlsx', selection?: Selection) => {
        const f = selection ? selectionFilter(filter, selection) : filter;
        if (f !== filter || total > SYNC_EXPORT_LIMIT) {
            startTransition(async () => {
                const r = await guard(() =>
                    createExportAction(campaign.id, {
                        kind: 'recipients',
                        format,
                        filter: f,
                    }),
                );
                if (notify(r))
                    toast.success(t('analytics.recipients.exportQueued'));
            });
            return;
        }
        const a = document.createElement('a');
        a.href = `/bff/campaigns/${campaign.id}/recipients/export?${exportQuery(filter, format)}`;
        a.click();
    };

    const statusLabel = (s: string) =>
        t(`analytics.recipients.statuses.${s}` as MessageKey);
    const sortable = (key: string) => ({ asc: key, desc: `-${key}` });
    const when = (iso: string | null | undefined) => (
        <span className="text-xs whitespace-nowrap text-muted-foreground">
            {iso ? formatDateTime(iso) : '—'}
        </span>
    );
    const filtered = statuses.length > 0 || q || Object.keys(adv).length > 0;

    const columns: Column<RecipientRow>[] = [
        {
            id: 'phone',
            header: t('analytics.recipients.columns.phone'),
            hideable: false,
            sort: sortable('phone'),
            cell: (r) =>
                r.contact_id ? (
                    <Link
                        href={`/contacts/${r.contact_id}`}
                        className="font-mono text-sm underline-offset-4 hover:underline"
                    >
                        {r.phone}
                    </Link>
                ) : (
                    <span className="font-mono text-sm">{r.phone}</span>
                ),
        },
        {
            id: 'name',
            header: t('analytics.recipients.columns.name'),
            sort: sortable('name'),
            cell: (r) => (
                <span className="text-sm">
                    {r.name || '—'}
                    {r.contact_type && (
                        <span className="ml-1 text-xs text-muted-foreground">
                            · {r.contact_type}
                        </span>
                    )}
                </span>
            ),
        },
        {
            id: 'status',
            header: t('analytics.recipients.columns.status'),
            cell: (r) => <RecipientStatus r={r} />,
        },
        {
            id: 'reason',
            header: t('analytics.recipients.columns.reason'),
            cell: (r) => <RecipientReason r={r} />,
        },
        {
            id: 'sent',
            header: t('analytics.recipients.columns.sent'),
            sort: sortable('sent_at'),
            cell: (r) => when(r.sent_at),
        },
        {
            id: 'delivered',
            header: t('analytics.recipients.columns.delivered'),
            sort: sortable('delivered_at'),
            cell: (r) => when(r.delivered_at),
        },
        {
            id: 'read',
            header: t('analytics.recipients.columns.read'),
            sort: sortable('read_at'),
            cell: (r) => when(r.read_at),
        },
        {
            id: 'replied',
            header: t('analytics.recipients.columns.replied'),
            sort: sortable('replied_at'),
            cell: (r) => (
                <span className="inline-flex items-center gap-1">
                    {when(r.replied_at)}
                    {r.opted_out_at && (
                        <Badge variant="destructive">
                            {statusLabel('opted_out')}
                        </Badge>
                    )}
                </span>
            ),
        },
        {
            id: 'reaction',
            header: t('analytics.recipients.columns.reaction'),
            cell: (r) => (
                <ReactionCell
                    campaignId={campaign.id}
                    row={r}
                    canEdit={canEdit}
                />
            ),
        },
        {
            id: 'cost',
            header: t('analytics.recipients.columns.cost'),
            className: 'text-right',
            sort: sortable('cost'),
            cell: (r) => (
                <span className="text-xs whitespace-nowrap tabular-nums">
                    {r.cost
                        ? `${Number(r.cost).toLocaleString(NUMBER_LOCALE, { maximumFractionDigits: 4 })} ${r.cost_currency ?? ''}`
                        : '—'}
                </span>
            ),
        },
    ];

    return (
        <div className="space-y-3" data-testid="recipients-panel">
            {showAdv && (
                <AdvancedFilters value={adv} onChange={setAdv} refs={refs} />
            )}
            <DataTable
                rows={data?.rows ?? []}
                columns={columns}
                rowId={(r) => r.id}
                total={total}
                page={page}
                pageSize={pageSize}
                sort={sort}
                storageKey="campaign-recipients"
                rowTestId="recipient-row"
                selectionKey={`${campaign.id}|${filterKey}`}
                loading={loading || pending}
                onNavigate={(patch) => {
                    if ('sort' in patch)
                        setSort(String(patch.sort || DEFAULT_SORT));
                    if ('limit' in patch && patch.limit)
                        setPageSize(Number(patch.limit));
                    if ('page' in patch)
                        setPage(patch.page ? Number(patch.page) : 1);
                }}
                onRowClick={(r) =>
                    r.contact_id && router.push(`/contacts/${r.contact_id}`)
                }
                empty={
                    <p className="p-8 text-center text-sm text-muted-foreground">
                        {data
                            ? filtered
                                ? t('analytics.recipients.empty')
                                : t('analytics.recipients.none')
                            : t('table.loading')}
                    </p>
                }
                bulkActions={(selection, clear) => (
                    <>
                        {canEdit && (
                            <>
                                <TargetMenu
                                    icon={<TagIcon />}
                                    label={t('analytics.recipients.addTag')}
                                    items={refs?.tags ?? []}
                                    onPick={(id) =>
                                        bulk('add_tag', selection, clear, id)
                                    }
                                />
                                <TargetMenu
                                    icon={<Users />}
                                    label={t('analytics.recipients.addToGroup')}
                                    items={refs?.groups ?? []}
                                    onPick={(id) =>
                                        bulk(
                                            'add_to_group',
                                            selection,
                                            clear,
                                            id,
                                        )
                                    }
                                />
                                <Button
                                    size="sm"
                                    variant="outline"
                                    disabled={pending}
                                    onClick={() =>
                                        bulk('follow_up', selection, clear)
                                    }
                                    data-testid="follow-up"
                                >
                                    <Megaphone />
                                    {t('analytics.recipients.followUp')}
                                </Button>
                            </>
                        )}
                        <ExportMenu
                            onPick={(format) => exportAs(format, selection)}
                        />
                    </>
                )}
                toolbar={
                    <>
                        <div className="relative w-full sm:w-64">
                            <Search className="pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2 text-muted-foreground" />
                            <Input
                                type="search"
                                value={q}
                                onChange={(e) => setQ(e.target.value)}
                                placeholder={t('analytics.recipients.search')}
                                aria-label={t('analytics.recipients.search')}
                                className="pl-8"
                                maxLength={100}
                                data-testid="recipients-search"
                            />
                        </div>
                        <DropdownMenu>
                            <DropdownMenuTrigger asChild>
                                <Button
                                    variant="outline"
                                    size="sm"
                                    data-testid="recipients-status"
                                >
                                    {t('analytics.recipients.status')}
                                    {statuses.length > 0 && (
                                        <Badge variant="secondary">
                                            {statuses.length}
                                        </Badge>
                                    )}
                                    <ChevronDown />
                                </Button>
                            </DropdownMenuTrigger>
                            <DropdownMenuContent className="max-h-80 overflow-y-auto">
                                {STATUS_FILTERS.map((s) => (
                                    <DropdownMenuCheckboxItem
                                        key={s}
                                        checked={statuses.includes(s)}
                                        data-status={s}
                                        onSelect={(e) => e.preventDefault()}
                                        onCheckedChange={(on) =>
                                            setStatuses((cur) =>
                                                on
                                                    ? [...cur, s]
                                                    : cur.filter(
                                                          (x) => x !== s,
                                                      ),
                                            )
                                        }
                                    >
                                        {statusLabel(s)}
                                    </DropdownMenuCheckboxItem>
                                ))}
                            </DropdownMenuContent>
                        </DropdownMenu>
                        <Button
                            variant={showAdv ? 'secondary' : 'outline'}
                            size="sm"
                            aria-pressed={showAdv}
                            onClick={() => setShowAdv((v) => !v)}
                        >
                            <Filter /> {t('analytics.recipients.advanced')}
                        </Button>
                        {filtered && (
                            <Button
                                variant="ghost"
                                size="sm"
                                onClick={() => {
                                    setQ('');
                                    setStatuses([]);
                                    setAdv({});
                                }}
                            >
                                {t('analytics.recipients.clear')}
                            </Button>
                        )}
                        <Button
                            variant="ghost"
                            size="sm"
                            disabled={loading}
                            onClick={() => setTick((n) => n + 1)}
                        >
                            <RefreshCw /> {t('analytics.recipients.refresh')}
                        </Button>
                        <ExportMenu
                            testId="recipients-export"
                            onPick={(format) => exportAs(format)}
                        />
                    </>
                }
            />
        </div>
    );
}

const MESSAGE_STATUSES = [
    'pending',
    'sending',
    'sent',
    'delivered',
    'read',
    'failed',
    'unknown',
    'held',
    'cancelled',
    'deferred',
    'skipped',
] as const;

/**
 * The one status of a recipient: why it was skipped, waiting, a scheduled
 * new attempt, or its message's status, with resends and the next try.
 */
function RecipientStatus({ r }: { r: RecipientRow }) {
    const t = useT();
    const formatDateTime = useFormatDateTime();
    const skipReason =
        r.state === 'skipped'
            ? r.skip_reason
            : r.message_status === 'skipped'
              ? r.error_code // consent stopped it right before sending
              : undefined;
    if (skipReason !== undefined) {
        return (
            <Badge variant="outline" data-testid="recipient-skip-reason">
                {skipReason
                    ? t('campaigns.dashboard.skippedReason', {
                          reason: skipReasonLabel(t, skipReason),
                      })
                    : t('campaigns.dashboard.skipped')}
            </Badge>
        );
    }
    if (r.state === 'cancelled')
        return (
            <Badge variant="outline">
                {t('campaigns.dashboard.cancelled')}
            </Badge>
        );
    if (r.state === 'pending')
        return (
            <Badge variant="outline">{t('campaigns.dashboard.waiting')}</Badge>
        );
    const st: string =
        r.state === 'retry' ? 'deferred' : (r.message_status ?? 'pending');
    const known = MESSAGE_STATUSES.find((x) => x === st);
    const label = known
        ? t(`campaigns.dashboard.messageStatuses.${known}`)
        : st;
    const variant =
        st === 'failed'
            ? 'destructive'
            : st === 'read' || st === 'delivered'
              ? 'secondary'
              : 'outline';
    const badge = (
        <Badge variant={variant} data-status={st}>
            {label}
            {r.resends > 0 && (
                <span
                    className="tabular-nums opacity-70"
                    title={t('analytics.recipients.resends', {
                        count: r.resends,
                    })}
                >
                    +{r.resends}
                </span>
            )}
        </Badge>
    );
    const next = r.next_attempt_at && (
        <div className="mt-1 text-xs whitespace-nowrap text-muted-foreground">
            {t('analytics.recipients.nextAttempt', {
                time: formatDateTime(r.next_attempt_at),
            })}
        </div>
    );
    if (st !== 'unknown')
        return (
            <>
                {badge}
                {next}
            </>
        );
    const help = t('campaigns.dashboard.unknownHelp');
    return (
        <Tooltip>
            <TooltipTrigger asChild>
                <button
                    type="button"
                    className="rounded-full focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
                    aria-label={`${label}: ${help}`}
                >
                    {badge}
                </button>
            </TooltipTrigger>
            <TooltipContent className="max-w-xs">{help}</TooltipContent>
        </Tooltip>
    );
}

/**
 * Why a recipient was not reached, once: the class, then Meta's error code
 * and title. Empty while it is still on its way or reached.
 */
function RecipientReason({ r }: { r: RecipientRow }) {
    const t = useT();
    const inProgress =
        r.state === 'pending' ||
        ['pending', 'sending', 'held'].includes(r.message_status ?? '');
    const cls = inProgress ? '' : r.unreached_class || r.failure_class || '';
    const label = cls
        ? r.unreached_class
            ? unreachedLabel(t, cls)
            : failureClassLabel(t, cls)
        : '';
    // A consent skip's code is the reason the status already names.
    const code = r.message_status === 'skipped' ? null : r.error_code;
    const title =
        r.error_title && r.error_title !== label ? r.error_title : null;
    if (!label && !code && !title)
        return <span className="text-muted-foreground">—</span>;
    return (
        <div
            className="max-w-xs text-xs break-words"
            data-testid="recipient-reason"
        >
            {label && <span className="font-medium">{label}</span>}
            {code && (
                <code className="ml-1 rounded bg-muted px-1 py-0.5">
                    {code}
                </code>
            )}
            {title && (
                <span className="ml-1 text-muted-foreground">{title}</span>
            )}
        </div>
    );
}

function ExportMenu({
    onPick,
    testId,
}: {
    onPick: (format: 'csv' | 'xlsx') => void;
    testId?: string;
}) {
    const t = useT();
    return (
        <DropdownMenu>
            <DropdownMenuTrigger asChild>
                <Button variant="outline" size="sm" data-testid={testId}>
                    <Download /> {t('analytics.recipients.export')}
                </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
                <DropdownMenuItem
                    onSelect={() => onPick('csv')}
                    data-testid={testId && 'export-csv'}
                >
                    {t('analytics.recipients.exportCsv')}
                </DropdownMenuItem>
                <DropdownMenuItem
                    onSelect={() => onPick('xlsx')}
                    data-testid={testId && 'export-xlsx'}
                >
                    {t('analytics.recipients.exportXlsx')}
                </DropdownMenuItem>
            </DropdownMenuContent>
        </DropdownMenu>
    );
}

function TargetMenu({
    icon,
    label,
    items,
    onPick,
}: {
    icon: React.ReactNode;
    label: string;
    items: { id: string; name: string }[];
    onPick: (id: string) => void;
}) {
    return (
        <DropdownMenu>
            <DropdownMenuTrigger asChild>
                <Button size="sm" variant="outline">
                    {icon} {label} <ChevronDown />
                </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent className="max-h-72 overflow-y-auto">
                <DropdownMenuLabel>{label}</DropdownMenuLabel>
                <DropdownMenuSeparator />
                {items.map((x) => (
                    <DropdownMenuItem key={x.id} onSelect={() => onPick(x.id)}>
                        {x.name}
                    </DropdownMenuItem>
                ))}
            </DropdownMenuContent>
        </DropdownMenu>
    );
}

function AdvancedFilters({
    value,
    onChange,
    refs,
}: {
    value: RecipientFilter;
    onChange: (v: RecipientFilter) => void;
    refs: Refs | null;
}) {
    const t = useT();
    const pick = (
        key: 'tag_id' | 'group_id' | 'segment_id',
        label: string,
        items: { id: string; name: string }[],
    ) => (
        <Select
            value={value[key] ?? ANY}
            onValueChange={(v) => {
                const next = { ...value };
                if (v === ANY) delete next[key];
                else next[key] = v;
                onChange(next);
            }}
        >
            <SelectTrigger className="w-44" aria-label={label}>
                <SelectValue placeholder={label} />
            </SelectTrigger>
            <SelectContent>
                <SelectItem value={ANY}>{label}: —</SelectItem>
                {items.map((x) => (
                    <SelectItem key={x.id} value={x.id}>
                        {x.name}
                    </SelectItem>
                ))}
            </SelectContent>
        </Select>
    );
    const [ct, setCt] = useState(value.contact_type ?? '');
    const [key, setKey] = useState(value.attr_key ?? '');
    const [val, setVal] = useState(value.attr_value ?? '');
    return (
        <div className="flex flex-wrap items-center gap-2 rounded-md border p-2">
            {pick('tag_id', t('analytics.breakdown.tag'), refs?.tags ?? [])}
            {pick(
                'group_id',
                t('analytics.breakdown.group'),
                refs?.groups ?? [],
            )}
            {pick(
                'segment_id',
                t('analytics.breakdown.segment'),
                refs?.segments ?? [],
            )}
            <form
                className="flex flex-wrap gap-2"
                onSubmit={(e) => {
                    e.preventDefault();
                    const next = { ...value };
                    if (ct.trim()) next.contact_type = ct.trim();
                    else delete next.contact_type;
                    if (key.trim()) {
                        next.attr_key = key.trim();
                        next.attr_value = val;
                    } else {
                        delete next.attr_key;
                        delete next.attr_value;
                    }
                    onChange(next);
                }}
            >
                <Input
                    value={ct}
                    onChange={(e) => setCt(e.target.value)}
                    placeholder={t('analytics.recipients.contactType')}
                    className="w-36"
                    maxLength={60}
                />
                <Input
                    value={key}
                    onChange={(e) => setKey(e.target.value)}
                    placeholder={t('analytics.recipients.attrKey')}
                    className="w-32"
                    maxLength={100}
                />
                <Input
                    value={val}
                    onChange={(e) => setVal(e.target.value)}
                    placeholder={t('analytics.recipients.attrValue')}
                    className="w-32"
                    maxLength={200}
                />
                <Button type="submit" variant="outline">
                    {t('analytics.breakdown.show')}
                </Button>
            </form>
        </div>
    );
}

function ReactionCell({
    campaignId,
    row,
    canEdit,
}: {
    campaignId: string;
    row: RecipientRow;
    canEdit: boolean;
}) {
    const t = useT();
    const { guard } = useReauth();
    const [reaction, setReaction] = useState(row.reaction);
    const [source, setSource] = useState(row.reaction_source);
    const [pending, startTransition] = useTransition();
    const set = (r: 'positive' | 'negative' | 'neutral' | null) =>
        startTransition(async () => {
            const res = await guard(() =>
                setReactionAction(campaignId, row.id, r),
            );
            if (notify(res)) {
                setReaction(r);
                setSource(r ? 'manual' : null);
            }
        });
    const label = reaction ? t(`analytics.metrics.${reaction}`) : '—';
    const badge = (
        <span className="inline-flex items-center gap-1 text-xs">
            {label}
            {source && (
                <span className="text-muted-foreground">
                    (
                    {source === 'manual'
                        ? t('analytics.recipients.manual')
                        : t('analytics.recipients.system')}
                    )
                </span>
            )}
        </span>
    );
    if (!canEdit) return badge;
    return (
        <DropdownMenu>
            <DropdownMenuTrigger asChild>
                <Button
                    size="sm"
                    variant="ghost"
                    disabled={pending}
                    className="h-7 px-2"
                >
                    {badge}
                </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent>
                <DropdownMenuLabel>
                    {t('analytics.recipients.setReaction')}
                </DropdownMenuLabel>
                {(['positive', 'negative', 'neutral'] as const).map((r) => (
                    <DropdownMenuItem key={r} onSelect={() => set(r)}>
                        {t(`analytics.metrics.${r}`)}
                    </DropdownMenuItem>
                ))}
                <DropdownMenuSeparator />
                <DropdownMenuItem onSelect={() => set(null)}>
                    {t('analytics.recipients.clearReaction')}
                </DropdownMenuItem>
            </DropdownMenuContent>
        </DropdownMenu>
    );
}
