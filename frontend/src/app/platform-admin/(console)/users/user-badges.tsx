import { Badge } from '@/components/ui/badge';
import type { getT } from '@/i18n/server';
import type { Schemas } from '@/lib/types';

/** Status badges of an account (server components pass their t()). */
export function UserBadges({
    user: u,
    t,
}: {
    user: Schemas['PlatformUser'];
    t: Awaited<ReturnType<typeof getT>>;
}) {
    return (
        <>
            <Badge
                variant="outline"
                className={
                    u.status === 'active'
                        ? 'border-success/40 bg-success/10 text-success'
                        : 'border-destructive/40 bg-destructive/10 text-destructive'
                }
            >
                {u.status === 'active'
                    ? t('platform.users.active')
                    : t('platform.users.disabled')}
            </Badge>
            {u.is_platform_admin && <Badge>{t('platform.users.admin')}</Badge>}
            {u.mfa_enabled && (
                <Badge variant="outline">{t('platform.users.mfa')}</Badge>
            )}
            {!u.email_verified && (
                <Badge variant="outline">
                    {t('platform.users.unverified')}
                </Badge>
            )}
            {u.password_reset_required && (
                <Badge variant="outline">
                    {t('platform.users.resetRequired')}
                </Badge>
            )}
            {u.locked_until && new Date(u.locked_until) > new Date() && (
                <Badge variant="outline">{t('platform.users.locked')}</Badge>
            )}
        </>
    );
}
