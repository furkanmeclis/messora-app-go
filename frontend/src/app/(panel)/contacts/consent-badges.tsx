'use client';

import { BellOff, BellRing, CircleHelp, Hand, PhoneOff } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import {
    Tooltip,
    TooltipContent,
    TooltipTrigger,
} from '@/components/ui/tooltip';
import { useT } from '@/i18n/client';
import {
    statusTone,
    type ConsentCategory,
    type ConsentStatus,
} from '@/lib/consent';
import type { Contact } from '@/lib/types';
import { cn } from '@/lib/utils';

const TONE_CLASS = {
    ok: 'border-transparent bg-success/15 text-success',
    neutral: '',
    warn: 'border-transparent bg-amber-500/15 text-amber-700 dark:text-amber-400',
    bad: '',
} as const;

const TONE_ICON = {
    ok: BellRing,
    neutral: CircleHelp,
    warn: Hand,
    bad: BellOff,
} as const;

/** One category's consent, e.g. "Marketing: Opted out". */
export function ConsentBadge({
    category,
    status,
    compact,
}: {
    category: ConsentCategory;
    status: ConsentStatus;
    /**
     * For tables: shows the category name only; the status is in the icon,
     * colour, title and screen reader text.
     */
    compact?: boolean;
}) {
    const t = useT();
    const tone = statusTone(status);
    const Icon = TONE_ICON[tone];
    const label = `${t(`consent.categories.${category}`)}: ${t(`consent.statuses.${status}`)}`;
    return (
        <Badge
            variant={
                tone === 'bad'
                    ? 'destructive'
                    : tone === 'neutral'
                      ? 'outline'
                      : 'secondary'
            }
            className={cn('gap-1', TONE_CLASS[tone])}
            data-testid={`consent-${category}`}
            data-status={status}
            title={compact ? label : undefined}
        >
            <Icon className="size-3" aria-hidden />
            {compact ? (
                <>
                    <span className="sr-only">{label}</span>
                    <span aria-hidden>
                        {t(`consent.categories.${category}`)}
                    </span>
                </>
            ) : (
                label
            )}
        </Badge>
    );
}

/** "Stopped in WhatsApp" and "Undeliverable", when they apply. */
export function ReachabilityBadges({
    consent,
    withHelp,
}: {
    consent: Contact['consent'];
    withHelp?: boolean;
}) {
    const t = useT();
    const items: {
        id: string;
        label: string;
        help: string;
    }[] = [];
    if (consent.meta_marketing_stopped_at)
        items.push({
            id: 'consent-meta-stopped',
            label: t('consent.flags.metaStopped'),
            help: t('consent.flags.metaStoppedHint'),
        });
    if (consent.undeliverable_at)
        items.push({
            id: 'consent-undeliverable',
            label: t('consent.flags.undeliverable'),
            help:
                t('consent.flags.undeliverableHint') +
                (consent.undeliverable_code
                    ? ` (${t('consent.flags.undeliverableCode', { code: consent.undeliverable_code })})`
                    : ''),
        });
    return items.map((it) => {
        const badge = (
            <Badge
                key={it.id}
                variant="destructive"
                className="gap-1"
                data-testid={it.id}
            >
                <PhoneOff className="size-3" aria-hidden />
                {it.label}
            </Badge>
        );
        if (!withHelp) return badge;
        return (
            <Tooltip key={it.id}>
                <TooltipTrigger asChild>
                    <button
                        type="button"
                        className="rounded-full focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
                        aria-label={`${it.label}: ${it.help}`}
                    >
                        {badge}
                    </button>
                </TooltipTrigger>
                <TooltipContent className="max-w-xs">{it.help}</TooltipContent>
            </Tooltip>
        );
    });
}
