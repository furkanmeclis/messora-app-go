'use client';

import Link from 'next/link';
import { useRouter } from 'next/navigation';
import {
    useEffect,
    useLayoutEffect,
    useMemo,
    useRef,
    useState,
    useTransition,
} from 'react';
import {
    AlertCircle,
    ArrowDown,
    ArrowLeft,
    Ban,
    Check,
    CheckCheck,
    Clock,
    ExternalLink,
    FileText,
    HelpCircle,
    Inbox as InboxIcon,
    LayoutTemplate,
    MessageSquare,
    Paperclip,
    PauseCircle,
    Search,
    Send,
    Sparkles,
    StickyNote,
    X,
} from 'lucide-react';
import { newIdempotencyKey, notify } from '@/components/notify';
import {
    ParamIssues,
    TemplateParamsEditor,
} from '@/components/template-params';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Avatar, AvatarFallback } from '@/components/ui/avatar';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Combobox } from '@/components/ui/combobox';
import {
    Empty,
    EmptyDescription,
    EmptyHeader,
    EmptyMedia,
    EmptyTitle,
} from '@/components/ui/empty';
import { FileDrop } from '@/components/ui/file-drop';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import {
    Popover,
    PopoverContent,
    PopoverTrigger,
} from '@/components/ui/popover';
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from '@/components/ui/select';
import { Spinner } from '@/components/ui/spinner';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Textarea } from '@/components/ui/textarea';
import {
    useDisplayPrefs,
    useFormatDate,
    useFormatDateTime,
    useFormatTimeLabel,
} from '@/components/display-preferences';
import { formatTime } from '@/lib/format';
import { WhatsAppPreview } from '@/components/whatsapp-preview';
import { useT } from '@/i18n/client';
import type { MessageKey, Translate } from '@/i18n/translate';
import {
    emptyValues,
    issuesOf,
    paramsFromValues,
    shapeOf,
    templateKey,
    type ParamValues,
    type SampleContact,
} from '@/lib/templates';
import { cn } from '@/lib/utils';
import { isConsentReason } from '@/lib/consent';
import type {
    Conversation,
    ConversationNote,
    InboxMessage,
    Template,
} from '@/lib/types';
import type { ThreadData } from './page';
import {
    ActivityTimeline,
    AutomationRuns,
    ContactAISwitch,
    DraftCard,
    EscalationBanner,
    isAIMessage,
    NoteBubble,
    NoteComposer,
    OwnerControls,
} from './inbox-ai';
import {
    addNoteAction,
    assignAction,
    markReadAction,
    moreConversationsAction,
    olderMessagesAction,
    replyAction,
    setStatusAction,
} from './actions';

type UserOption = { id: string; name: string };

function displayName(c: Conversation['contact']): string {
    return c.name || c.profile_name || c.phone;
}

/** The translated name of a WhatsApp message type, or the type itself. */
function kindLabel(t: Translate, type: string): string {
    const key = `overview.live.kinds.${type}` as MessageKey;
    const text = t(key);
    return text === key ? type : text;
}

function inboxHref(query: string, extra: Record<string, string>): string {
    const q = new URLSearchParams(query);
    for (const [k, v] of Object.entries(extra)) {
        if (v) q.set(k, v);
        else q.delete(k);
    }
    const s = q.toString();
    return s ? `/inbox?${s}` : '/inbox';
}

const TONES = [
    'bg-rose-500/15 text-rose-700 dark:text-rose-300',
    'bg-amber-500/15 text-amber-700 dark:text-amber-300',
    'bg-emerald-500/15 text-emerald-700 dark:text-emerald-300',
    'bg-sky-500/15 text-sky-700 dark:text-sky-300',
    'bg-violet-500/15 text-violet-700 dark:text-violet-300',
    'bg-teal-500/15 text-teal-700 dark:text-teal-300',
];

function initials(name: string): string {
    const parts = name.replace(/^\+/, '').trim().split(/\s+/);
    if (/^\d/.test(parts[0] ?? '')) return '#';
    return (
        ((parts[0]?.[0] ?? '') + (parts[1]?.[0] ?? '')).toLocaleUpperCase() ||
        '?'
    );
}

function ContactAvatar({
    name,
    id,
    className,
}: {
    name: string;
    id: string;
    className?: string;
}) {
    let h = 0;
    for (const ch of id) h = (h * 31 + ch.charCodeAt(0)) >>> 0;
    return (
        <Avatar className={cn('size-9', className)}>
            <AvatarFallback
                className={cn('text-xs font-semibold', TONES[h % TONES.length])}
            >
                {initials(name)}
            </AvatarFallback>
        </Avatar>
    );
}

// ── Filters ────────────────────────────────────────────────────────────────

const STATUSES = [
    ['open', 'inbox.filters.open'],
    ['closed', 'inbox.filters.closed'],
    ['all', 'inbox.filters.all'],
] as const;
const ASSIGNEES = [
    ['', 'inbox.filters.everyone'],
    ['me', 'inbox.filters.mine'],
    ['unassigned', 'inbox.filters.unassigned'],
] as const;
const AI_FILTERS = [
    ['ai', 'inbox.ai.filters.ai'],
    ['waiting_human', 'inbox.ai.filters.waiting'],
] as const;

