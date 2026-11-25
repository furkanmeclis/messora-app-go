'use client';

import { useState, useTransition } from 'react';
import { FlaskConical, Play, Plus, Trash2 } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from '@/components/ui/select';
import { Spinner } from '@/components/ui/spinner';
import { Textarea } from '@/components/ui/textarea';
import { useLocale, useT } from '@/i18n/client';
import type { Schemas } from '@/lib/types';
import { cn } from '@/lib/utils';
import { testProfileAction } from './actions';

type Msg = Schemas['AITestMessage'];
type Role = Msg['role'];

const ROLE_STYLE: Record<Role, string> = {
    user: 'mr-8 bg-muted',
    assistant: 'ml-8 bg-primary/10',
    operator_note: 'border border-dashed border-warning/50 bg-warning/5',
};

/** Runs a sample conversation through the saved profile. */
export function TestConsole({
    profileId,
    canRun,
    dirty,
}: {
    profileId: string | null;
    canRun: boolean;
    dirty: boolean;
}) {
    const t = useT();
    const locale = useLocale();
    const [msgs, setMsgs] = useState<Msg[]>([]);
    const [role, setRole] = useState<Role>('user');
    const [text, setText] = useState('');
    const [result, setResult] = useState<Schemas['AITestResult'] | null>(null);
    const [error, setError] = useState<string | null>(null);
    const [pending, start] = useTransition();

    const add = () => {
        if (!text.trim()) return;
        setMsgs([...msgs, { role, text: text.trim() }]);
        setText('');
    };
    const run = () => {
        if (!profileId) return;
        const all = text.trim() ? [...msgs, { role, text: text.trim() }] : msgs;
        if (!all.some((m) => m.role === 'user')) {
            setError(t('ai.test.empty'));
            return;
        }
        setMsgs(all);
        setText('');
        start(async () => {
            const r = await testProfileAction(profileId, {
                messages: all,
                language: locale === 'en' ? 'en' : 'tr',
            });
            if (r.ok) {
                setResult(r.data);
                setError(null);
            } else {
                setResult(null);
                setError(r.message);
            }
        });
    };

    return (
        <section
            className="overflow-hidden rounded-2xl border bg-card shadow-xs"
            data-testid="ai-test-console"
        >
            <header className="border-b bg-muted/30 px-5 py-4">
                <h2 className="flex items-center gap-2 font-semibold tracking-tight">
                    <FlaskConical className="size-4" />
                    {t('ai.test.title')}
                </h2>
                <p className="mt-1 text-sm text-muted-foreground">
                    {t('ai.test.description')}
                </p>
            </header>
            <div className="space-y-4 px-5 py-5">
                {!profileId ? (
                    <p className="text-sm text-muted-foreground">
                        {t('ai.profiles.saveFirst')}
                    </p>
                ) : (
                    <>
                        {msgs.length > 0 && (
                            <ol className="max-h-80 space-y-2 overflow-y-auto">
                                {msgs.map((m, i) => (
                                    <li
                                        key={i}
                                        className={cn(
                                            'rounded-xl px-3 py-2 text-sm',
                                            ROLE_STYLE[m.role],
                                        )}
                                    >
                                        <span className="block text-[11px] font-medium text-muted-foreground uppercase">
                                            {t(`ai.test.roles.${m.role}`)}
                                        </span>
                                        <span className="whitespace-pre-wrap">
                                            {m.text}
                                        </span>
                                    </li>
                                ))}
                            </ol>
                        )}
                        <div className="space-y-2">
                            <Select
                                value={role}
                                onValueChange={(r) => setRole(r as Role)}
                            >
                                <SelectTrigger
                                    className="w-full"
                                    aria-label={t('ai.test.roles.user')}
                                >
                                    <SelectValue />
                                </SelectTrigger>
                                <SelectContent>
                                    {(
                                        [
                                            'user',
                                            'assistant',
                                            'operator_note',
                                        ] as const
                                    ).map((r) => (
                                        <SelectItem key={r} value={r}>
                                            {t(`ai.test.roles.${r}`)}
                                        </SelectItem>
                                    ))}
                                </SelectContent>
                            </Select>
                            <Textarea
                                rows={3}
                                maxLength={4000}
                                value={text}
                                placeholder={t('ai.test.placeholder')}
                                aria-label={t('ai.test.placeholder')}
                                onChange={(e) => setText(e.target.value)}
                                onKeyDown={(e) => {
                                    if (
                                        e.key === 'Enter' &&
                                        (e.metaKey || e.ctrlKey)
                                    ) {
                                        e.preventDefault();
                                        run();
                                    }
                                }}
                            />
                            <div className="flex flex-wrap gap-2">
                                <Button
                                    type="button"
                                    variant="outline"
                                    size="sm"
                                    onClick={add}
                                    disabled={!text.trim()}
                                >
                                    <Plus /> {t('ai.test.add')}
                                </Button>
                                <Button
                                    type="button"
                                    size="sm"
                                    onClick={run}
                                    disabled={!canRun || pending}
                                    data-testid="ai-test-run"
                                >
                                    {pending ? <Spinner /> : <Play />}
                                    {t('ai.test.run')}
                                </Button>
                                {msgs.length > 0 && (
                                    <Button
                                        type="button"
                                        variant="ghost"
                                        size="sm"
                                        onClick={() => {
                                            setMsgs([]);
                                            setResult(null);
                                            setError(null);
                                        }}
                                    >
                                        <Trash2 /> {t('ai.test.clear')}
                                    </Button>
                                )}
                            </div>
                            {dirty && (
                                <p className="text-xs text-muted-foreground">
                                    {t('ai.profiles.saveFirst')}
                                </p>
                            )}
                        </div>
                        {error && (
                            <p
                                role="alert"
                                className="rounded-lg border border-destructive/40 bg-destructive/5 p-3 text-sm text-destructive"
                            >
                                {error}
                            </p>
                        )}
                        {result && (
                            <TestResult
                                result={result}
                                onUse={(r) =>
                                    setMsgs([
                                        ...msgs,
                                        { role: 'assistant', text: r },
                                    ])
                                }
                            />
                        )}
                    </>
                )}
            </div>
        </section>
    );
}

