'use client';

import Link from 'next/link';
import {
    AlertCircle,
    ShieldBan,
    ShieldCheck,
    TriangleAlert,
} from 'lucide-react';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { Badge } from '@/components/ui/badge';
import { Spinner } from '@/components/ui/spinner';
import { useT } from '@/i18n/client';
import { refusedCount, sortReasons } from '@/lib/consent';
import { NUMBER_LOCALE } from '@/lib/format';
import type { ActionResult, Schemas } from '@/lib/types';
import type { Preflight } from './actions';

type Reason = Schemas['PreflightReason'];

const fmt = (n: number) => n.toLocaleString(NUMBER_LOCALE);

/** The consent check of the review step (POST /campaigns/preflight). */
export function PreflightPanel({
    result,
    canManageSettings,
}: {
    /** undefined while checking. */
    result: ActionResult<Preflight> | undefined;
    canManageSettings: boolean;
}) {
    const t = useT();
    if (!result) {
        return (
            <p
                className="flex items-center gap-2 text-sm text-muted-foreground"
                role="status"
                data-testid="campaign-preflight"
                data-state="loading"
            >
                <Spinner /> {t('consent.preflight.checking')}
            </p>
        );
    }
    if (!result.ok) {
        return (
            <Alert
                variant="destructive"
                data-testid="campaign-preflight"
                data-state="error"
            >
                <AlertCircle />
                <AlertDescription>
                    {t('consent.preflight.failed', {
                        message: result.message,
                    })}
                </AlertDescription>
            </Alert>
        );
    }
    const p = result.data;
    const refused = refusedCount(p.blocked_reasons);
    const hasUnknown = [...p.blocked_reasons, ...p.warned_reasons].some(
        (r) => r.reason === 'consent_unknown',
    );
    const tail =
        refused > 0
            ? t('consent.preflight.refused', { n: fmt(refused) })
            : p.blocked > 0
              ? t('consent.preflight.blockedOnly', { n: fmt(p.blocked) })
              : t('consent.preflight.allClear');

    return (
        <section
            className="space-y-4 rounded-lg border p-4"
            aria-labelledby="preflight-title"
            data-testid="campaign-preflight"
            data-state="ready"
        >
            <div className="space-y-1">
                <h3 id="preflight-title" className="font-medium">
                    {t('consent.preflight.title')}
                </h3>
                <p className="text-sm" data-testid="preflight-summary">
                    {t('consent.preflight.audience', {
                        count: p.total,
                        n: fmt(p.total),
                    })}
                    {tail}
                </p>
                <p
                    className="flex items-center gap-1.5 text-sm font-medium"
                    data-testid="preflight-eligible"
                    data-count={p.eligible}
                >
                    <ShieldCheck className="size-4 text-success" aria-hidden />
                    {p.warned > 0
                        ? t('consent.preflight.eligibleWarned', {
                              n: fmt(p.eligible),
                              warned: fmt(p.warned),
                          })
                        : t('consent.preflight.eligible', {
                              n: fmt(p.eligible),
                          })}
                </p>
                <p className="text-xs text-muted-foreground">
                    {t(`consent.preflight.category.${p.category}`)}
                </p>
            </div>

            {p.total > 0 && p.eligible === 0 && (
                <Alert variant="destructive" data-testid="preflight-none">
                    <AlertCircle />
                    <AlertDescription>
                        {t('consent.preflight.noneEligible')}
                    </AlertDescription>
                </Alert>
            )}

            {p.blocked_reasons.length > 0 && (
                <ReasonList
                    testId="preflight-blocked"
                    count={p.blocked}
                    title={t('consent.preflight.blocked', {
                        n: fmt(p.blocked),
                    })}
                    reasons={p.blocked_reasons}
                    icon={<ShieldBan className="size-4 text-destructive" />}
                />
            )}
            {p.warned_reasons.length > 0 && (
                <ReasonList
                    testId="preflight-warned"
                    count={p.warned}
                    title={t('consent.preflight.warned', {
                        n: fmt(p.warned),
                    })}
                    reasons={p.warned_reasons}
                    icon={<TriangleAlert className="size-4 text-amber-600" />}
                />
            )}

            {p.hard_blocked > 0 && (
                <p className="text-xs text-muted-foreground">
                    {t('consent.preflight.hardNote')}
                </p>
            )}
            {hasUnknown && (
                <p
                    className="text-xs text-muted-foreground"
                    data-testid="preflight-policy"
                    data-policy={p.unknown_marketing}
                >
                    {t(
                        `consent.preflight.unknownPolicy.${p.unknown_marketing}`,
                    )}{' '}
                    {canManageSettings && (
                        <Link
                            href="/settings/consent"
                            className="font-medium text-foreground underline underline-offset-2"
                        >
                            {t('consent.preflight.changePolicy')}
                        </Link>
                    )}
                </p>
            )}
        </section>
    );
}

function ReasonList({
    testId,
    count,
    title,
    reasons,
    icon,
}: {
    testId: string;
    count: number;
    title: string;
    reasons: Reason[];
    icon: React.ReactNode;
}) {
    const t = useT();
    return (
        <div className="space-y-2" data-testid={testId} data-count={count}>
            <h4 className="flex items-center gap-1.5 text-sm font-medium">
                {icon}
                {title}
            </h4>
            <ul className="space-y-2 text-sm">
                {sortReasons(reasons).map((r) => (
                    <li
                        key={r.reason}
                        data-testid={`preflight-reason-${r.reason}`}
                        data-count={r.count}
                        className="flex gap-3"
                    >
                        <span className="w-14 shrink-0 text-right font-medium tabular-nums">
                            {fmt(r.count)}
                        </span>
                        <span className="min-w-0">
                            <span className="flex flex-wrap items-center gap-1.5">
                                {t(`consent.reasons.${r.reason}`)}
                                {r.hard && (
                                    <Badge
                                        variant="outline"
                                        className="text-[10px]"
                                    >
                                        {t('consent.preflight.hard')}
                                    </Badge>
                                )}
                            </span>
                            <span className="block text-xs text-muted-foreground">
                                {t(`consent.reasonHelp.${r.reason}`)}
                            </span>
                        </span>
                    </li>
                ))}
            </ul>
        </div>
    );
}
