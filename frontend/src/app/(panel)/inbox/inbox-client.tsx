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
    ['open', 'Open'],
    ['closed', 'Closed'],
    ['all', 'All'],
] as const;
const ASSIGNEES = [
    ['', 'Everyone'],
    ['me', 'Mine'],
    ['unassigned', 'Unassigned'],
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
                    Search conversations
                </Label>
                <Input
                    id="inbox-search"
                    name="q"
                    defaultValue={q}
                    maxLength={100}
                    placeholder="Name or phone"
                />
                <Button
                    type="submit"
                    variant="outline"
                    size="icon"
                    aria-label="Search"
                >
                    <Search />
                </Button>
            </form>
            <nav aria-label="Status" className="flex flex-wrap gap-1">
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
                        {label}
                    </Link>
                ))}
            </nav>
            <nav aria-label="Assignee" className="flex flex-wrap gap-1">
                {ASSIGNEES.map(([v, label]) => (
                    <Link
                        key={v || 'all'}
                        href={inboxHref(query, { assignee: v, c: '' })}
                        aria-current={assignee === v ? 'page' : undefined}
                        className={pill(assignee === v)}
                    >
                        {label}
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
                    <EmptyTitle>No conversations</EmptyTitle>
                    <EmptyDescription>
                        Messages from your customers appear here.
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
                                              ? 'You: '
                                              : '') +
                                          (c.last_message.body ||
                                              `[${c.last_message.type}]`)
                                        : '—'}
                                </span>
                                <span className="flex shrink-0 items-center gap-1">
                                    {c.status === 'closed' && (
                                        <Badge variant="outline">Closed</Badge>
                                    )}
                                    {c.unread_count > 0 && (
                                        <Badge
                                            aria-label={`${c.unread_count} unread`}
                                            data-testid="unread-count"
                                        >
                                            {c.unread_count}
                                        </Badge>
                                    )}
                                </span>
                            </span>
                            {c.assignee && (
                                <span className="text-xs text-muted-foreground">
                                    Assigned to {c.assignee.name}
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
                    {pending && <Spinner />} Load more
                </Button>
            )}
        </div>
    );
}

// ── Thread ─────────────────────────────────────────────────────────────────

const STATUS_LABEL: Record<InboxMessage['status'], string> = {
    pending: 'Queued',
    sending: 'Sending',
    unknown: 'Delivery unknown',
    sent: 'Sent',
    delivered: 'Delivered',
    read: 'Read',
    failed: 'Failed',
    received: 'Received',
};

