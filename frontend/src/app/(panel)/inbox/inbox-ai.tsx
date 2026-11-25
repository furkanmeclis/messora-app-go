'use client';

import { useState, useTransition } from 'react';
import {
    AlertCircle,
    Bot,
    Check,
    Hand,
    Pencil,
    Sparkles,
    StickyNote,
    UserRound,
    Workflow,
    X,
} from 'lucide-react';
import { notify } from '@/components/notify';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Label } from '@/components/ui/label';
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from '@/components/ui/select';
import { Spinner } from '@/components/ui/spinner';
import { Switch } from '@/components/ui/switch';
import { Textarea } from '@/components/ui/textarea';
import {
    useDisplayPrefs,
    useFormatDateTime,
} from '@/components/display-preferences';
import { formatTime } from '@/lib/format';
import { useT } from '@/i18n/client';
import type { MessageKey, Translate } from '@/i18n/translate';
import { cn } from '@/lib/utils';
import type {
    AIDraft,
    Conversation,
    ConversationActivity,
    ConversationAIMode,
    ConversationAutomationRun,
    ConversationHandoff,
    ConversationNote,
    InboxMessage,
} from '@/lib/types';
import {
    addConversationNoteAction,
    decideDraftAction,
    handOffAction,
    olderActivityAction,
    setAIModeAction,
    setContactAIAction,
    takeOverAction,
} from './actions';

const MODES: ConversationAIMode[] = [
    'off',
    'suggest',
    'auto_reply',
    'awaiting_approval',
];

/** A message the AI wrote. */
export function isAIMessage(m: InboxMessage): boolean {
    return (
        m.direction === 'outbound' && (m.source === 'ai' || m.sent_by === 'ai')
    );
}

/** A translated text for a dynamic key, or the fallback. */
function tr(
    t: Translate,
    key: string,
    vars?: Record<string, string | number>,
    fallback = '',
) {
    const text = t(key as MessageKey, vars);
    return text === key ? fallback || key : text;
}

// ── Thread header ──────────────────────────────────────────────────────────

/** Owner badge, AI status, AI mode and the handoff buttons. */
export function OwnerControls({
    c,
    canReply,
}: {
    c: Conversation;
    canReply: boolean;
}) {
    const t = useT();
    const [pending, start] = useTransition();
    const run = (fn: () => Promise<Parameters<typeof notify>[0]>) =>
        start(async () => {
            notify(await fn());
        });
    const ai = c.owner_type === 'ai';
    return (
        <div
            className="flex flex-wrap items-center gap-2"
            data-testid="owner-controls"
        >
            <Badge
                variant={ai ? 'default' : 'secondary'}
                data-testid="owner-badge"
                data-owner={c.owner_type}
                className="gap-1"
            >
                {ai ? <Bot /> : <UserRound />}
                {t(`inbox.ai.owner.${c.owner_type}`)}
            </Badge>
            {c.ai_status === 'thinking' && (
                <span
                    className="inline-flex items-center gap-1 text-xs text-muted-foreground"
                    data-testid="ai-status"
                    data-status="thinking"
                >
                    <Spinner className="size-3" /> {t('inbox.ai.thinking')}
                </span>
            )}
            {c.ai_status === 'error' && (
                <span
                    className="inline-flex items-center gap-1 text-xs text-destructive"
                    data-testid="ai-status"
                    data-status="error"
                >
                    <AlertCircle className="size-3" />
                    {t('inbox.ai.error', { detail: c.ai_status_detail ?? '—' })}
                </span>
            )}
            {canReply && (
                <>
                    <Select
                        value={c.ai_mode}
                        disabled={pending}
                        onValueChange={(v) =>
                            run(() =>
                                setAIModeAction(c.id, v as ConversationAIMode),
                            )
                        }
                    >
                        <SelectTrigger
                            size="sm"
                            className="w-40"
                            aria-label={t('inbox.ai.mode.label')}
                            data-testid="ai-mode"
                        >
                            <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                            {MODES.map((m) => (
                                <SelectItem key={m} value={m}>
                                    {t(`inbox.ai.mode.${m}`)}
                                </SelectItem>
                            ))}
                        </SelectContent>
                    </Select>
                    {ai || c.pending_handoff_id ? (
                        <Button
                            size="sm"
                            disabled={pending}
                            onClick={() => run(() => takeOverAction(c.id))}
                            data-testid="take-over"
                        >
                            <Hand /> {t('inbox.ai.takeOver')}
                        </Button>
                    ) : (
                        <Button
                            size="sm"
                            variant="outline"
                            disabled={pending}
                            onClick={() => run(() => handOffAction(c.id, 'ai'))}
                            data-testid="hand-to-ai"
                        >
                            <Bot /> {t('inbox.ai.handToAI')}
                        </Button>
                    )}
                </>
            )}
        </div>
    );
}

