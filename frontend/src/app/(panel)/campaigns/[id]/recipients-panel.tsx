'use client';

import {
    useCallback,
    useEffect,
    useMemo,
    useState,
    useTransition,
} from 'react';
import { useRouter } from 'next/navigation';
import {
    ArrowDown,
    ArrowUp,
    ChevronDown,
    Download,
    Filter,
    Megaphone,
    Tag as TagIcon,
    Users,
} from 'lucide-react';
import { toast } from 'sonner';
import { notify } from '@/components/notify';
import { useReauth } from '@/components/reauth';
import { useFormatDateTime } from '@/components/display-preferences';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
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
import { Spinner } from '@/components/ui/spinner';
import {
    Table,
    TableBody,
    TableCell,
    TableHead,
    TableHeader,
    TableRow,
} from '@/components/ui/table';
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

const PAGE = 50;
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

/** The campaign's recipients: server-side filters, sort, pages, bulk actions. */
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
    const [sort, setSort] = useState('-created');
    const [offset, setOffset] = useState(0);
    const [rows, setRows] = useState<RecipientRow[] | null>(null);
    const [total, setTotal] = useState(0);
    const [selected, setSelected] = useState<Set<string>>(new Set());
    const [allMatching, setAllMatching] = useState(false);
    const [refs, setRefs] = useState<Refs | null>(null);
    const [pending, startTransition] = useTransition();

    const filter = useMemo<RecipientFilter>(() => {
        const f: RecipientFilter = { ...adv };
        if (q.trim()) f.q = q.trim().slice(0, 100);
        if (statuses.length) f.status = statuses;
        return f;
    }, [adv, q, statuses]);
    const filterKey = JSON.stringify(filter);

    const load = useCallback(
        (off: number) =>
            startTransition(async () => {
                const r = await searchRecipientsAction(campaign.id, {
                    filter,
                    sort,
                    offset: off,
                    limit: PAGE,
                });
                if (notify(r)) {
                    setRows(r.data.rows);
                    setTotal(r.data.total);
                }
            }),
        // eslint-disable-next-line react-hooks/exhaustive-deps
        [campaign.id, filterKey, sort],
    );
    useEffect(() => {
        const id = setTimeout(() => {
            setOffset(0);
            setSelected(new Set());
            setAllMatching(false);
            load(0);
        }, 250);
        return () => clearTimeout(id);
    }, [load]);
    useEffect(() => {
        if (!showAdv && !canEdit) return;
        if (refs) return;
        void loadAudienceRefsAction().then((r) => r.ok && setRefs(r.data));
    }, [showAdv, canEdit, refs]);

    const pages = Math.max(1, Math.ceil(total / PAGE));
    const page = Math.floor(offset / PAGE) + 1;
    const go = (off: number) => {
        setOffset(off);
        setSelected(new Set());
        setAllMatching(false);
        load(off);
    };
    const pageIds = rows?.map((r) => r.id) ?? [];
    const pageSelected =
        pageIds.length > 0 && pageIds.every((id) => selected.has(id));
    const count = allMatching ? total : selected.size;
    const selection = (): RecipientFilter =>
        allMatching ? filter : { ...filter, ids: Array.from(selected) };

    const bulk = (
        action: 'add_tag' | 'add_to_group' | 'follow_up',
        target?: string,
    ) =>
        startTransition(async () => {
            const r = await guard(() =>
                bulkRecipientsAction(campaign.id, {
                    action,
                    target_id: target,
                    filter: selection(),
                }),
            );
            if (!notify(r)) return;
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

    const exportAs = (format: 'csv' | 'xlsx') => {
        if (!allMatching && selected.size > 0) {
            // A page selection: the background export takes explicit ids.
            startTransition(async () => {
                const r = await guard(() =>
                    createExportAction(campaign.id, {
                        kind: 'recipients',
                        format,
                        filter: selection(),
                    }),
                );
                if (notify(r))
                    toast.success(t('analytics.recipients.exportQueued'));
            });
            return;
        }
        if (total > SYNC_EXPORT_LIMIT) {
            startTransition(async () => {
                const r = await guard(() =>
                    createExportAction(campaign.id, {
                        kind: 'recipients',
                        format,
                        filter,
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

    const sortBy = (key: string) =>
        setSort((s) => (s === key ? `-${key}` : key));
    const statusLabel = (s: string) =>
        t(`analytics.recipients.statuses.${s}` as MessageKey);

    return (
        <div className="space-y-3" data-testid="recipients-panel">
            <div className="flex flex-wrap items-center gap-2">
                <Input
                    value={q}
                    onChange={(e) => setQ(e.target.value)}
                    placeholder={t('analytics.recipients.search')}
                    className="w-56"
                    maxLength={100}
                    data-testid="recipients-search"
                />
                <DropdownMenu>
                    <DropdownMenuTrigger asChild>
                        <Button
                            variant="outline"
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
                                            : cur.filter((x) => x !== s),
                                    )
                                }
                            >
                                {statusLabel(s)}
                            </DropdownMenuCheckboxItem>
                        ))}
                    </DropdownMenuContent>
                </DropdownMenu>
                <Button variant="outline" onClick={() => setShowAdv((v) => !v)}>
                    <Filter /> {t('analytics.recipients.advanced')}
                </Button>
                {(statuses.length > 0 || q || Object.keys(adv).length > 0) && (
                    <Button
                        variant="ghost"
                        onClick={() => {
                            setQ('');
                            setStatuses([]);
                            setAdv({});
                        }}
                    >
                        {t('analytics.recipients.clear')}
                    </Button>
                )}
                <span
                    className="text-sm text-muted-foreground"
                    data-testid="recipients-total"
                >
                    {t('analytics.recipients.total', {
                        count: total.toLocaleString(NUMBER_LOCALE),
                    })}
                </span>
                {pending && <Spinner />}
                <div className="ml-auto flex gap-2">
                    <DropdownMenu>
                        <DropdownMenuTrigger asChild>
                            <Button
                                variant="outline"
                                data-testid="recipients-export"
                            >
                                <Download /> {t('analytics.recipients.export')}
                            </Button>
                        </DropdownMenuTrigger>
                        <DropdownMenuContent>
                            <DropdownMenuItem
                                onSelect={() => exportAs('csv')}
                                data-testid="export-csv"
                            >
                                {t('analytics.recipients.exportCsv')}
                            </DropdownMenuItem>
                            <DropdownMenuItem
                                onSelect={() => exportAs('xlsx')}
                                data-testid="export-xlsx"
                            >
                                {t('analytics.recipients.exportXlsx')}
                            </DropdownMenuItem>
                        </DropdownMenuContent>
                    </DropdownMenu>
                </div>
            </div>
            {showAdv && (
                <AdvancedFilters value={adv} onChange={setAdv} refs={refs} />
            )}
            {count > 0 && (
                <div className="flex flex-wrap items-center gap-2 rounded-md border bg-muted/40 p-2 text-sm">
                    <span>
                        {allMatching
                            ? t('analytics.recipients.allSelected', {
                                  count: total,
                              })
                            : t('analytics.recipients.selected', { count })}
                    </span>
                    {!allMatching && pageSelected && total > pageIds.length && (
                        <Button
                            size="sm"
                            variant="link"
                            onClick={() => setAllMatching(true)}
                        >
                            {t('analytics.recipients.selectAll', {
                                count: total,
                            })}
                        </Button>
                    )}
                    {canEdit && (
                        <div className="ml-auto flex flex-wrap gap-2">
                            <TargetMenu
                                icon={<TagIcon />}
                                label={t('analytics.recipients.addTag')}
                                items={
                                    refs?.tags.map((x) => ({
                                        id: x.id,
                                        name: x.name,
                                    })) ?? []
                                }
                                onPick={(id) => bulk('add_tag', id)}
                            />
                            <TargetMenu
                                icon={<Users />}
                                label={t('analytics.recipients.addToGroup')}
                                items={
                                    refs?.groups.map((x) => ({
                                        id: x.id,
                                        name: x.name,
                                    })) ?? []
                                }
                                onPick={(id) => bulk('add_to_group', id)}
                            />
                            <Button
                                size="sm"
                                variant="outline"
                                onClick={() => bulk('follow_up')}
                                data-testid="follow-up"
                            >
                                <Megaphone />{' '}
                                {t('analytics.recipients.followUp')}
                            </Button>
                        </div>
                    )}
                </div>
            )}
            <div className="overflow-x-auto">
                <Table>
                    <TableHeader>
                        <TableRow>
                            <TableHead className="w-8">
                                <Checkbox
                                    checked={pageSelected}
                                    aria-label={t(
                                        'analytics.recipients.selectPage',
                                    )}
                                    onCheckedChange={(on) => {
                                        setAllMatching(false);
                                        setSelected(
                                            on ? new Set(pageIds) : new Set(),
                                        );
                                    }}
                                />
                            </TableHead>
                            <SortHead
                                sort={sort}
                                onSort={sortBy}
                                k="phone"
                                label={t('analytics.recipients.columns.phone')}
                            />
                            <SortHead
                                sort={sort}
                                onSort={sortBy}
                                k="name"
                                label={t('analytics.recipients.columns.name')}
                            />
                            <TableHead>
                                {t('analytics.recipients.columns.outcome')}
                            </TableHead>
                            <SortHead
                                sort={sort}
                                onSort={sortBy}
                                k="sent_at"
                                label={t('analytics.recipients.columns.sent')}
                            />
                            <SortHead
                                sort={sort}
                                onSort={sortBy}
                                k="delivered_at"
                                label={t(
                                    'analytics.recipients.columns.delivered',
                                )}
                            />
                            <SortHead
                                sort={sort}
                                onSort={sortBy}
                                k="read_at"
                                label={t('analytics.recipients.columns.read')}
                            />
                            <SortHead
                                sort={sort}
                                onSort={sortBy}
                                k="replied_at"
                                label={t(
                                    'analytics.recipients.columns.replied',
                                )}
                            />
                            <TableHead>
                                {t('analytics.recipients.columns.reaction')}
                            </TableHead>
                            <SortHead
                                sort={sort}
                                onSort={sortBy}
                                k="cost"
                                label={t('analytics.recipients.columns.cost')}
                            />
                        </TableRow>
                    </TableHeader>
                    <TableBody>
                        {rows?.length === 0 && (
                            <TableRow>
                                <TableCell
                                    colSpan={10}
                                    className="text-center text-sm text-muted-foreground"
                                >
                                    {t('analytics.recipients.empty')}
                                </TableCell>
                            </TableRow>
                        )}
                        {rows?.map((r) => (
                            <TableRow
                                key={r.id}
                                data-testid="analysis-recipient-row"
                                data-phone={r.phone}
                            >
                                <TableCell>
                                    <Checkbox
                                        checked={
                                            allMatching || selected.has(r.id)
                                        }
                                        aria-label={r.phone}
                                        onCheckedChange={(on) => {
                                            setAllMatching(false);
                                            setSelected((cur) => {
                                                const next = new Set(cur);
                                                if (on) next.add(r.id);
                                                else next.delete(r.id);
                                                return next;
                                            });
                                        }}
                                    />
                                </TableCell>
                                <TableCell className="font-mono text-xs">
                                    {r.phone}
                                </TableCell>
                                <TableCell>
                                    {r.name}
                                    {r.contact_type && (
                                        <span className="ml-1 text-xs text-muted-foreground">
                                            · {r.contact_type}
                                        </span>
                                    )}
                                </TableCell>
                                <TableCell>
                                    <Badge variant="outline">
                                        {t(
                                            `campaigns.engine.outcomes.${r.outcome}` as MessageKey,
                                        )}
                                    </Badge>
                                    {r.unreached_class && (
                                        <span className="ml-1 text-xs text-muted-foreground">
                                            {statusLabel(r.unreached_class)}
                                        </span>
                                    )}
                                    {r.error_code && (
                                        <span className="ml-1 font-mono text-xs text-muted-foreground">
                                            {r.error_code}
                                        </span>
                                    )}
                                </TableCell>
                                <TableCell className="text-xs">
                                    {r.sent_at
                                        ? formatDateTime(r.sent_at)
                                        : '—'}
                                </TableCell>
                                <TableCell className="text-xs">
                                    {r.delivered_at
                                        ? formatDateTime(r.delivered_at)
                                        : '—'}
                                </TableCell>
                                <TableCell className="text-xs">
                                    {r.read_at
                                        ? formatDateTime(r.read_at)
                                        : '—'}
                                </TableCell>
                                <TableCell className="text-xs">
                                    {r.replied_at
                                        ? formatDateTime(r.replied_at)
                                        : '—'}
                                    {r.opted_out_at && (
                                        <Badge
                                            variant="destructive"
                                            className="ml-1"
                                        >
                                            {statusLabel('opted_out')}
                                        </Badge>
                                    )}
                                </TableCell>
                                <TableCell>
                                    <ReactionCell
                                        campaignId={campaign.id}
                                        row={r}
                                        canEdit={canEdit}
                                    />
                                </TableCell>
                                <TableCell className="text-xs tabular-nums">
                                    {r.cost
                                        ? `${Number(r.cost).toLocaleString(NUMBER_LOCALE, { maximumFractionDigits: 4 })} ${r.cost_currency ?? ''}`
                                        : '—'}
                                </TableCell>
                            </TableRow>
                        ))}
                    </TableBody>
                </Table>
            </div>
            <div className="flex items-center justify-end gap-2 text-sm">
                <span className="text-muted-foreground">
                    {t('analytics.recipients.page', { page, pages })}
                </span>
                <Button
                    size="sm"
                    variant="outline"
                    disabled={offset === 0 || pending}
                    onClick={() => go(Math.max(0, offset - PAGE))}
                >
                    {t('analytics.recipients.prev')}
                </Button>
                <Button
                    size="sm"
                    variant="outline"
                    disabled={page >= pages || pending}
                    onClick={() => go(offset + PAGE)}
                >
                    {t('analytics.recipients.next')}
                </Button>
            </div>
        </div>
    );
}

function SortHead({
    k,
    label,
    sort,
    onSort,
}: {
    k: string;
    label: string;
    sort: string;
    onSort: (k: string) => void;
}) {
    return (
        <TableHead>
            <button
                type="button"
                className="inline-flex items-center gap-1"
                onClick={() => onSort(k)}
            >
                {label}
                {sort === k && <ArrowUp className="size-3" />}
                {sort === `-${k}` && <ArrowDown className="size-3" />}
            </button>
        </TableHead>
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