export function InboxFilters({
    title,
    unreadTotal,
    status,
    assignee,
    unread,
    ai,
    q,
}: {
    title: string;
    unreadTotal: number;
    status: string;
    assignee: string;
    unread: boolean;
    ai: string;
    q: string;
}) {
    const t = useT();
    const router = useRouter();
    const [text, setText] = useState(q);
    const base = new URLSearchParams();
    if (status !== 'open') base.set('status', status);
    if (assignee) base.set('assignee', assignee);
    if (unread) base.set('unread', '1');
    if (q) base.set('q', q);
    if (ai) base.set('ai', ai);
    const query = base.toString();

    // Search as you type (after a short pause).
    useEffect(() => {
        if (text.trim() === q) return;
        const id = setTimeout(
            () => router.replace(inboxHref(query, { q: text.trim(), c: '' })),
            300,
        );
        return () => clearTimeout(id);
    }, [text, q, query, router]);

    const pill = (active: boolean) =>
        cn(
            'rounded-full px-2.5 py-1 text-xs font-medium outline-none focus-visible:ring-2 focus-visible:ring-ring',
            active
                ? 'bg-primary text-primary-foreground'
                : 'text-muted-foreground hover:bg-muted',
        );
    return (
        <div className="space-y-2 border-b p-3">
            <div className="flex items-center justify-between gap-2">
                <h1 className="text-base font-semibold">{title}</h1>
                {unreadTotal > 0 && (
                    <Badge
                        className="rounded-full"
                        aria-label={t('inbox.list.unread', {
                            count: unreadTotal,
                        })}
                    >
                        {unreadTotal}
                    </Badge>
                )}
            </div>
            <form
                role="search"
                className="relative"
                onSubmit={(e) => {
                    e.preventDefault();
                    router.replace(inboxHref(query, { q: text.trim(), c: '' }));
                }}
            >
                <Label htmlFor="inbox-search" className="sr-only">
                    {t('inbox.filters.searchLabel')}
                </Label>
                <Search className="pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2 text-muted-foreground" />
                <Input
                    id="inbox-search"
                    value={text}
                    maxLength={100}
                    data-search
                    className="pl-8"
                    placeholder={t('inbox.filters.searchPlaceholder')}
                    onChange={(e) => setText(e.target.value)}
                />
            </form>
            <nav
                aria-label={t('inbox.filters.status')}
                className="flex flex-wrap gap-1"
            >
                {STATUSES.map(([v, label]) => (
                    <Link
                        key={v}
                        href={inboxHref(query, {
                            status: v === 'open' ? '' : v,
                            c: '',
                        })}
                        aria-current={status === v ? 'page' : undefined}
                        className={pill(status === v)}
                    >
                        {t(label)}
                    </Link>
                ))}
                <Link
                    href={inboxHref(query, {
                        unread: unread ? '' : '1',
                        c: '',
                    })}
                    aria-current={unread ? 'page' : undefined}
                    className={pill(unread)}
                >
                    {t('inbox.filters.unread')}
                </Link>
            </nav>
            <nav
                aria-label={t('inbox.filters.assignee')}
                className="flex flex-wrap gap-1"
            >
                {ASSIGNEES.map(([v, label]) => (
                    <Link
                        key={v || 'all'}
                        href={inboxHref(query, { assignee: v, c: '' })}
                        aria-current={assignee === v ? 'page' : undefined}
                        className={pill(assignee === v)}
                    >
                        {t(label)}
                    </Link>
                ))}
            </nav>
            <nav
                aria-label={t('inbox.ai.filters.label')}
                className="flex flex-wrap gap-1"
            >
                {AI_FILTERS.map(([v, label]) => (
                    <Link
                        key={v}
                        href={inboxHref(query, {
                            ai: ai === v ? '' : v,
                            c: '',
                        })}
                        aria-current={ai === v ? 'page' : undefined}
                        className={pill(ai === v)}
                        data-testid={`filter-${v}`}
                    >
                        {t(label)}
                    </Link>
                ))}
            </nav>
        </div>
    );
}

// ── List ───────────────────────────────────────────────────────────────────

export function ConversationList({
    initial,
    initialCursor,
    query,
    selected,
}: {
    initial: Conversation[];
    initialCursor?: string;
    query: string;
    selected?: string;
}) {
    const t = useT();
    const timeLabel = useFormatTimeLabel();
    const [more, setMore] = useState<Conversation[]>([]);
    const [cursor, setCursor] = useState<string | undefined>(undefined);
    const [loaded, setLoaded] = useState(false);
    const [pending, startTransition] = useTransition();
    // Live refreshes replace the first page; extra pages stay below it.
    const ids = new Set(initial.map((c) => c.id));
    const items = [...initial, ...more.filter((c) => !ids.has(c.id))];
    const next = loaded ? cursor : initialCursor;

    const sentinel = useRef<HTMLDivElement>(null);
    const loadMore = () =>
        startTransition(async () => {
            if (!next) return;
            const r = await moreConversationsAction(query, next);
            if (notify(r)) {
                setMore((m) => [...m, ...r.data.items]);
                setCursor(r.data.next);
                setLoaded(true);
            }
        });
    // The next page loads when the end of the list scrolls into view.
    useEffect(() => {
        const el = sentinel.current;
        if (!el || !next) return;
        const io = new IntersectionObserver(
            (entries) => {
                if (entries.some((e) => e.isIntersecting) && !pending)
                    loadMore();
            },
            { rootMargin: '200px' },
        );
        io.observe(el);
        return () => io.disconnect();
        // eslint-disable-next-line react-hooks/exhaustive-deps -- loadMore reads the latest cursor
    }, [next, pending]);

    if (items.length === 0) {
        return (
            <Empty className="flex-1 py-12">
                <EmptyHeader>
                    <EmptyMedia variant="icon">
                        <InboxIcon />
                    </EmptyMedia>
                    <EmptyTitle>{t('inbox.list.empty')}</EmptyTitle>
                    <EmptyDescription>
                        {t('inbox.list.emptyHint')}
                    </EmptyDescription>
                </EmptyHeader>
            </Empty>
        );
    }
    return (
        <div className="min-h-0 flex-1 overflow-y-auto">
            <ul data-testid="conversation-list">
                {items.map((c) => {
                    const name = displayName(c.contact);
                    const last = c.last_message;
                    return (
                        <li key={c.id}>
                            <Link
                                href={inboxHref(query, { c: c.id })}
                                aria-current={
                                    c.id === selected ? 'true' : undefined
                                }
                                data-testid="conversation-item"
                                className={cn(
                                    'relative flex gap-3 border-b px-3 py-2.5 outline-none hover:bg-muted/60 focus-visible:bg-muted focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-inset',
                                    c.unread_count > 0 && 'bg-primary/[0.04]',
                                    c.id === selected && 'bg-muted',
                                )}
                            >
                                {c.unread_count > 0 && (
                                    <span
                                        aria-hidden
                                        className="absolute inset-y-0 left-0 w-0.5 bg-primary"
                                    />
                                )}
                                <ContactAvatar
                                    name={name}
                                    id={c.contact.id}
                                    className="mt-0.5"
                                />
                                <span className="flex min-w-0 flex-1 flex-col gap-0.5">
                                    <span className="flex items-center justify-between gap-2">
                                        <span
                                            className={cn(
                                                'truncate text-sm',
                                                c.unread_count > 0 &&
                                                    'font-semibold',
                                            )}
                                        >
                                            {name}
                                        </span>
                                        <time
                                            dateTime={c.last_message_at}
                                            className={cn(
                                                'shrink-0 text-xs tabular-nums',
                                                c.unread_count > 0
                                                    ? 'font-medium text-primary'
                                                    : 'text-muted-foreground',
                                            )}
                                        >
                                            {timeLabel(c.last_message_at)}
                                        </time>
                                    </span>
                                    <span className="flex items-center justify-between gap-2">
                                        <span
                                            className={cn(
                                                'flex min-w-0 items-center gap-1 text-xs',
                                                c.unread_count > 0
                                                    ? 'font-medium text-foreground'
                                                    : 'text-muted-foreground',
                                            )}
                                        >
                                            {last?.direction === 'outbound' && (
                                                <CheckCheck className="size-3.5 shrink-0" />
                                            )}
                                            <span className="truncate">
                                                {last
                                                    ? (last.direction ===
                                                      'outbound'
                                                          ? t('inbox.list.you')
                                                          : '') +
                                                      (last.body ||
                                                          `[${kindLabel(t, last.type)}]`)
                                                    : '—'}
                                            </span>
                                        </span>
                                        <span className="flex shrink-0 items-center gap-1">
                                            {c.owner_type === 'ai' && (
                                                <Badge
                                                    className="h-4 gap-0.5 bg-violet-600 px-1.5 text-[10px] text-white"
                                                    data-testid="list-ai"
                                                >
                                                    <Sparkles className="size-2.5" />
                                                    {t('inbox.ai.aiLabel')}
                                                </Badge>
                                            )}
                                            {c.pending_handoff_id && (
                                                <Badge
                                                    variant="outline"
                                                    className="h-4 border-amber-500 px-1.5 text-[10px] text-amber-700 dark:text-amber-300"
                                                    data-testid="list-waiting"
                                                >
                                                    {t('inbox.ai.waiting')}
                                                </Badge>
                                            )}
                                            {c.status === 'closed' && (
                                                <Badge
                                                    variant="outline"
                                                    className="h-4 px-1.5 text-[10px]"
                                                >
                                                    {t('inbox.filters.closed')}
                                                </Badge>
                                            )}
                                            {c.unread_count > 0 && (
                                                <Badge
                                                    className="h-5 min-w-5 rounded-full px-1.5"
                                                    aria-label={t(
                                                        'inbox.list.unread',
                                                        {
                                                            count: c.unread_count,
                                                        },
                                                    )}
                                                    data-testid="unread-count"
                                                >
                                                    {c.unread_count}
                                                </Badge>
                                            )}
                                        </span>
                                    </span>
                                    {c.assignee && (
                                        <span className="truncate text-[11px] text-muted-foreground">
                                            {t('inbox.list.assignedTo', {
                                                name: c.assignee.name,
                                            })}
                                        </span>
                                    )}
                                </span>
                            </Link>
                        </li>
                    );
                })}
            </ul>
            {next && (
                <div ref={sentinel} className="flex justify-center p-3">
                    {pending ? (
                        <Spinner />
                    ) : (
                        <Button variant="ghost" size="sm" onClick={loadMore}>
                            {t('inbox.list.loadMore')}
                        </Button>
                    )}
                </div>
            )}
        </div>
    );
}