// ── Escalation banner ──────────────────────────────────────────────────────

export function EscalationBanner({
    c,
    handoff,
    canReply,
}: {
    c: Conversation;
    handoff: ConversationHandoff;
    canReply: boolean;
}) {
    const t = useT();
    const [pending, start] = useTransition();
    const fields = Object.entries(handoff.collected_fields ?? {});
    return (
        <Alert
            className="rounded-none border-x-0 border-t-0 border-amber-500/50 bg-amber-500/10"
            data-testid="escalation-banner"
            data-reason={handoff.reason}
        >
            <Hand />
            <AlertTitle className="flex flex-wrap items-center justify-between gap-2">
                <span>
                    {t('inbox.ai.banner.title')} —{' '}
                    {t(`inbox.ai.reasons.${handoff.reason}`)}
                </span>
                {canReply && (
                    <Button
                        size="sm"
                        disabled={pending}
                        onClick={() =>
                            start(async () => {
                                notify(await takeOverAction(c.id));
                            })
                        }
                        data-testid="banner-take-over"
                    >
                        <Hand /> {t('inbox.ai.takeOver')}
                    </Button>
                )}
            </AlertTitle>
            <AlertDescription className="space-y-1 text-sm">
                {handoff.reason_detail && (
                    <p className="text-xs text-muted-foreground">
                        {handoff.reason_detail}
                    </p>
                )}
                {handoff.summary && (
                    <p>
                        <span className="font-medium">
                            {t('inbox.ai.banner.summary')}:
                        </span>{' '}
                        {handoff.summary}
                    </p>
                )}
                {handoff.detected_need && (
                    <p>
                        <span className="font-medium">
                            {t('inbox.ai.banner.need')}:
                        </span>{' '}
                        {handoff.detected_need}
                    </p>
                )}
                {handoff.suggested_next_action && (
                    <p>
                        <span className="font-medium">
                            {t('inbox.ai.banner.next')}:
                        </span>{' '}
                        {handoff.suggested_next_action}
                    </p>
                )}
                {fields.length > 0 && (
                    <div>
                        <span className="font-medium">
                            {t('inbox.ai.banner.fields')}:
                        </span>
                        <ul className="mt-0.5 flex flex-wrap gap-1">
                            {fields.map(([k, v]) => (
                                <li key={k}>
                                    <Badge variant="outline">
                                        {k}: {v}
                                    </Badge>
                                </li>
                            ))}
                        </ul>
                    </div>
                )}
                {handoff.summary_source !== 'none' && (
                    <p className="text-[11px] text-muted-foreground">
                        {handoff.summary_source === 'ai'
                            ? t('inbox.ai.banner.aiSummary')
                            : t('inbox.ai.banner.fallback')}
                    </p>
                )}
            </AlertDescription>
        </Alert>
    );
}

// ── AI draft card ──────────────────────────────────────────────────────────

