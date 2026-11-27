import { Eye, LifeBuoy, PenLine } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { getLocale, getT } from '@/i18n/server';
import { intlLocale } from '@/i18n/config';
import type { Me } from '@/lib/types';

/**
 * Shown on every page while Messora support has this workspace open
 * (support access from the platform admin panel). The workspace's audit
 * log records the session.
 */
export async function SupportBanner({
    support,
}: {
    support: NonNullable<Me['support']>;
}) {
    const [t, locale] = await Promise.all([getT(), getLocale()]);
    const until = new Intl.DateTimeFormat(intlLocale(locale), {
        timeStyle: 'short',
    }).format(new Date(support.expires_at));
    return (
        <div
            role="status"
            data-testid="support-banner"
            className="mb-4 flex flex-wrap items-center gap-x-3 gap-y-1.5 rounded-xl border border-warning/50 bg-warning/10 px-4 py-2.5 text-sm"
        >
            <LifeBuoy className="size-4 shrink-0 text-warning" aria-hidden />
            <strong className="font-semibold">
                {t('topbar.support.title', { name: support.admin_name })}
            </strong>
            <Badge variant="outline" className="gap-1">
                {support.read_only ? (
                    <Eye className="size-3" aria-hidden />
                ) : (
                    <PenLine className="size-3" aria-hidden />
                )}
                {support.read_only
                    ? t('topbar.support.readOnly')
                    : t('topbar.support.write')}
            </Badge>
            <span className="text-muted-foreground">
                {t('topbar.support.reason', { reason: support.reason })}
            </span>
            <span className="ml-auto text-muted-foreground tabular-nums">
                {t('topbar.support.until', { time: until })}
            </span>
        </div>
    );
}