// ── Thread ─────────────────────────────────────────────────────────────────

function Ticks({ status }: { status: InboxMessage['status'] }) {
    const t = useT();
    const label = t(`inbox.status.${status}`);
    const icon =
        status === 'pending' || status === 'sending' ? (
            <Clock className="size-3.5" />
        ) : status === 'sent' ? (
            <Check className="size-3.5" />
        ) : status === 'delivered' ? (
            <CheckCheck className="size-3.5" />
        ) : status === 'read' ? (
            <CheckCheck className="size-3.5 text-sky-500" />
        ) : status === 'failed' ? (
            <AlertCircle className="size-3.5 text-destructive" />
        ) : status === 'deferred' ? (
            <Clock className="size-3.5 text-amber-600" />
        ) : status === 'held' ? (
            <PauseCircle className="size-3.5" />
        ) : status === 'skipped' ? (
            <Ban className="size-3.5 text-destructive" />
        ) : status === 'cancelled' ? (
            <Ban className="size-3.5" />
        ) : (
            <HelpCircle className="size-3.5" />
        );
    return (
        <span role="img" aria-label={label} title={label} data-testid="ticks">
            {icon}
        </span>
    );
}

function MediaView({
    conversationId,
    m,
}: {
    conversationId: string;
    m: InboxMessage;
}) {
    const t = useT();
    const md = m.media;
    if (!md) return null;
    if (md.status !== 'stored') {
        return (
            <p className="text-xs italic opacity-80">
                {md.status === 'failed'
                    ? t('inbox.media.failed')
                    : t('inbox.media.downloading')}
            </p>
        );
    }
    const src = `/bff/media/${conversationId}/${md.id}`;
    const alt =
        m.body || t('inbox.media.fromCustomer', { type: kindLabel(t, m.type) });
    if (m.type === 'image' || m.type === 'sticker') {
        return (
            // eslint-disable-next-line @next/next/no-img-element -- served by the BFF with the session, not optimisable
            <img
                src={src}
                alt={alt}
                className="max-h-72 rounded-md"
                loading="lazy"
            />
        );
    }
    if (m.type === 'video')
        return <video src={src} controls className="max-h-72 rounded-md" />;
    if (m.type === 'audio')
        return <audio src={src} controls aria-label={t('inbox.media.voice')} />;
    return (
        <a
            href={src}
            className="flex items-center gap-2 rounded-md bg-black/5 p-2 underline-offset-2 hover:underline dark:bg-white/10"
            download={md.filename || undefined}
        >
            <FileText className="size-5 shrink-0" />{' '}
            <span className="truncate">
                {md.filename || t('inbox.media.download')}
            </span>
        </a>
    );
}