export function DraftCard({
    c,
    draft,
    canReply,
}: {
    c: Conversation;
    draft: AIDraft;
    canReply: boolean;
}) {
    const t = useT();
    const [pending, start] = useTransition();
    const [editing, setEditing] = useState(false);
    const [body, setBody] = useState(draft.body);
    const decide = (decision: 'accept' | 'reject', text?: string) =>
        start(async () => {
            if (notify(await decideDraftAction(c.id, draft.id, decision, text)))
                setEditing(false);
        });
    return (
        <div
            className="space-y-2 rounded-lg border border-violet-500/40 bg-violet-500/5 p-3"
            data-testid="ai-draft"
            data-kind={draft.kind}
        >
            <div className="flex flex-wrap items-center gap-2 text-xs">
                <Badge className="gap-1 bg-violet-600 text-white">
                    <Sparkles />
                    {t(`inbox.ai.draft.${draft.kind}`)}
                </Badge>
                <span className="text-muted-foreground">
                    {t('inbox.ai.draft.confidence', {
                        value: Math.round(draft.confidence * 100),
                    })}
                </span>
                {draft.intent && (
                    <span className="text-muted-foreground">
                        {t('inbox.ai.draft.intent', { intent: draft.intent })}
                    </span>
                )}
            </div>
            {editing ? (
                <>
                    <Label htmlFor="draft-body" className="sr-only">
                        {t('inbox.ai.draft.edit')}
                    </Label>
                    <Textarea
                        id="draft-body"
                        rows={3}
                        maxLength={4096}
                        value={body}
                        onChange={(e) => setBody(e.target.value)}
                    />
                </>
            ) : (
                <p
                    className="text-sm whitespace-pre-wrap"
                    data-testid="ai-draft-body"
                >
                    {draft.body}
                </p>
            )}
            {canReply && (
                <div className="flex flex-wrap gap-2">
                    {editing ? (
                        <>
                            <Button
                                size="sm"
                                disabled={pending || !body.trim()}
                                onClick={() => decide('accept', body.trim())}
                            >
                                <Check /> {t('inbox.ai.draft.send')}
                            </Button>
                            <Button
                                size="sm"
                                variant="ghost"
                                disabled={pending}
                                onClick={() => {
                                    setEditing(false);
                                    setBody(draft.body);
                                }}
                            >
                                {t('inbox.ai.draft.cancel')}
                            </Button>
                        </>
                    ) : (
                        <>
                            <Button
                                size="sm"
                                disabled={pending || !c.window_open}
                                onClick={() => decide('accept')}
                                data-testid="ai-draft-accept"
                            >
                                <Check /> {t('inbox.ai.draft.accept')}
                            </Button>
                            <Button
                                size="sm"
                                variant="outline"
                                disabled={pending || !c.window_open}
                                onClick={() => setEditing(true)}
                            >
                                <Pencil /> {t('inbox.ai.draft.edit')}
                            </Button>
                            <Button
                                size="sm"
                                variant="ghost"
                                disabled={pending}
                                onClick={() => decide('reject')}
                                data-testid="ai-draft-reject"
                            >
                                <X /> {t('inbox.ai.draft.reject')}
                            </Button>
                        </>
                    )}
                </div>
            )}
        </div>
    );
}

// ── Notes ──────────────────────────────────────────────────────────────────

/** An internal note in the thread: clearly not a customer-visible message. */
export function NoteBubble({ n }: { n: ConversationNote }) {
    const t = useT();
    const prefs = useDisplayPrefs();
    const formatDateTime = useFormatDateTime();
    return (
        <li
            className="flex justify-center"
            data-testid="internal-note"
            data-for-ai={n.for_ai}
        >
            <div className="w-full max-w-[85%] space-y-1 rounded-md border border-dashed border-amber-500/60 bg-amber-50 px-2.5 py-1.5 text-sm text-amber-950 dark:bg-amber-950/40 dark:text-amber-100">
                <p className="flex items-center gap-1 text-[11px] font-medium tracking-wide uppercase opacity-80">
                    <StickyNote className="size-3" />
                    {t('inbox.ai.note.label')}
                    {n.for_ai && (
                        <Badge
                            variant="outline"
                            className="h-4 gap-0.5 px-1 text-[10px]"
                        >
                            <Bot className="size-2.5" />
                            {t('inbox.ai.note.forAI')}
                        </Badge>
                    )}
                </p>
                <p className="break-words whitespace-pre-wrap">{n.body}</p>
                <p className="text-right text-[11px] opacity-60">
                    {n.author?.name ? `${n.author.name} · ` : ''}
                    <time
                        dateTime={n.created_at}
                        title={formatDateTime(n.created_at)}
                    >
                        {formatTime(n.created_at, prefs)}
                    </time>
                </p>
            </div>
        </li>
    );
}

