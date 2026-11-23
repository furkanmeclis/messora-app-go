'use client';

import { useState } from 'react';
import Link from 'next/link';
import { ArrowRight, KeyRound, ShieldAlert } from 'lucide-react';
import { useFormatDate } from '@/components/display-preferences';
import { Button } from '@/components/ui/button';
import { useT } from '@/i18n/client';
import type { Schemas } from '@/lib/types';
import { cn } from '@/lib/utils';

type Summary = Schemas['LicenseSummary'];

const DAY = 24 * 60 * 60 * 1000;

/**
 * Tells the team when the licence needs attention. The trial banner is
 * shown only in its last week, and only to people who can act on it.
 */
export function LicenseBanner({
    license,
    canManage,
}: {
    license: Summary;
    canManage: boolean;
}) {
    const t = useT();
    const date = useFormatDate();
    const [now] = useState(() => Date.now());
    const until = license.until ? date(license.until) : '';
    const reason = license.reason
        ? t(`license.reason.${license.reason as 'expired'}`)
        : '';
    let text = '';
    switch (license.status) {
        case 'trial': {
            const days = license.until
                ? Math.max(
                      0,
                      Math.ceil((Date.parse(license.until) - now) / DAY),
                  )
                : 0;
            if (!canManage || days > 7) return null;
            text = t('license.banner.trial', { days });
            break;
        }
        case 'expiring':
            if (!canManage) return null;
            text = t('license.banner.expiring', { date: until });
            break;
        case 'grace':
            text = t('license.banner.grace', { reason, date: until });
            break;
        case 'restricted':
            text = t('license.banner.restricted', { reason });
            break;
        default:
            return null;
    }
    const severe = license.status === 'restricted';
    const Icon = severe ? ShieldAlert : KeyRound;
    return (
        <div
            data-chrome
            role={severe ? 'alert' : 'status'}
            className={cn(
                'mb-6 flex flex-wrap items-center gap-3 rounded-xl border p-4',
                severe
                    ? 'border-destructive/30 bg-destructive/10'
                    : 'border-amber-500/30 bg-amber-500/10',
            )}
        >
            <span
                className={cn(
                    'flex size-9 items-center justify-center rounded-lg text-white',
                    severe ? 'bg-destructive' : 'bg-amber-500',
                )}
            >
                <Icon className="size-4" />
            </span>
            <p className="min-w-0 flex-1 text-sm font-medium">{text}</p>
            {canManage && (
                <Button
                    asChild
                    size="sm"
                    variant={severe ? 'destructive' : 'outline'}
                >
                    <Link href="/settings/license">
                        {t('license.banner.action')} <ArrowRight />
                    </Link>
                </Button>
            )}
        </div>
    );
}