function Bubble({
    conversationId,
    m,
}: {
    conversationId: string;
    m: InboxMessage;
}) {
    const t = useT();
    const prefs = useDisplayPrefs();
    const formatDateTime = useFormatDateTime();
    const out = m.direction === 'outbound';
    const byAI = isAIMessage(m);
    const at = m.direction === 'inbound' ? m.occurred_at : m.created_at;
    return (
        <li
            className={cn('flex', out ? 'justify-end' : 'justify-start')}
            data-testid="message"
            data-direction={m.direction}
            data-status={m.status}
            data-ai={byAI ? 'true' : undefined}
        >
            <div
                className={cn(
                    'max-w-[78%] space-y-1 rounded-lg px-2.5 py-1.5 text-sm shadow-sm',
                    out
                        ? 'rounded-tr-none bg-[#d9fdd3] text-[#111b21] dark:bg-[#005c4b] dark:text-[#e9edef]'
                        : 'rounded-tl-none bg-white text-[#111b21] dark:bg-[#202c33] dark:text-[#e9edef]',
                    byAI &&
                        'bg-violet-100 ring-1 ring-violet-400/60 dark:bg-violet-950 dark:ring-violet-500/50',
                    (m.status === 'failed' || m.status === 'skipped') &&
                        'ring-1 ring-destructive/60',
                )}
            >
                {byAI && (
                    <p
                        className="flex items-center gap-1 text-[11px] font-medium text-violet-700 dark:text-violet-300"
                        data-testid="ai-label"
                    >
                        <Sparkles className="size-3" />
                        {t('inbox.ai.aiLabel')}
                    </p>
                )}
                <MediaView conversationId={conversationId} m={m} />
                {m.body && (
                    <p className="break-words whitespace-pre-wrap">{m.body}</p>
                )}
                {!m.body && !m.media && (
                    <p className="italic opacity-80">[{m.type}]</p>
                )}
                {m.status === 'skipped' && (
                    <p
                        className="flex items-start gap-1 text-xs text-destructive"
                        data-testid="message-skipped"
                    >
                        <Ban className="mt-0.5 size-3 shrink-0" />
                        {isConsentReason(m.error_code)
                            ? t(`consent.reasons.${m.error_code}`)
                            : t('inbox.status.skipped')}
                    </p>
                )}
                {m.status === 'failed' && m.error_title && (
                    <p className="flex items-start gap-1 text-xs text-destructive">
                        <AlertCircle className="mt-0.5 size-3 shrink-0" />
                        {m.error_title}
                    </p>
                )}
                <p className="flex items-center justify-end gap-1 text-[11px] opacity-60">
                    <time dateTime={at} title={formatDateTime(at)}>
                        {formatTime(at, prefs)}
                    </time>
                    {out && <Ticks status={m.status} />}
                </p>
            </div>
        </li>
    );
}

export function Thread({
    selected,
    data,
    me,
    users,
    canReply,
    canAssign,
    canNote,
    backHref,
}: {
    selected?: string;
    data: ThreadData | null;
    me: string;
    users: UserOption[];
    canReply: boolean;
    canAssign: boolean;
    canNote: boolean;
    backHref: string;
}) {
    const t = useT();
    if (!selected || !data) {
        return (
            <Empty className="h-full">
                <EmptyHeader>
                    <EmptyMedia variant="icon">
                        <MessageSquare />
                    </EmptyMedia>
                    <EmptyTitle>
                        {selected
                            ? t('inbox.thread.notFound')
                            : t('inbox.thread.select')}
                    </EmptyTitle>
                    <EmptyDescription>
                        {selected
                            ? t('inbox.thread.notFoundHint')
                            : t('inbox.thread.selectHint')}
                    </EmptyDescription>
                </EmptyHeader>
            </Empty>
        );
    }
    return (
        <div className="grid h-full min-h-0 xl:grid-cols-[1fr_18rem]">
            <ThreadMain
                data={data}
                me={me}
                users={users}
                canReply={canReply}
                canAssign={canAssign}
                backHref={backHref}
            />
            <ContactPanel data={data} canNote={canNote} />
        </div>
    );
}