/** The composer's note mode. */
export function NoteComposer({ c }: { c: Conversation }) {
    const t = useT();
    const [text, setText] = useState('');
    const [forAI, setForAI] = useState(false);
    const [pending, start] = useTransition();
    return (
        <form
            className="space-y-2"
            data-testid="note-composer"
            onSubmit={(e) => {
                e.preventDefault();
                if (!text.trim()) return;
                start(async () => {
                    if (
                        notify(
                            await addConversationNoteAction(
                                c.id,
                                text.trim(),
                                forAI,
                            ),
                        )
                    )
                        setText('');
                });
            }}
        >
            <Label htmlFor="conversation-note" className="sr-only">
                {t('inbox.ai.composer.note')}
            </Label>
            <Textarea
                id="conversation-note"
                rows={2}
                maxLength={4000}
                value={text}
                className="max-h-48 min-h-10 resize-none border-amber-500/50 bg-amber-50/50 dark:bg-amber-950/20"
                placeholder={t('inbox.ai.composer.placeholder')}
                onChange={(e) => setText(e.target.value)}
            />
            <div className="flex flex-wrap items-center justify-between gap-2">
                <label className="flex items-center gap-2 text-sm">
                    <Switch
                        checked={forAI}
                        onCheckedChange={setForAI}
                        aria-label={t('inbox.ai.composer.forAI')}
                        data-testid="note-for-ai"
                    />
                    <span>
                        {t('inbox.ai.composer.forAI')}
                        <span className="block text-xs text-muted-foreground">
                            {t('inbox.ai.composer.forAIHint')}
                        </span>
                    </span>
                </label>
                <Button
                    size="sm"
                    type="submit"
                    disabled={pending || !text.trim()}
                >
                    {pending ? <Spinner /> : <StickyNote />}{' '}
                    {t('inbox.ai.composer.add')}
                </Button>
            </div>
        </form>
    );
}

// ── Contact AI switch ──────────────────────────────────────────────────────

export function ContactAISwitch({
    c,
    canEdit,
}: {
    c: Conversation;
    canEdit: boolean;
}) {
    const t = useT();
    const [pending, start] = useTransition();
    return (
        <label className="flex items-start justify-between gap-3 text-sm">
            <span>
                <span className="block font-medium">
                    {t('inbox.ai.panel.contactAI')}
                </span>
                <span className="block text-xs text-muted-foreground">
                    {t('inbox.ai.panel.contactAIHint')}
                </span>
            </span>
            <Switch
                checked={!c.contact.ai_disabled}
                disabled={!canEdit || pending}
                aria-label={t('inbox.ai.panel.contactAI')}
                data-testid="contact-ai"
                onCheckedChange={(on) =>
                    start(async () => {
                        notify(await setContactAIAction(c.contact.id, !on));
                    })
                }
            />
        </label>
    );
}

// ── Activity ───────────────────────────────────────────────────────────────

function str(v: unknown): string {
    return typeof v === 'string' || typeof v === 'number' ? String(v) : '';
}

