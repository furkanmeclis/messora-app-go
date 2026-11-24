'use client';

import { useState, useTransition } from 'react';
import {
    ChevronDown,
    Clock,
    ExternalLink,
    ShieldAlert,
    ShieldCheck,
    Stethoscope,
} from 'lucide-react';
import { toast } from 'sonner';
import {
    useFormatDate,
    useFormatDateTime,
} from '@/components/display-preferences';
import { useReauth } from '@/components/reauth';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Spinner } from '@/components/ui/spinner';
import { useT } from '@/i18n/client';
import type { WhatsAppAccount } from '@/lib/types';
import { cn } from '@/lib/utils';
import { checkAccountAction } from './actions';

const DAY = 24 * 60 * 60 * 1000;

type Health = 'permanent' | 'expiring' | 'soon' | 'expired' | 'unknown';

function healthOf(a: WhatsAppAccount, now: number): Health {
    const tok = a.token;
    if (!tok.checked_at || !tok.type) return 'unknown';
    if (!tok.expires_at) return 'permanent';
    const left = Date.parse(tok.expires_at) - now;
    if (left <= 0) return 'expired';
    if (left < 7 * DAY) return 'soon';
    return 'expiring';
}

const TONE: Record<Health, string> = {
    permanent: 'bg-emerald-500/10 text-emerald-700 dark:text-emerald-400',
    expiring: 'bg-amber-500/10 text-amber-700 dark:text-amber-400',
    soon: 'bg-destructive/10 text-destructive',
    expired: 'bg-destructive/10 text-destructive',
    unknown: 'bg-muted text-muted-foreground',
};

/** The token's type and lifetime as Meta reported it, plus "check now". */
export function TokenHealth({
    account: a,
    manage,
}: {
    account: WhatsAppAccount;
    manage: boolean;
}) {
    const t = useT();
    const date = useFormatDate();
    const dateTime = useFormatDateTime();
    const { guard } = useReauth();
    const [now] = useState(() => Date.now());
    const [pending, start] = useTransition();
    const health = healthOf(a, now);
    const Icon =
        health === 'permanent'
            ? ShieldCheck
            : health === 'unknown'
              ? ShieldAlert
              : Clock;

    const check = () =>
        start(async () => {
            const r = await guard(() => checkAccountAction(a.id));
            if (!r.ok) {
                toast.error(r.message);
                return;
            }
            if (r.data.status === 'error')
                toast.error(t('channels.token.checkFailed'));
            else toast.success(t('channels.token.checkOk'));
        });

    return (
        <div className="space-y-2 rounded-lg border bg-muted/20 p-3">
            <div className="flex flex-wrap items-center gap-2">
                <Badge className={cn('h-6 gap-1.5 px-2.5', TONE[health])}>
                    <Icon className="size-3.5" />
                    {health === 'permanent' && t('channels.token.permanent')}
                    {(health === 'expiring' || health === 'soon') &&
                        t('channels.token.until', {
                            date: date(a.token.expires_at),
                        })}
                    {health === 'expired' && t('channels.token.expired')}
                    {health === 'unknown' && t('channels.token.unknown')}
                </Badge>
                {a.token.type && (
                    <span className="text-xs text-muted-foreground">
                        {a.token.type === 'SYSTEM_USER'
                            ? t('channels.token.systemUser')
                            : t('channels.token.type', { type: a.token.type })}
                    </span>
                )}
                <span className="ml-auto text-xs text-muted-foreground">
                    {a.token.checked_at
                        ? t('channels.token.checkedAt', {
                              date: dateTime(a.token.checked_at),
                          })
                        : t('channels.token.neverChecked')}
                </span>
                {manage && (
                    <Button
                        size="sm"
                        variant="ghost"
                        disabled={pending}
                        onClick={check}
                    >
                        {pending ? <Spinner /> : <Stethoscope />}{' '}
                        {t('channels.token.check')}
                    </Button>
                )}
            </div>
            {a.token.warning && health !== 'permanent' && (
                <p className="text-xs text-muted-foreground">
                    {a.token.warning}
                </p>
            )}
        </div>
    );
}

/** Step-by-step: a System User token that never expires. */
export function SystemUserGuide({
    defaultOpen = false,
}: {
    defaultOpen?: boolean;
}) {
    const t = useT();
    const [open, setOpen] = useState(defaultOpen);
    const steps = [
        t('channels.guide.step1'),
        t('channels.guide.step2'),
        t('channels.guide.step3'),
        t('channels.guide.step4'),
        t('channels.guide.step5'),
    ];
    return (
        <div className="rounded-lg border">
            <button
                type="button"
                className="flex w-full items-center gap-2 px-3 py-2 text-left text-sm font-medium"
                aria-expanded={open}
                onClick={() => setOpen((o) => !o)}
            >
                <ShieldCheck className="size-4 text-emerald-600" />
                {t('channels.guide.title')}
                <ChevronDown
                    className={cn(
                        'ml-auto size-4 transition-transform',
                        open && 'rotate-180',
                    )}
                />
            </button>
            {open && (
                <div className="space-y-3 border-t px-3 py-3 text-sm">
                    <p className="text-muted-foreground">
                        {t('channels.guide.why')}
                    </p>
                    <ol className="space-y-2">
                        {steps.map((s, i) => (
                            <li key={i} className="flex gap-2">
                                <span className="flex size-5 shrink-0 items-center justify-center rounded-full bg-primary/10 text-xs font-semibold text-primary">
                                    {i + 1}
                                </span>
                                <span>{s}</span>
                            </li>
                        ))}
                    </ol>
                    <a
                        className="inline-flex items-center gap-1 text-xs text-primary underline-offset-4 hover:underline"
                        href="https://business.facebook.com/settings/system-users"
                        target="_blank"
                        rel="noreferrer"
                    >
                        {t('channels.guide.open')}{' '}
                        <ExternalLink className="size-3" />
                    </a>
                </div>
            )}
        </div>
    );
}

/** Shown when Meta refused the account's credentials. */
export function AccountProblem({
    account: a,
    action,
}: {
    account: WhatsAppAccount;
    action?: React.ReactNode;
}) {
    const t = useT();
    if (a.status !== 'error' || !a.last_error) return null;
    return (
        <Alert variant="destructive">
            <ShieldAlert />
            <AlertTitle>{t('channels.token.problemTitle')}</AlertTitle>
            <AlertDescription className="space-y-2">
                <p>{a.last_error}</p>
                <p>{t('channels.token.problemHint')}</p>
                {action}
            </AlertDescription>
        </Alert>
    );
}