function ThreadMain({
    data,
    me,
    users,
    canReply,
    canAssign,
    backHref,
}: {
    data: ThreadData;
    me: string;
    users: UserOption[];
    canReply: boolean;
    canAssign: boolean;
    backHref: string;
}) {
    const t = useT();
    const router = useRouter();
    const formatDateTime = useFormatDateTime();
    const formatDate = useFormatDate();
    const c = data.conversation;
    const [older, setOlder] = useState<InboxMessage[]>([]);
    const [olderCursor, setOlderCursor] = useState<string | undefined>();
    const [loadedOlder, setLoadedOlder] = useState(false);
    const [pending, startTransition] = useTransition();
    const scroller = useRef<HTMLDivElement>(null);
    const topSentinel = useRef<HTMLDivElement>(null);
    const anchor = useRef<{ height: number; top: number } | null>(null);
    const nearBottom = useRef(true);
    const [unseen, setUnseen] = useState(false);
    const [today] = useState(() => Date.now());

    // The API pages newest first; the thread reads oldest first.
    const recentIds = new Set(data.messages.map((m) => m.id));
    const messages = [
        ...older.filter((m) => !recentIds.has(m.id)).reverse(),
        ...[...data.messages].reverse(),
    ];
    const next = loadedOlder ? olderCursor : data.nextCursor;
    const newest = data.messages[0]?.id ?? '';
    const newestNote = data.convNotes[0]?.id ?? '';
    // Internal notes sit in the thread by time, within the loaded messages.
    const msgAt = (m: InboxMessage) =>
        m.direction === 'inbound' ? m.occurred_at : m.created_at;
    const oldestAt = messages[0] ? msgAt(messages[0]) : '';
    const entries: (
        | { kind: 'm'; m: InboxMessage; at: string }
        | { kind: 'n'; n: ConversationNote; at: string }
    )[] = [
        ...messages.map((m) => ({ kind: 'm' as const, m, at: msgAt(m) })),
        ...data.convNotes
            .filter((n) => !next || n.created_at >= oldestAt)
            .map((n) => ({ kind: 'n' as const, n, at: n.created_at })),
    ].sort((a, b) => (a.at < b.at ? -1 : a.at > b.at ? 1 : 0));
    const draft = data.drafts.find((d) => d.status === 'pending');

    // Opening a conversation (or a new customer message while it is open)
    // marks it read for the team; the list's badge follows.
    useEffect(() => {
        if (c.unread_count > 0)
            void markReadAction(c.id).then(() => router.refresh());
    }, [c.id, c.unread_count, router]);

    const toBottom = (smooth = false) => {
        const el = scroller.current;
        if (el)
            el.scrollTo({
                top: el.scrollHeight,
                behavior: smooth ? 'smooth' : 'auto',
            });
        setUnseen(false);
    };

    // A new message: follow it when the reader is at the bottom, otherwise
    // offer a jump instead of moving the text they are reading.
    useEffect(() => {
        if (nearBottom.current) toBottom();
        else setUnseen(true);
    }, [newest, newestNote]);

    const loadOlder = () => {
        const el = scroller.current;
        if (!next || pending || !el) return;
        anchor.current = { height: el.scrollHeight, top: el.scrollTop };
        startTransition(async () => {
            const r = await olderMessagesAction(c.id, next);
            if (notify(r)) {
                setOlder((o) => [...o, ...r.data.items]);
                setOlderCursor(r.data.next);
                setLoadedOlder(true);
            }
        });
    };

    // Older messages are inserted above: keep the visible ones in place.
    useLayoutEffect(() => {
        const el = scroller.current;
        const a = anchor.current;
        if (!el || !a) return;
        el.scrollTop = el.scrollHeight - a.height + a.top;
        anchor.current = null;
    }, [older.length]);

    // Scrolling near the top loads the previous page, like WhatsApp.
    useEffect(() => {
        const el = topSentinel.current;
        const root = scroller.current;
        if (!el || !root || !next) return;
        const io = new IntersectionObserver(
            (e) => {
                if (e.some((x) => x.isIntersecting)) loadOlder();
            },
            { root, rootMargin: '300px 0px 0px 0px' },
        );
        io.observe(el);
        return () => io.disconnect();
        // eslint-disable-next-line react-hooks/exhaustive-deps -- loadOlder reads the latest cursor
    }, [next, pending]);

    const dayLabel = (iso: string) => {
        const d = formatDate(iso);
        if (d === formatDate(new Date(today).toISOString()))
            return t('inbox.thread.today');
        if (d === formatDate(new Date(today - 86_400_000).toISOString()))
            return t('inbox.thread.yesterday');
        return d;
    };
    const name = displayName(c.contact);

    return (
        <div className="flex min-h-0 flex-col border-r">
            <header className="flex flex-wrap items-center gap-3 border-b px-3 py-2.5">
                <Button
                    asChild
                    size="icon"
                    variant="ghost"
                    className="lg:hidden"
                >
                    <Link href={backHref} aria-label={t('inbox.thread.back')}>
                        <ArrowLeft />
                    </Link>
                </Button>
                <ContactAvatar name={name} id={c.contact.id} />
                <div className="min-w-[8rem] flex-1">
                    <h2
                        className="truncate text-sm font-semibold"
                        data-testid="thread-title"
                    >
                        {name}
                    </h2>
                    <div className="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1 text-xs text-muted-foreground">
                        <span className="truncate">
                            {t('inbox.thread.via', {
                                phone: c.contact.phone,
                                channel:
                                    c.phone_number.verified_name ||
                                    c.phone_number.display_phone_number,
                            })}
                        </span>
                        {c.window_open && c.window_expires_at ? (
                            <span
                                className="inline-flex items-center gap-1 text-emerald-700 dark:text-emerald-400"
                                title={t('inbox.thread.windowHint')}
                            >
                                <Clock className="size-3" />
                                {t('inbox.thread.windowUntil', {
                                    time: formatDateTime(c.window_expires_at),
                                })}
                            </span>
                        ) : (
                            <span className="inline-flex items-center gap-1">
                                <Clock className="size-3" />
                                {t('inbox.thread.windowClosed')}
                            </span>
                        )}
                        {c.contact.opted_out && (
                            <Badge
                                variant="destructive"
                                className="h-4 px-1.5 text-[10px]"
                            >
                                {t('inbox.thread.optedOut')}
                            </Badge>
                        )}
                        {c.status === 'closed' && (
                            <Badge
                                variant="outline"
                                className="h-4 px-1.5 text-[10px]"
                            >
                                {t('inbox.filters.closed')}
                            </Badge>
                        )}
                    </div>
                </div>
                {canAssign && (
                    <ConversationActions c={c} me={me} users={users} />
                )}
                <div className="basis-full">
                    <OwnerControls c={c} canReply={canReply} />
                </div>
            </header>
            {data.pendingHandoff && (
                <EscalationBanner
                    c={c}
                    handoff={data.pendingHandoff}
                    canReply={canReply}
                />
            )}
            <div className="relative min-h-0 flex-1">
                <div
                    ref={scroller}
                    className="h-full overflow-y-auto bg-[#efeae2] px-3 py-4 dark:bg-[#0b141a]"
                    tabIndex={0}
                    aria-label={t('inbox.thread.messages')}
                    onScroll={(e) => {
                        const el = e.currentTarget;
                        nearBottom.current =
                            el.scrollHeight - el.scrollTop - el.clientHeight <
                            120;
                        if (nearBottom.current && unseen) setUnseen(false);
                    }}
                >
                    {next && (
                        <div
                            ref={topSentinel}
                            className="mb-3 flex justify-center"
                        >
                            {pending ? (
                                <Spinner />
                            ) : (
                                <Button
                                    size="sm"
                                    variant="secondary"
                                    onClick={loadOlder}
                                >
                                    {t('inbox.thread.loadOlder')}
                                </Button>
                            )}
                        </div>
                    )}
                    <ol
                        className="mx-auto max-w-3xl space-y-1.5"
                        aria-live="polite"
                        data-testid="thread"
                    >
                        {entries.map((e, i) => {
                            const prev = entries[i - 1];
                            const newDay =
                                !prev ||
                                formatDate(prev.at) !== formatDate(e.at);
                            return (
                                <MessageWithDay
                                    key={e.kind === 'm' ? e.m.id : e.n.id}
                                    day={newDay ? dayLabel(e.at) : null}
                                >
                                    {e.kind === 'm' ? (
                                        <Bubble conversationId={c.id} m={e.m} />
                                    ) : (
                                        <NoteBubble n={e.n} />
                                    )}
                                </MessageWithDay>
                            );
                        })}
                    </ol>
                </div>
                {unseen && (
                    <Button
                        size="sm"
                        className="absolute right-4 bottom-4 rounded-full shadow-md"
                        onClick={() => toBottom(true)}
                    >
                        <ArrowDown /> {t('inbox.thread.newMessages')}
                    </Button>
                )}
            </div>
            <div className="space-y-2 border-t bg-background p-3">
                {draft && <DraftCard c={c} draft={draft} canReply={canReply} />}
                {canReply ? (
                    <ReplyBox
                        c={c}
                        templates={data.templates}
                        contact={sampleOf(data)}
                    />
                ) : (
                    <p className="text-sm text-muted-foreground">
                        {t('inbox.thread.cannotReply')}
                    </p>
                )}
            </div>
        </div>
    );
}