/** A readable line for one timeline item. */
export function activityText(t: Translate, a: ConversationActivity): string {
    const d = a.data ?? {};
    const k = 'inbox.ai.activity.';
    switch (a.kind) {
        case 'handed_to_ai':
            return d.profile_name
                ? t('inbox.ai.activity.handed_to_ai_profile', {
                      name: str(d.profile_name),
                  })
                : t('inbox.ai.activity.handed_to_ai');
        case 'handed_to_human':
            return d.to === 'pool'
                ? t('inbox.ai.activity.handed_to_pool')
                : t('inbox.ai.activity.handed_to_person');
        case 'ai_mode_changed':
            return t('inbox.ai.activity.ai_mode_changed', {
                mode: tr(
                    t,
                    `inbox.ai.mode.${str(d.mode)}`,
                    undefined,
                    str(d.mode),
                ),
            });
        case 'assignee_changed':
            return d.assignee_id
                ? t('inbox.ai.activity.assigned', {
                      name: str(d.assignee_name) || '—',
                  })
                : t('inbox.ai.activity.unassigned');
        case 'closed':
        case 'reopened':
            return t(`inbox.ai.activity.${a.kind}`);
        case 'note_added':
            return t(
                d.for_ai
                    ? 'inbox.ai.activity.note_ai'
                    : 'inbox.ai.activity.note',
                {
                    text: str(d.preview),
                },
            );
        case 'ai_action':
            if (d.action === 'start_automation') {
                if (d.status === 'unavailable')
                    return t('inbox.ai.activity.unavailable');
                return t('inbox.ai.activity.start_automation', {
                    name: str(d.automation_name),
                });
            }
            if (d.action === 'update_field')
                return t('inbox.ai.activity.update_field', {
                    field: str(d.label) || str(d.field),
                });
            if (d.status === 'not_allowed')
                return t('inbox.ai.activity.not_allowed', {
                    action: str(d.action),
                });
            return t('inbox.ai.activity.requested', { action: str(d.action) });
        case 'ai_reply_sent':
            return t('inbox.ai.activity.ai_reply_sent', {
                value: Math.round(Number(d.confidence ?? 0) * 100),
            });
        case 'ai_draft_created':
            return t(
                d.kind === 'approval'
                    ? 'inbox.ai.activity.draft_approval'
                    : 'inbox.ai.activity.draft_suggestion',
            );
        case 'ai_draft_decided':
            return tr(t, `${k}draft_${str(d.status)}`);
        case 'ai_error':
            return t('inbox.ai.activity.ai_error', {
                detail: str(d.detail) || '—',
            });
        case 'escalation':
            return t('inbox.ai.activity.escalation', {
                reason: tr(
                    t,
                    `inbox.ai.reasons.${str(d.reason)}`,
                    undefined,
                    str(d.reason),
                ),
            });
        case 'handoff_accepted':
            return t('inbox.ai.activity.handoff_accepted');
        case 'automation_started':
            return t('inbox.ai.activity.automation_started', {
                name: str(d.automation_name),
                by: tr(
                    t,
                    `inbox.ai.run.startedBy.${str(d.started_by)}`,
                    undefined,
                    str(d.started_by),
                ),
            });
        case 'automation_completed':
            return t('inbox.ai.activity.automation_completed', {
                name: str(d.automation_name),
                steps: str(d.steps) || '0',
            });
        case 'automation_failed':
            return t('inbox.ai.activity.automation_failed', {
                name: str(d.automation_name),
                error: str(d.error) || str(d.status),
            });
        case 'message_sent':
            return t('inbox.ai.activity.message_sent', {
                source: str(d.source),
            });
        case 'message_failed':
            return t('inbox.ai.activity.message_failed', {
                code: d.error_code ? ` (${str(d.error_code)})` : '',
            });
        case 'contact_ai_disabled':
        case 'contact_ai_enabled':
            return t(`inbox.ai.activity.${a.kind}`);
    }
    return a.kind;
}

function ActivityIcon({ a }: { a: ConversationActivity }) {
    if (a.actor_type === 'ai')
        return <Bot className="size-3.5 text-violet-600" />;
    if (a.kind.startsWith('automation'))
        return <Workflow className="size-3.5" />;
    if (a.kind === 'escalation')
        return <Hand className="size-3.5 text-amber-600" />;
    if (a.kind === 'note_added') return <StickyNote className="size-3.5" />;
    return <UserRound className="size-3.5" />;
}