function Ticks({ status }: { status: InboxMessage['status'] }) {
    const label = STATUS_LABEL[status];
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
    const md = m.media;
    if (!md) return null;
    if (md.status !== 'stored') {
        return (
            <p className="text-xs italic opacity-80">
                {md.status === 'failed'
                    ? 'The file could not be downloaded from WhatsApp.'
                    : 'Downloading the file…'}
            </p>
        );
    }
    const src = `/bff/media/${conversationId}/${md.id}`;
    const alt = m.body || `${m.type} from the customer`;
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
        return <audio src={src} controls aria-label="Voice message" />;
    }
    return (
        <a
            href={src}
            className="flex items-center gap-2 underline underline-offset-2"
            download={md.filename || undefined}
        >
            <FileText className="size-4" /> {md.filename || 'Download file'}
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
    if (!selected || !data) {
        return (
            <Empty className="h-full rounded-lg border py-16">
                <EmptyHeader>
                    <EmptyMedia variant="icon">
                        <MessageSquare />
                    </EmptyMedia>
                    <EmptyTitle>
                        {selected
                            ? 'Conversation not found'
                            : 'Select a conversation'}
                    </EmptyTitle>
                    <EmptyDescription>
                        {selected
                            ? 'It does not exist or is assigned to someone else.'
                            : 'Choose a conversation from the list to read and reply.'}
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
                        {c.contact.phone} · via{' '}
                        {c.phone_number.verified_name ||
                            c.phone_number.display_phone_number}
                    </p>
                    <div className="mt-1 flex flex-wrap gap-1">
                        <Badge
                            variant={
                                c.status === 'open' ? 'secondary' : 'outline'
                            }
                        >
                            {c.status === 'open' ? 'Open' : 'Closed'}
                        </Badge>
                        {c.window_open && c.window_expires_at ? (
                            <Badge
                                variant="secondary"
                                title="Free-form replies are possible until then"
                            >
                                24h window until{' '}
                                {formatDateTime(c.window_expires_at)}
                            </Badge>
                        ) : (
                            <Badge variant="outline">24h window closed</Badge>
                        )}
                        {c.contact.opted_out && (
                            <Badge variant="destructive">Opted out</Badge>
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
                aria-label="Messages"
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
                            {pending && <Spinner />} Load older messages
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
                        Your role cannot reply to conversations.
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
    const [pending, startTransition] = useTransition();
    const options = new Map<string, string>([
        ['none', 'Unassigned'],
        [me, 'Me'],
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
                <SelectTrigger className="w-40" aria-label="Assignee">
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
                {c.status === 'open' ? 'Close' : 'Reopen'}
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
    const [mode, setMode] = useState(c.window_open ? 'text' : 'template');
    const [text, setText] = useState('');
    const [template, setTemplate] = useState('');
    const [pending, startTransition] = useTransition();
    const key = useRef(newIdempotencyKey());
    const fileInput = useRef<HTMLInputElement>(null);
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
                            'The file could not be uploaded.',
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
                <TabsTrigger value="text">Message</TabsTrigger>
                <TabsTrigger value="template">Template</TabsTrigger>
            </TabsList>
            <TabsContent value="text" className="space-y-2">
                {!c.window_open ? (
                    <Alert data-testid="window-closed">
                        <Clock />
                        <AlertTitle>Outside the 24-hour window</AlertTitle>
                        <AlertDescription>
                            The customer has not written in the last 24 hours.
                            Send an approved template instead; free-form
                            messages are possible again after they reply.
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
                            Reply
                        </Label>
                        <input
                            ref={fileInput}
                            type="file"
                            className="sr-only"
                            aria-label="Attach a file"
                            accept="image/jpeg,image/png,video/mp4,video/3gpp,audio/aac,audio/mpeg,audio/mp4,audio/ogg,application/pdf,text/plain,.doc,.docx,.xls,.xlsx,.ppt,.pptx"
                            onChange={(e) =>
                                setFile(e.target.files?.[0] ?? null)
                            }
                        />
                        <Button
                            type="button"
                            variant="outline"
                            size="icon"
                            aria-label="Attach a file"
                            onClick={() => fileInput.current?.click()}
                        >
                            <Paperclip />
                        </Button>
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
                                        onClick={() => {
                                            setFile(null);
                                            if (fileInput.current)
                                                fileInput.current.value = '';
                                        }}
                                    >
                                        Remove
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
                                        ? 'Add a caption (optional)…'
                                        : 'Write a reply… (Ctrl+Enter to send)'
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
                            {(pending || uploading) && <Spinner />} Send
                        </Button>
                    </form>
                )}
            </TabsContent>
            <TabsContent value="template" className="space-y-2">
                {c.contact.opted_out && (
                    <Alert variant="destructive">
                        <AlertCircle />
                        <AlertDescription>
                            This contact opted out; templates cannot be sent.
                        </AlertDescription>
                    </Alert>
                )}
                {templates.length === 0 ? (
                    <p className="text-sm text-muted-foreground">
                        No approved templates. Sync templates in the channel
                        settings.
                    </p>
                ) : (
                    <div className="flex items-center gap-2">
                        <Select value={template} onValueChange={setTemplate}>
                            <SelectTrigger
                                className="w-64"
                                aria-label="Template"
                            >
                                <SelectValue placeholder="Choose a template" />
                            </SelectTrigger>
                            <SelectContent>
                                {templates.map((t) => (
                                    <SelectItem
                                        key={t.id}
                                        value={`${t.name}|${t.language}`}
                                    >
                                        {t.name} ({t.language})
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
                            {pending && <Spinner />} Send template
                        </Button>
                    </div>
                )}
            </TabsContent>
        </Tabs>
    );
}

// ── Customer panel ─────────────────────────────────────────────────────────

const EVENT_LABEL = {
    assigned: 'Assigned',
    unassigned: 'Unassigned',
    closed: 'Closed',
    reopened: 'Reopened',
} as const;

function ContactPanel({
    data,
    canNote,
}: {
    data: ThreadData;
    canNote: boolean;
}) {
    const formatDateTime = useFormatDateTime();
    const { contact, notes, events } = data;
    const [note, setNote] = useState('');
    const [pending, startTransition] = useTransition();
    return (
        <aside className="space-y-4" aria-label="Customer">
            <Card className="gap-3 py-4">
                <CardHeader className="px-4">
                    <CardTitle className="text-sm">Customer</CardTitle>
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
                        Consent:{' '}
                        {data.conversation.contact.opted_out ? (
                            <Badge variant="destructive">Opted out</Badge>
                        ) : (
                            <Badge variant="secondary">Subscribed</Badge>
                        )}
                    </p>
                    {contact && contact.tags.length > 0 && (
                        <div className="flex flex-wrap gap-1" aria-label="Tags">
                            {contact.tags.map((t) => (
                                <Badge key={t.id} variant="outline">
                                    {t.name}
                                </Badge>
                            ))}
                        </div>
                    )}
                </CardContent>
            </Card>
            {contact && (
                <Card className="gap-3 py-4">
                    <CardHeader className="px-4">
                        <CardTitle className="text-sm">Notes</CardTitle>
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
                                    New note
                                </Label>
                                <Textarea
                                    id="note"
                                    rows={2}
                                    maxLength={5000}
                                    value={note}
                                    placeholder="Internal note"
                                    onChange={(e) => setNote(e.target.value)}
                                />
                                <Button
                                    size="sm"
                                    type="submit"
                                    disabled={pending || !note.trim()}
                                >
                                    Add note
                                </Button>
                            </form>
                        )}
                        {notes.length === 0 ? (
                            <p className="text-muted-foreground">No notes.</p>
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
                    <CardTitle className="text-sm">History</CardTitle>
                </CardHeader>
                <CardContent className="px-4 text-sm">
                    {events.length === 0 ? (
                        <p className="text-muted-foreground">No changes yet.</p>
                    ) : (
                        <ul className="space-y-1.5">
                            {events.slice(0, 10).map((e) => (
                                <li key={e.id}>
                                    {EVENT_LABEL[e.kind]}
                                    {e.assignee ? ` to ${e.assignee.name}` : ''}
                                    <span className="block text-xs text-muted-foreground">
                                        {e.actor_type === 'system'
                                            ? 'by a customer message'
                                            : `by ${e.actor_name ?? 'an API key'}`}{' '}
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

/** The WhatsApp message type for an attached file. */
function mediaKind(mime: string): 'image' | 'video' | 'audio' | 'document' {
    if (mime.startsWith('image/')) return 'image';
    if (mime.startsWith('video/')) return 'video';
    if (mime.startsWith('audio/')) return 'audio';
    return 'document';
}