function MessageWithDay({
    day,
    children,
}: {
    day: string | null;
    children: React.ReactNode;
}) {
    return (
        <>
            {day && (
                <li className="flex justify-center py-1" aria-hidden>
                    <span className="rounded-md bg-white/90 px-2.5 py-0.5 text-[11px] font-medium text-[#54656f] shadow-sm dark:bg-[#182229] dark:text-[#8696a0]">
                        {day}
                    </span>
                </li>
            )}
            {children}
        </>
    );
}

function sampleOf(data: ThreadData): SampleContact {
    const c = data.contact;
    const name =
        data.conversation.contact.name ||
        data.conversation.contact.profile_name;
    return c
        ? { ...c, name: c.name || c.profile_name }
        : { name, phone: data.conversation.contact.phone };
}

function ConversationActions({
    c,
    me,
    users,
}: {
    c: Conversation;
    me: string;
    users: UserOption[];
}) {
    const t = useT();
    const [pending, startTransition] = useTransition();
    const options = new Map<string, string>([
        ['none', t('inbox.filters.unassigned')],
        [me, t('inbox.thread.me')],
    ]);
    for (const u of users) if (!options.has(u.id)) options.set(u.id, u.name);
    if (c.assignee && !options.has(c.assignee.id))
        options.set(c.assignee.id, c.assignee.name);
    return (
        <div className="flex items-center gap-2">
            <Select
                value={c.assignee?.id ?? 'none'}
                disabled={pending}
                onValueChange={(v) =>
                    startTransition(async () => {
                        notify(
                            await assignAction(c.id, v === 'none' ? null : v),
                        );
                    })
                }
            >
                <SelectTrigger
                    size="sm"
                    className="w-36"
                    aria-label={t('inbox.filters.assignee')}
                >
                    <SelectValue />
                </SelectTrigger>
                <SelectContent>
                    {[...options].map(([id, name]) => (
                        <SelectItem key={id} value={id}>
                            {name}
                        </SelectItem>
                    ))}
                </SelectContent>
            </Select>
            <Button
                variant="outline"
                size="sm"
                disabled={pending}
                onClick={() =>
                    startTransition(async () => {
                        notify(
                            await setStatusAction(
                                c.id,
                                c.status === 'open' ? 'close' : 'reopen',
                            ),
                        );
                    })
                }
            >
                {c.status === 'open'
                    ? t('inbox.thread.close')
                    : t('inbox.thread.reopen')}
            </Button>
        </div>
    );
}

function ReplyBox({
    c,
    templates,
    contact,
}: {
    c: Conversation;
    templates: Template[];
    contact: SampleContact;
}) {
    const t = useT();
    const [mode, setMode] = useState(c.window_open ? 'text' : 'template');
    const [text, setText] = useState('');
    const [pending, startTransition] = useTransition();
    const key = useRef(newIdempotencyKey());
    const [attachOpen, setAttachOpen] = useState(false);
    const [file, setFile] = useState<File | null>(null);
    const [uploading, setUploading] = useState(false);

    const send = (
        reply: Parameters<typeof replyAction>[1],
        after?: () => void,
    ) =>
        startTransition(async () => {
            const r = await replyAction(c.id, reply, key.current);
            if (notify(r)) {
                key.current = newIdempotencyKey();
                setText('');
                setFile(null);
                after?.();
            }
        });
    const sendText = async () => {
        if (file) {
            setUploading(true);
            try {
                const q = new URLSearchParams({
                    filename: file.name,
                    phone_number_id: c.phone_number.id,
                });
                const res = await fetch(`/bff/media-upload?${q}`, {
                    method: 'POST',
                    headers: {
                        'Content-Type': file.type || 'application/octet-stream',
                    },
                    body: file,
                });
                const out = await res.json().catch(() => null);
                if (!res.ok || !out?.data?.id) {
                    notify({
                        ok: false,
                        code: out?.error?.code ?? 'UPLOAD_FAILED',
                        message:
                            out?.error?.message ??
                            t('inbox.reply.uploadFailed'),
                    });
                    return;
                }
                send({
                    type: mediaKind(file.type),
                    id: out.data.id,
                    filename: file.name,
                    caption: text.trim() || undefined,
                });
            } finally {
                setUploading(false);
            }
            return;
        }
        if (text.trim()) send({ type: 'text', body: text });
    };

    return (
        <Tabs value={mode} onValueChange={setMode}>
            <TabsList>
                <TabsTrigger value="text">
                    <MessageSquare /> {t('inbox.reply.message')}
                </TabsTrigger>
                <TabsTrigger value="template">
                    <LayoutTemplate /> {t('inbox.reply.template')}
                </TabsTrigger>
                <TabsTrigger value="note" data-testid="composer-note">
                    <StickyNote /> {t('inbox.ai.composer.note')}
                </TabsTrigger>
            </TabsList>
            <TabsContent value="note">
                <NoteComposer c={c} />
            </TabsContent>
            <TabsContent value="text" className="space-y-2">
                {!c.window_open ? (
                    <Alert data-testid="window-closed">
                        <Clock />
                        <AlertTitle>{t('inbox.reply.outsideTitle')}</AlertTitle>
                        <AlertDescription>
                            {t('inbox.reply.outsideBody')}
                        </AlertDescription>
                    </Alert>
                ) : (
                    <form
                        className="flex items-end gap-2"
                        onSubmit={(e) => {
                            e.preventDefault();
                            sendText();
                        }}
                    >
                        <Label htmlFor="reply" className="sr-only">
                            {t('inbox.reply.label')}
                        </Label>
                        <Popover open={attachOpen} onOpenChange={setAttachOpen}>
                            <PopoverTrigger asChild>
                                <Button
                                    type="button"
                                    variant="ghost"
                                    size="icon"
                                    aria-label={t('inbox.reply.attach')}
                                >
                                    <Paperclip />
                                </Button>
                            </PopoverTrigger>
                            <PopoverContent align="start" className="w-72 p-2">
                                <FileDrop
                                    compact
                                    accept={ATTACH_ACCEPT}
                                    file={file}
                                    onFile={(f) => {
                                        setFile(f);
                                        if (f) setAttachOpen(false);
                                    }}
                                />
                            </PopoverContent>
                        </Popover>
                        <div className="flex-1 space-y-1">
                            {file && (
                                <div className="flex items-center gap-2 text-sm">
                                    <Badge
                                        variant="secondary"
                                        className="max-w-full truncate"
                                    >
                                        <FileText /> {file.name}
                                    </Badge>
                                    <Button
                                        type="button"
                                        variant="ghost"
                                        size="sm"
                                        aria-label={t('common.remove')}
                                        onClick={() => setFile(null)}
                                    >
                                        <X />
                                    </Button>
                                </div>
                            )}
                            <Textarea
                                id="reply"
                                value={text}
                                maxLength={file ? 1024 : 4096}
                                rows={2}
                                className="max-h-48 min-h-10 resize-none"
                                placeholder={
                                    file
                                        ? t('inbox.reply.captionPlaceholder')
                                        : t('inbox.reply.placeholder')
                                }
                                onChange={(e) => setText(e.target.value)}
                                onKeyDown={(e) => {
                                    if (
                                        e.key === 'Enter' &&
                                        (e.ctrlKey || e.metaKey)
                                    ) {
                                        e.preventDefault();
                                        sendText();
                                    }
                                }}
                            />
                        </div>
                        <Button
                            type="submit"
                            size="icon"
                            aria-label={t('inbox.reply.send')}
                            disabled={
                                pending || uploading || (!text.trim() && !file)
                            }
                        >
                            {pending || uploading ? <Spinner /> : <Send />}
                        </Button>
                    </form>
                )}
            </TabsContent>
            <TabsContent value="template">
                <TemplateComposer
                    c={c}
                    templates={templates}
                    contact={contact}
                    pending={pending}
                    onSend={(reply, done) => send(reply, done)}
                />
            </TabsContent>
        </Tabs>
    );
}

