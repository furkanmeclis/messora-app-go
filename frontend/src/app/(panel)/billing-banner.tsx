import Link from 'next/link';
import { AlertTriangle, Clock, Lock, MessageCircle } from 'lucide-react';
import { getT } from '@/i18n/server';
import { billingBanner, BILLING_PAGE } from '@/lib/billing';
import { formatDate, type DisplayPrefs } from '@/lib/format';
import type { Schemas } from '@/lib/types';
import { cn } from '@/lib/utils';

/**
 * The subscription's state on every page (Phase 17 WP9): read-only (red),
 * an unpaid renewal in its grace period (amber), a trial ending within
 * three days (info), a used-up message balance (amber).
 */
export async function BillingBanner({
    plan,
    prefs,
}: {
    plan: Schemas['MePlan'] | null | undefined;
    prefs: DisplayPrefs;
}) {
    const dateLabel = (iso: string) => formatDate(iso, prefs);
    const b = billingBanner(plan);
    if (!b) return null;
    const t = await getT();
    const tone =
        b.kind === 'read_only'
            ? 'border-destructive/50 bg-destructive/10'
            : b.kind === 'trial'
              ? 'border-primary/40 bg-primary/10'
              : 'border-warning/50 bg-warning/10';
    const Icon =
        b.kind === 'read_only'
            ? Lock
            : b.kind === 'trial'
              ? Clock
              : b.kind === 'balance'
                ? MessageCircle
                : AlertTriangle;
    const text =
        b.kind === 'read_only'
            ? t('billing.banner.readOnly')
            : b.kind === 'past_due'
              ? b.until
                  ? t('billing.banner.pastDue', { date: dateLabel(b.until) })
                  : t('billing.banner.pastDueNoDate')
              : b.kind === 'trial'
                ? b.days === 0
                    ? t('billing.banner.trialToday')
                    : t('billing.banner.trial', { days: b.days })
                : t('billing.banner.balance');
    return (
        <div
            role={b.kind === 'read_only' ? 'alert' : 'status'}
            data-testid="billing-banner"
            data-kind={b.kind}
            className={cn(
                'mb-4 flex flex-wrap items-center gap-x-3 gap-y-1.5 rounded-xl border px-4 py-2.5 text-sm',
                tone,
            )}
        >
            <Icon
                className={cn(
                    'size-4 shrink-0',
                    b.kind === 'read_only'
                        ? 'text-destructive'
                        : b.kind === 'trial'
                          ? 'text-primary'
                          : 'text-warning',
                )}
                aria-hidden
            />
            <strong className="font-semibold">{text}</strong>
            <Link
                href={BILLING_PAGE}
                className="ml-auto font-medium text-primary underline-offset-4 hover:underline"
            >
                {t('billing.banner.link')}
            </Link>
        </div>
    );
}
