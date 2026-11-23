'use client';

import Link from 'next/link';
import { useEffect, useRef, useState, useTransition } from 'react';
import {
    AlertCircle,
    Check,
    CheckCheck,
    Clock,
    FileText,
    HelpCircle,
    Inbox as InboxIcon,
    MessageSquare,
    Search,
    Paperclip,
} from 'lucide-react';
import { newIdempotencyKey, notify } from '@/components/notify';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import {
    Empty,
    EmptyDescription,
    EmptyHeader,
    EmptyMedia,
    EmptyTitle,
} from '@/components/ui/empty';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
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
    useFormatTimeLabel,
    useFormatDateTime,
} from '@/components/display-preferences';
import { FileDrop } from '@/components/ui/file-drop';
import {
    Popover,
    PopoverContent,
    PopoverTrigger,
} from '@/components/ui/popover';
import { useT } from '@/i18n/client';
import type { MessageKey, Translate } from '@/i18n/translate';
import { cn } from '@/lib/utils';
import type { Conversation, InboxMessage, Template } from '@/lib/types';
import type { ThreadData } from './page';
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

export function InboxFilters({
    status,
    assignee,
    q,
}: {
    status: string;
    assignee: string;
    q: string;
}) {
    const t = useT();
    const base = new URLSearchParams();
    if (status !== 'open') base.set('status', status);
    if (assignee) base.set('assignee', assignee);
    if (q) base.set('q', q);
    const query = base.toString();
    const pill = (active: boolean) =>
        cn(
            'rounded-md px-2.5 py-1 text-sm outline-none focus-visible:ring-2 focus-visible:ring-ring',
            active
                ? 'bg-primary text-primary-foreground'
                : 'text-muted-foreground hover:bg-muted',
        );
    return (
        <div className="space-y-2">
            <form
                action="/inbox"
                method="get"
                role="search"
                className="flex gap-2"
            >
                {status !== 'open' && (
                    <input type="hidden" name="status" value={status} />
                )}
                {assignee && (
                    <input type="hidden" name="assignee" value={assignee} />
                )}
                <Label htmlFor="inbox-search" className="sr-only">
                    {t('inbox.filters.searchLabel')}
                </Label>
                <Input
                    id="inbox-search"
                    name="q"
                    defaultValue={q}
                    maxLength={100}
                    data-search
                    placeholder={t('inbox.filters.searchPlaceholder')}
                />
                <Button
                    type="submit"
                    variant="outline"
                    size="icon"
                    aria-label={t('common.search')}
                >
                    <Search />
                </Button>
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

    if (items.length === 0) {
        return (
            <Empty className="rounded-lg border py-12">
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
        <div className="flex flex-col gap-2">
            <ul
                className="divide-y rounded-lg border"
                data-testid="conversation-list"
            >
                {items.map((c) => (
                    <li key={c.id}>
                        <Link
                            href={inboxHref(query, { c: c.id })}
                            aria-current={
                                c.id === selected ? 'true' : undefined
                            }
                            data-testid="conversation-item"
                            className={cn(
                                'flex flex-col gap-1 px-3 py-2.5 outline-none hover:bg-muted/60 focus-visible:bg-muted focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-inset',
                                c.id === selected && 'bg-muted',
                            )}
                        >
                            <span className="flex items-center justify-between gap-2">
                                <span
                                    className={cn(
                                        'truncate text-sm',
                                        c.unread_count > 0 && 'font-semibold',
                                    )}
                                >
                                    {displayName(c.contact)}
                                </span>
                                <time
                                    dateTime={c.last_message_at}
                                    className="shrink-0 text-xs text-muted-foreground tabular-nums"
                                >
                                    {timeLabel(c.last_message_at)}
                                </time>
                            </span>
                            <span className="flex items-center justify-between gap-2">
                                <span className="truncate text-xs text-muted-foreground">
                                    {c.last_message
                                        ? (c.last_message.direction ===
                                          'outbound'
                                              ? t('inbox.list.you')
                                              : '') +
                                          (c.last_message.body ||
                                              `[${c.last_message.type}]`)
                                        : '—'}
                                </span>
                                <span className="flex shrink-0 items-center gap-1">
                                    {c.status === 'closed' && (
                                        <Badge variant="outline">
                                            {t('inbox.filters.closed')}
                                        </Badge>
                                    )}
                                    {c.unread_count > 0 && (
                                        <Badge
                                            aria-label={t('inbox.list.unread', {
                                                count: c.unread_count,
                                            })}
                                            data-testid="unread-count"
                                        >
                                            {c.unread_count}
                                        </Badge>
                                    )}
                                </span>
                            </span>
                            {c.assignee && (
                                <span className="text-xs text-muted-foreground">
                                    {t('inbox.list.assignedTo', {
                                        name: c.assignee.name,
                                    })}
                                </span>
                            )}
                        </Link>
                    </li>
                ))}
            </ul>
            {next && (
                <Button
                    variant="outline"
                    disabled={pending}
                    onClick={() =>
                        startTransition(async () => {
                            const r = await moreConversationsAction(
                                query,
                                next,
                            );
                            if (notify(r)) {
                                setMore((m) => [...m, ...r.data.items]);
                                setCursor(r.data.next);
                                setLoaded(true);
                            }
                        })
                    }
                >
                    {pending && <Spinner />} {t('inbox.list.loadMore')}
                </Button>
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
    if (m.type === 'video') {
        return <video src={src} controls className="max-h-72 rounded-md" />;
    }
    if (m.type === 'audio') {
        return <audio src={src} controls aria-label={t('inbox.media.voice')} />;
    }
    return (
        <a
            href={src}
            className="flex items-center gap-2 underline underline-offset-2"
            download={md.filename || undefined}
        >
            <FileText className="size-4" />{' '}
            {md.filename || t('inbox.media.download')}
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
    const formatDateTime = useFormatDateTime();
    const out = m.direction === 'outbound';
    return (
        <li
            className={cn('flex', out ? 'justify-end' : 'justify-start')}
            data-testid="message"
            data-direction={m.direction}
            data-status={m.status}
        >
            <div
                className={cn(
                    'max-w-[80%] space-y-1 rounded-lg px-3 py-2 text-sm',
                    out ? 'bg-primary text-primary-foreground' : 'bg-muted',
                )}
            >
                <MediaView conversationId={conversationId} m={m} />
                {m.body && (
                    <p className="break-words whitespace-pre-wrap">{m.body}</p>
                )}
                {!m.body && !m.media && (
                    <p className="italic opacity-80">[{m.type}]</p>
                )}
                {m.status === 'failed' && m.error_title && (
                    <p className="text-xs opacity-90">{m.error_title}</p>
                )}
                <p className="flex items-center justify-end gap-1 text-[11px] opacity-75">
                    <time dateTime={m.created_at}>
                        {formatDateTime(
                            m.direction === 'inbound'
                                ? m.occurred_at
                                : m.created_at,
                        )}
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
}: {
    selected?: string;
    data: ThreadData | null;
    me: string;
    users: UserOption[];
    canReply: boolean;
    canAssign: boolean;
    canNote: boolean;
}) {
    const t = useT();
    if (!selected || !data) {
        return (
            <Empty className="h-full rounded-lg border py-16">
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
        <div className="grid gap-4 xl:grid-cols-[1fr_16rem]">
            <ThreadMain
                data={data}
                me={me}
                users={users}
                canReply={canReply}
                canAssign={canAssign}
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
}: {
    data: ThreadData;
    me: string;
    users: UserOption[];
    canReply: boolean;
    canAssign: boolean;
}) {
    const t = useT();
    const formatDateTime = useFormatDateTime();
    const c = data.conversation;
    const [older, setOlder] = useState<InboxMessage[]>([]);
    const [olderCursor, setOlderCursor] = useState<string | undefined>();
    const [loadedOlder, setLoadedOlder] = useState(false);
    const [pending, startTransition] = useTransition();
    const scroller = useRef<HTMLDivElement>(null);

    // The API pages newest first; the thread reads oldest first.
    const recentIds = new Set(data.messages.map((m) => m.id));
    const messages = [
        ...older.filter((m) => !recentIds.has(m.id)).reverse(),
        ...[...data.messages].reverse(),
    ];
    const next = loadedOlder ? olderCursor : data.nextCursor;
    const newest = data.messages[0]?.id;

    // Opening a conversation (or a new customer message while it is open)
    // marks it read for the team.
    useEffect(() => {
        if (c.unread_count > 0) void markReadAction(c.id);
    }, [c.id, c.unread_count]);

    useEffect(() => {
        const el = scroller.current;
        if (el) el.scrollTop = el.scrollHeight;
    }, [newest]);

    return (
        <Card className="flex min-h-[70vh] flex-col gap-0 py-0">
            <CardHeader className="flex flex-row flex-wrap items-start justify-between gap-3 border-b py-3">
                <div className="min-w-0">
                    <CardTitle
                        className="truncate text-base"
                        data-testid="thread-title"
                    >
                        {displayName(c.contact)}
                    </CardTitle>
                    <p className="text-xs text-muted-foreground">
                        {t('inbox.thread.via', {
                            phone: c.contact.phone,
                            channel:
                                c.phone_number.verified_name ||
                                c.phone_number.display_phone_number,
                        })}
                    </p>
                    <div className="mt-1 flex flex-wrap gap-1">
                        <Badge
                            variant={
                                c.status === 'open' ? 'secondary' : 'outline'
                            }
                        >
                            {c.status === 'open'
                                ? t('inbox.filters.open')
                                : t('inbox.filters.closed')}
                        </Badge>
                        {c.window_open && c.window_expires_at ? (
                            <Badge
                                variant="secondary"
                                title={t('inbox.thread.windowHint')}
                            >
                                {t('inbox.thread.windowUntil', {
                                    time: formatDateTime(c.window_expires_at),
                                })}
                            </Badge>
                        ) : (
                            <Badge variant="outline">
                                {t('inbox.thread.windowClosed')}
                            </Badge>
                        )}
                        {c.contact.opted_out && (
                            <Badge variant="destructive">
                                {t('inbox.thread.optedOut')}
                            </Badge>
                        )}
                    </div>
                </div>
                {canAssign && (
                    <ConversationActions c={c} me={me} users={users} />
                )}
            </CardHeader>
            <div
                ref={scroller}
                className="flex-1 overflow-y-auto p-4"
                style={{ maxHeight: '60vh' }}
                tabIndex={0}
                aria-label={t('inbox.thread.messages')}
            >
                {next && (
                    <div className="mb-3 text-center">
                        <Button
                            size="sm"
                            variant="outline"
                            disabled={pending}
                            onClick={() =>
                                startTransition(async () => {
                                    const r = await olderMessagesAction(
                                        c.id,
                                        next,
                                    );
                                    if (notify(r)) {
                                        setOlder((o) => [
                                            ...o,
                                            ...r.data.items,
                                        ]);
                                        setOlderCursor(r.data.next);
                                        setLoadedOlder(true);
                                    }
                                })
                            }
                        >
                            {pending && <Spinner />}{' '}
                            {t('inbox.thread.loadOlder')}
                        </Button>
                    </div>
                )}
                <ol
                    className="space-y-2"
                    aria-live="polite"
                    data-testid="thread"
                >
                    {messages.map((m) => (
                        <Bubble key={m.id} conversationId={c.id} m={m} />
                    ))}
                </ol>
            </div>
            <div className="border-t p-3">
                {canReply ? (
                    <ReplyBox c={c} templates={data.templates} />
                ) : (
                    <p className="text-sm text-muted-foreground">
                        {t('inbox.thread.cannotReply')}
                    </p>
                )}
            </div>
        </Card>
    );
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
    if (c.assignee && !options.has(c.assignee.id)) {
        options.set(c.assignee.id, c.assignee.name);
    }
    return (
        <div className="flex flex-wrap items-center gap-2">
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
                    className="w-40"
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
}: {
    c: Conversation;
    templates: Template[];
}) {
    const t = useT();
    const [mode, setMode] = useState(c.window_open ? 'text' : 'template');
    const [text, setText] = useState('');
    const [template, setTemplate] = useState('');
    const [pending, startTransition] = useTransition();
    const key = useRef(newIdempotencyKey());
    const [attachOpen, setAttachOpen] = useState(false);
    const [file, setFile] = useState<File | null>(null);
    const [uploading, setUploading] = useState(false);

    const send = (reply: Parameters<typeof replyAction>[1]) =>
        startTransition(async () => {
            const r = await replyAction(c.id, reply, key.current);
            if (notify(r)) {
                key.current = newIdempotencyKey();
                setText('');
                setFile(null);
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
                    {t('inbox.reply.message')}
                </TabsTrigger>
                <TabsTrigger value="template">
                    {t('inbox.reply.template')}
                </TabsTrigger>
            </TabsList>
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
                                    variant="outline"
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
                                        {file.name}
                                    </Badge>
                                    <Button
                                        type="button"
                                        variant="ghost"
                                        size="sm"
                                        onClick={() => setFile(null)}
                                    >
                                        {t('common.remove')}
                                    </Button>
                                </div>
                            )}
                            <Textarea
                                id="reply"
                                value={text}
                                maxLength={file ? 1024 : 4096}
                                rows={2}
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
                            disabled={
                                pending || uploading || (!text.trim() && !file)
                            }
                        >
                            {(pending || uploading) && <Spinner />}{' '}
                            {t('inbox.reply.send')}
                        </Button>
                    </form>
                )}
            </TabsContent>
            <TabsContent value="template" className="space-y-2">
                {c.contact.opted_out && (
                    <Alert variant="destructive">
                        <AlertCircle />
                        <AlertDescription>
                            {t('inbox.reply.optedOut')}
                        </AlertDescription>
                    </Alert>
                )}
                {templates.length === 0 ? (
                    <p className="text-sm text-muted-foreground">
                        {t('inbox.reply.noTemplates')}
                    </p>
                ) : (
                    <div className="flex items-center gap-2">
                        <Select value={template} onValueChange={setTemplate}>
                            <SelectTrigger
                                className="w-64"
                                aria-label={t('inbox.reply.template')}
                            >
                                <SelectValue
                                    placeholder={t(
                                        'inbox.reply.chooseTemplate',
                                    )}
                                />
                            </SelectTrigger>
                            <SelectContent>
                                {templates.map((tpl) => (
                                    <SelectItem
                                        key={tpl.id}
                                        value={`${tpl.name}|${tpl.language}`}
                                    >
                                        {tpl.name} ({tpl.language})
                                    </SelectItem>
                                ))}
                            </SelectContent>
                        </Select>
                        <Button
                            disabled={
                                pending || !template || c.contact.opted_out
                            }
                            onClick={() => {
                                const [name = '', language = ''] =
                                    template.split('|');
                                send({ type: 'template', name, language });
                            }}
                        >
                            {pending && <Spinner />}{' '}
                            {t('inbox.reply.sendTemplate')}
                        </Button>
                    </div>
                )}
            </TabsContent>
        </Tabs>
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
    const formatDateTime = useFormatDateTime();
    const { contact, notes, events } = data;
    const [note, setNote] = useState('');
    const [pending, startTransition] = useTransition();
    return (
        <aside className="space-y-4" aria-label={t('inbox.panel.customer')}>
            <Card className="gap-3 py-4">
                <CardHeader className="px-4">
                    <CardTitle className="text-sm">
                        {t('inbox.panel.customer')}
                    </CardTitle>
                </CardHeader>
                <CardContent className="space-y-2 px-4 text-sm">
                    <p className="font-medium">
                        {displayName(data.conversation.contact)}
                    </p>
                    <p className="text-muted-foreground">
                        {data.conversation.contact.phone}
                    </p>
                    {contact?.email && (
                        <p className="text-muted-foreground">{contact.email}</p>
                    )}
                    <p>
                        {t('inbox.panel.consent')}{' '}
                        {data.conversation.contact.opted_out ? (
                            <Badge variant="destructive">
                                {t('inbox.thread.optedOut')}
                            </Badge>
                        ) : (
                            <Badge variant="secondary">
                                {t('inbox.panel.subscribed')}
                            </Badge>
                        )}
                    </p>
                    {contact && contact.tags.length > 0 && (
                        <div
                            className="flex flex-wrap gap-1"
                            aria-label={t('inbox.panel.tags')}
                        >
                            {contact.tags.map((tag) => (
                                <Badge key={tag.id} variant="outline">
                                    {tag.name}
                                </Badge>
                            ))}
                        </div>
                    )}
                </CardContent>
            </Card>
            {contact && (
                <Card className="gap-3 py-4">
                    <CardHeader className="px-4">
                        <CardTitle className="text-sm">
                            {t('inbox.panel.notes')}
                        </CardTitle>
                    </CardHeader>
                    <CardContent className="space-y-3 px-4 text-sm">
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
                                        ) {
                                            setNote('');
                                        }
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
                                    placeholder={t(
                                        'inbox.panel.notePlaceholder',
                                    )}
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
                            <p className="text-muted-foreground">
                                {t('inbox.panel.noNotes')}
                            </p>
                        ) : (
                            <ul className="space-y-2">
                                {notes.slice(0, 5).map((n) => (
                                    <li key={n.id}>
                                        <p className="whitespace-pre-wrap">
                                            {n.body}
                                        </p>
                                        <p className="text-xs text-muted-foreground">
                                            {n.author_name} ·{' '}
                                            {formatDateTime(n.created_at)}
                                        </p>
                                    </li>
                                ))}
                            </ul>
                        )}
                    </CardContent>
                </Card>
            )}
            <Card className="gap-3 py-4">
                <CardHeader className="px-4">
                    <CardTitle className="text-sm">
                        {t('inbox.panel.history')}
                    </CardTitle>
                </CardHeader>
                <CardContent className="px-4 text-sm">
                    {events.length === 0 ? (
                        <p className="text-muted-foreground">
                            {t('inbox.panel.noHistory')}
                        </p>
                    ) : (
                        <ul className="space-y-1.5">
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
                        </ul>
                    )}
                </CardContent>
            </Card>
        </aside>
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