function TestResult({
    result,
    onUse,
}: {
    result: Schemas['AITestResult'];
    onUse: (reply: string) => void;
}) {
    const t = useT();
    const r = result.reply;
    const fields = r ? Object.entries(r.extracted_fields) : [];
    return (
        <div
            className="space-y-3 rounded-xl border p-3"
            data-testid="ai-test-result"
        >
            {r ? (
                <>
                    <div>
                        <span className="text-xs font-medium text-muted-foreground">
                            {t('ai.test.reply')}
                        </span>
                        <p
                            className="text-sm whitespace-pre-wrap"
                            data-testid="ai-test-reply"
                        >
                            {r.reply}
                        </p>
                    </div>
                    <dl className="grid grid-cols-2 gap-2 text-xs">
                        <div>
                            <dt className="text-muted-foreground">
                                {t('ai.test.confidence')}
                            </dt>
                            <dd>{Math.round(r.confidence * 100)}%</dd>
                        </div>
                        <div>
                            <dt className="text-muted-foreground">
                                {t('ai.test.intent')}
                            </dt>
                            <dd className="font-mono">{r.intent || '—'}</dd>
                        </div>
                        <div>
                            <dt className="text-muted-foreground">
                                {t('ai.test.escalate')}
                            </dt>
                            <dd>
                                <Badge
                                    variant={
                                        r.escalate ? 'destructive' : 'outline'
                                    }
                                >
                                    {r.escalate
                                        ? t('ai.test.yes')
                                        : t('ai.test.no')}
                                </Badge>
                            </dd>
                        </div>
                        {r.escalate_reason && (
                            <div>
                                <dt className="text-muted-foreground">
                                    {t('ai.test.reason')}
                                </dt>
                                <dd>{r.escalate_reason}</dd>
                            </div>
                        )}
                    </dl>
                    {fields.length > 0 && (
                        <div className="text-xs">
                            <span className="font-medium text-muted-foreground">
                                {t('ai.test.extracted')}
                            </span>
                            <ul className="mt-1 space-y-0.5">
                                {fields.map(([k, val]) => (
                                    <li key={k}>
                                        <span className="font-mono">{k}</span>:{' '}
                                        {val ?? '—'}
                                    </li>
                                ))}
                            </ul>
                        </div>
                    )}
                    {r.reply && (
                        <Button
                            type="button"
                            variant="ghost"
                            size="sm"
                            onClick={() => onUse(r.reply)}
                        >
                            {t('ai.test.useReply')}
                        </Button>
                    )}
                </>
            ) : (
                <pre className="overflow-x-auto text-xs whitespace-pre-wrap">
                    {result.text}
                </pre>
            )}
            <div className="space-y-1 border-t pt-2 text-xs text-muted-foreground">
                <p>
                    {result.provider_name} ·{' '}
                    <span className="font-mono">{result.model}</span> ·{' '}
                    {t('ai.test.latency', { ms: result.latency_ms })}
                </p>
                <p>
                    {t('ai.test.usage', {
                        input: result.usage.input_tokens,
                        cached: result.usage.cached_input_tokens,
                        output: result.usage.output_tokens,
                    })}
                </p>
                <p>
                    {t('ai.test.cost')}:{' '}
                    {result.cost
                        ? `${result.cost} ${result.currency ?? ''}${result.estimated ? ` (${t('ai.test.estimated')})` : ''}`
                        : t('ai.test.noPrice')}
                </p>
                <details>
                    <summary className="cursor-pointer">
                        {t('ai.test.instructions')}
                    </summary>
                    <pre className="mt-2 max-h-64 overflow-auto rounded bg-muted p-2 font-mono text-[11px] whitespace-pre-wrap">
                        {result.instructions}
                    </pre>
                </details>
            </div>
        </div>
    );
}