/** Pick a template, fill its values for this customer, see it, send it. */
function TemplateComposer({
    c,
    templates,
    contact,
    pending,
    onSend,
}: {
    c: Conversation;
    templates: Template[];
    contact: SampleContact;
    pending: boolean;
    onSend: (
        reply: Parameters<typeof replyAction>[1],
        done: () => void,
    ) => void;
}) {
    const t = useT();
    const [chosen, setChosen] = useState('');
    const [values, setValues] = useState<ParamValues>(emptyValues);
    const [tried, setTried] = useState(false);
    const template = templates.find((x) => templateKey(x) === chosen);
    const shape = useMemo(
        () => (template ? shapeOf(template) : null),
        [template],
    );
    const attributeKeys = Object.keys(contact.attributes ?? {});

    if (c.contact.opted_out) {
        return (
            <Alert variant="destructive">
                <AlertCircle />
                <AlertDescription>{t('inbox.reply.optedOut')}</AlertDescription>
            </Alert>
        );
    }
    if (templates.length === 0) {
        return (
            <p className="text-sm text-muted-foreground">
                {t('inbox.reply.noTemplates')}
            </p>
        );
    }
    const issues = shape ? issuesOf(values, shape) : [];
    return (
        <div className="space-y-3">
            <div className="flex items-center gap-2">
                <Combobox
                    className="flex-1"
                    aria-label={t('inbox.reply.template')}
                    value={chosen}
                    onValueChange={(v) => {
                        setChosen(v);
                        setValues(emptyValues());
                        setTried(false);
                    }}
                    placeholder={t('inbox.reply.chooseTemplate')}
                    searchPlaceholder={t('campaigns.wizard.searchTemplates')}
                    emptyText={t('campaigns.wizard.noTemplateMatch')}
                    options={templates.map((x) => ({
                        value: templateKey(x),
                        label: `${x.name} (${x.language})`,
                        keywords: `${x.name} ${x.language} ${x.category}`,
                    }))}
                />
                {template && (
                    <Button
                        size="icon"
                        variant="ghost"
                        aria-label={t('common.close')}
                        onClick={() => {
                            setChosen('');
                            setValues(emptyValues());
                            setTried(false);
                        }}
                    >
                        <X />
                    </Button>
                )}
            </div>
            {template && shape && (
                <div className="@container space-y-2">
                    {/* Kept short so the conversation stays readable above it. */}
                    <div className="grid max-h-[34svh] gap-4 overflow-y-auto pr-1 @2xl:grid-cols-[1fr_17rem]">
                        <div className="space-y-3">
                            <TemplateParamsEditor
                                shape={shape}
                                value={values}
                                onChange={setValues}
                                attributeKeys={attributeKeys}
                                phoneNumberId={c.phone_number.id}
                            />
                            {tried && (
                                <ParamIssues shape={shape} value={values} />
                            )}
                        </div>
                        <WhatsAppPreview
                            className="@2xl:self-start"
                            components={template.components}
                            params={values}
                            contact={contact}
                            mediaSrc={
                                values.mediaPreview ??
                                values.media?.link ??
                                shape.exampleMedia
                            }
                            mediaName={values.media?.filename}
                        />
                    </div>
                    <div className="flex justify-end">
                        <Button
                            disabled={pending}
                            onClick={() => {
                                setTried(true);
                                if (issues.length > 0) return;
                                onSend(
                                    {
                                        type: 'template',
                                        name: template.name,
                                        language: template.language,
                                        params: paramsFromValues(values, shape),
                                    },
                                    () => {
                                        setChosen('');
                                        setValues(emptyValues());
                                        setTried(false);
                                    },
                                );
                            }}
                        >
                            {pending ? <Spinner /> : <Send />}{' '}
                            {t('inbox.reply.sendTemplate')}
                        </Button>
                    </div>
                </div>
            )}
        </div>
    );
}

// ── Customer panel ─────────────────────────────────────────────────────────