export function ActivityTimeline({
    c,
    items,
    cursor,
}: {
    c: Conversation;
    items: ConversationActivity[];
    cursor?: string;
}) {
    const t = useT();
    const prefs = useDisplayPrefs();
    const formatDateTime = useFormatDateTime();
    const [older, setOlder] = useState<ConversationActivity[]>([]);
    const [next, setNext] = useState<string | undefined>();
    const [loaded, setLoaded] = useState(false);
    const [pending, start] = useTransition();
    const ids = new Set(items.map((a) => a.id));
    const all = [...older.filter((a) => !ids.has(a.id)), ...items];
    const more = loaded ? next : cursor;
    return (
        <section className="space-y-2" data-testid="activity">
            {more && (
                <Button
                    size="sm"
                    variant="ghost"
                    disabled={pending}
                    onClick={() =>
                        start(async () => {
                            const r = await olderActivityAction(c.id, more);
                            if (notify(r)) {
                                setOlder((o) => [...r.data.items, ...o]);
                                setNext(r.data.next);
                                setLoaded(true);
                            }
                        })
                    }
                >
                    {pending ? <Spinner /> : null} {t('inbox.ai.panel.older')}
                </Button>
            )}
            {all.length === 0 ? (
                <p className="text-sm text-muted-foreground">
                    {t('inbox.ai.panel.noActivity')}
                </p>
            ) : (
                <ol className="space-y-2 border-l pl-3 text-sm">
                    {all.map((a) => (
                        <li
                            key={a.id}
                            data-testid="activity-item"
                            data-kind={a.kind}
                            className="flex gap-2"
                        >
                            <span className="mt-0.5 shrink-0">
                                <ActivityIcon a={a} />
                            </span>
                            <span>
                                <time
                                    dateTime={a.created_at}
                                    title={formatDateTime(a.created_at)}
                                    className="text-muted-foreground tabular-nums"
                                >
                                    {formatTime(a.created_at, prefs)}
                                </time>{' '}
                                — {activityText(t, a)}
                                {a.actor_name && (
                                    <span className="block text-xs text-muted-foreground">
                                        {a.actor_name}
                                    </span>
                                )}
                            </span>
                        </li>
                    ))}
                </ol>
            )}
        </section>
    );
}

export function AutomationRuns({
    runs,
}: {
    runs: ConversationAutomationRun[];
}) {
    const t = useT();
    const formatDateTime = useFormatDateTime();
    return (
        <section className="space-y-2" data-testid="automation-runs">
            <h3 className="text-xs font-medium tracking-wide text-muted-foreground uppercase">
                {t('inbox.ai.panel.automations')}
            </h3>
            {runs.length === 0 ? (
                <p className="text-sm text-muted-foreground">
                    {t('inbox.ai.panel.noRuns')}
                </p>
            ) : (
                <ul className="space-y-2 text-sm">
                    {runs.map((r) => (
                        <li
                            key={r.id}
                            className="space-y-0.5 rounded-md border p-2"
                            data-testid="automation-run"
                            data-status={r.status}
                        >
                            <div className="flex items-center justify-between gap-2">
                                <span className="truncate font-medium">
                                    {r.automation_name}
                                </span>
                                <Badge
                                    variant={
                                        r.status === 'failed'
                                            ? 'destructive'
                                            : r.status === 'completed'
                                              ? 'secondary'
                                              : 'default'
                                    }
                                    className="shrink-0"
                                >
                                    {t(`inbox.ai.run.status.${r.status}`)}
                                </Badge>
                            </div>
                            <p className="text-xs text-muted-foreground">
                                {t(`inbox.ai.run.startedBy.${r.started_by}`)} ·{' '}
                                {t('inbox.ai.run.steps', {
                                    count: r.steps_done,
                                })}
                                {r.last_action
                                    ? ` · ${t('inbox.ai.run.last', { action: r.last_action })}`
                                    : ''}
                            </p>
                            <p className="text-xs text-muted-foreground">
                                {t('inbox.ai.run.started', {
                                    time: formatDateTime(r.created_at),
                                })}
                                {r.finished_at
                                    ? ` · ${t('inbox.ai.run.finished', { time: formatDateTime(r.finished_at) })}`
                                    : ''}
                            </p>
                            {r.error && (
                                <p className={cn('text-xs text-destructive')}>
                                    {r.error}
                                </p>
                            )}
                        </li>
                    ))}
                </ul>
            )}
        </section>
    );
}