function ContactPanel({
    data,
    canNote,
}: {
    data: ThreadData;
    canNote: boolean;
}) {
    const t = useT();
    const [tab, setTab] = useState('customer');
    return (
        <aside
            className="hidden min-h-0 overflow-y-auto p-4 xl:block"
            aria-label={t('inbox.panel.customer')}
        >
            <Tabs value={tab} onValueChange={setTab}>
                <TabsList className="mb-3 w-full">
                    <TabsTrigger value="customer">
                        {t('inbox.ai.panel.customer')}
                    </TabsTrigger>
                    <TabsTrigger value="activity" data-testid="tab-activity">
                        {t('inbox.ai.panel.activity')}
                    </TabsTrigger>
                </TabsList>
                <TabsContent value="customer">
                    <CustomerDetails data={data} canNote={canNote} />
                </TabsContent>
                <TabsContent value="activity" className="space-y-5">
                    <ActivityTimeline
                        key={data.conversation.id}
                        c={data.conversation}
                        items={data.activity}
                        cursor={data.activityCursor}
                    />
                    <AutomationRuns runs={data.runs} />
                </TabsContent>
            </Tabs>
        </aside>
    );
}

function CustomerDetails({
    data,
    canNote,
}: {
    data: ThreadData;
    canNote: boolean;
}) {
    const t = useT();
    const formatDateTime = useFormatDateTime();
    const { contact, notes, events } = data;
    const [note, setNote] = useState('');
    const [pending, startTransition] = useTransition();
    const name = displayName(data.conversation.contact);
    return (
        <div className="space-y-5">
            <div className="flex flex-col items-center gap-2 text-center">
                <ContactAvatar
                    name={name}
                    id={data.conversation.contact.id}
                    className="size-16 text-base"
                />
                <div>
                    <p className="font-semibold">{name}</p>
                    <p className="text-sm text-muted-foreground">
                        {data.conversation.contact.phone}
                    </p>
                    {contact?.email && (
                        <p className="text-sm text-muted-foreground">
                            {contact.email}
                        </p>
                    )}
                </div>
                {data.conversation.contact.opted_out ? (
                    <Badge variant="destructive">
                        {t('inbox.thread.optedOut')}
                    </Badge>
                ) : (
                    <Badge variant="secondary">
                        {t('inbox.panel.subscribed')}
                    </Badge>
                )}
                {contact && (
                    <Button asChild size="sm" variant="outline">
                        <Link href={`/contacts/${contact.id}`}>
                            <ExternalLink /> {t('inbox.panel.openContact')}
                        </Link>
                    </Button>
                )}
            </div>
            <ContactAISwitch c={data.conversation} canEdit={canNote} />
            {contact && contact.tags.length > 0 && (
                <section className="space-y-1.5">
                    <h3 className="text-xs font-medium tracking-wide text-muted-foreground uppercase">
                        {t('inbox.panel.tags')}
                    </h3>
                    <div className="flex flex-wrap gap-1">
                        {contact.tags.map((tag) => (
                            <Badge key={tag.id} variant="outline">
                                {tag.name}
                            </Badge>
                        ))}
                    </div>
                </section>
            )}
            {contact && (
                <section className="space-y-2">
                    <h3 className="text-xs font-medium tracking-wide text-muted-foreground uppercase">
                        {t('inbox.panel.notes')}
                    </h3>
                    {canNote && (
                        <form
                            className="space-y-2"
                            onSubmit={(e) => {
                                e.preventDefault();
                                if (!note.trim()) return;
                                startTransition(async () => {
                                    if (
                                        notify(
                                            await addNoteAction(
                                                contact.id,
                                                note.trim(),
                                            ),
                                        )
                                    )
                                        setNote('');
                                });
                            }}
                        >
                            <Label htmlFor="note" className="sr-only">
                                {t('inbox.panel.newNote')}
                            </Label>
                            <Textarea
                                id="note"
                                rows={2}
                                maxLength={5000}
                                value={note}
                                placeholder={t('inbox.panel.notePlaceholder')}
                                onChange={(e) => setNote(e.target.value)}
                            />
                            <Button
                                size="sm"
                                type="submit"
                                disabled={pending || !note.trim()}
                            >
                                {t('inbox.panel.addNote')}
                            </Button>
                        </form>
                    )}
                    {notes.length === 0 ? (
                        <p className="text-sm text-muted-foreground">
                            {t('inbox.panel.noNotes')}
                        </p>
                    ) : (
                        <ul className="space-y-2 text-sm">
                            {notes.slice(0, 5).map((n) => (
                                <li
                                    key={n.id}
                                    className="rounded-md bg-amber-500/10 p-2"
                                >
                                    <p className="whitespace-pre-wrap">
                                        {n.body}
                                    </p>
                                    <p className="mt-1 text-xs text-muted-foreground">
                                        {n.author_name} ·{' '}
                                        {formatDateTime(n.created_at)}
                                    </p>
                                </li>
                            ))}
                        </ul>
                    )}
                </section>
            )}
            <section className="space-y-2">
                <h3 className="text-xs font-medium tracking-wide text-muted-foreground uppercase">
                    {t('inbox.panel.history')}
                </h3>
                {events.length === 0 ? (
                    <p className="text-sm text-muted-foreground">
                        {t('inbox.panel.noHistory')}
                    </p>
                ) : (
                    <ol className="space-y-2 border-l pl-3 text-sm">
                        {events.slice(0, 10).map((e) => (
                            <li key={e.id}>
                                {t(`inbox.panel.events.${e.kind}`)}
                                {e.assignee
                                    ? t('inbox.panel.to', {
                                          name: e.assignee.name,
                                      })
                                    : ''}
                                <span className="block text-xs text-muted-foreground">
                                    {e.actor_type === 'system'
                                        ? t('inbox.panel.byCustomer')
                                        : t('inbox.panel.by', {
                                              name:
                                                  e.actor_name ??
                                                  t('inbox.panel.apiKey'),
                                          })}{' '}
                                    · {formatDateTime(e.created_at)}
                                </span>
                            </li>
                        ))}
                    </ol>
                )}
            </section>
        </div>
    );
}

/** File types WhatsApp accepts as media messages. */
const ATTACH_ACCEPT =
    'image/jpeg,image/png,video/mp4,video/3gpp,audio/aac,audio/mpeg,audio/mp4,audio/ogg,application/pdf,text/plain,.doc,.docx,.xls,.xlsx,.ppt,.pptx';

/** The WhatsApp message type for an attached file. */
function mediaKind(mime: string): 'image' | 'video' | 'audio' | 'document' {
    if (mime.startsWith('image/')) return 'image';
    if (mime.startsWith('video/')) return 'video';
    if (mime.startsWith('audio/')) return 'audio';
    return 'document';
}
