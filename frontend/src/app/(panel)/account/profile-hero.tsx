import Link from 'next/link';
import {
    CalendarDays,
    Clock,
    Crown,
    BriefcaseBusiness,
    Headset,
    ShieldAlert,
    ShieldCheck,
} from 'lucide-react';
import { getT } from '@/i18n/server';
import { formatDate, formatDateTime, type DisplayPrefs } from '@/lib/format';
import type { User } from '@/lib/types';
import { cn } from '@/lib/utils';
import { roleLabel } from '@/i18n/roles';

const ROLE_ICON = {
    owner: Crown,
    admin: ShieldCheck,
    manager: BriefcaseBusiness,
    agent: Headset,
} as const;

function initials(name: string) {
    return name
        .split(/\s+/)
        .map((p) => p[0])
        .slice(0, 2)
        .join('')
        .toUpperCase();
}

/** The account card at the top of the profile and security pages. */
export async function ProfileHero({
    user,
    prefs,
}: {
    user: User;
    prefs: DisplayPrefs;
}) {
    const t = await getT();
    const RoleIcon =
        ROLE_ICON[user.role.key as keyof typeof ROLE_ICON] ?? ShieldCheck;
    const chip =
        'flex items-center gap-1.5 rounded-full border bg-background/70 px-3 py-1 text-xs backdrop-blur';
    return (
        <div className="relative mb-6 overflow-hidden rounded-3xl border bg-card shadow-xs">
            <div className="h-24 bg-linear-to-r from-primary/25 via-chart-2/20 to-chart-4/20" />
            <div className="flex flex-col gap-4 px-6 pb-6 sm:flex-row sm:items-end">
                <div className="-mt-12 flex size-24 shrink-0 items-center justify-center rounded-3xl border-4 border-card bg-primary text-3xl font-semibold text-primary-foreground shadow-lg">
                    {initials(user.name)}
                </div>
                <div className="min-w-0 flex-1 space-y-2">
                    <div>
                        <h2 className="truncate text-2xl font-semibold tracking-tight">
                            {user.name}
                        </h2>
                        <p className="truncate text-sm text-muted-foreground">
                            {user.email}
                        </p>
                    </div>
                    <div className="flex flex-wrap gap-2">
                        <span
                            className={cn(
                                chip,
                                'border-primary/30 text-primary',
                            )}
                        >
                            <RoleIcon className="size-3.5" />
                            {roleLabel(t, user.role)}
                        </span>
                        <Link
                            href="/account/security"
                            className={cn(
                                chip,
                                user.mfa_enabled
                                    ? 'border-success/30 text-success'
                                    : 'border-warning/40 text-warning hover:bg-warning/10',
                            )}
                        >
                            {user.mfa_enabled ? (
                                <ShieldCheck className="size-3.5" />
                            ) : (
                                <ShieldAlert className="size-3.5" />
                            )}
                            {user.mfa_enabled
                                ? t('account.profile.twoFactorOn')
                                : t('account.profile.twoFactorOff')}
                        </Link>
                        <span className={cn(chip, 'text-muted-foreground')}>
                            <CalendarDays className="size-3.5" />
                            {t('account.profile.memberSince', {
                                date: formatDate(user.created_at, prefs),
                            })}
                        </span>
                        {user.last_login_at && (
                            <span className={cn(chip, 'text-muted-foreground')}>
                                <Clock className="size-3.5" />
                                {t('account.profile.lastSignIn', {
                                    date: formatDateTime(
                                        user.last_login_at,
                                        prefs,
                                    ),
                                })}
                            </span>
                        )}
                    </div>
                </div>
            </div>
        </div>
    );
}
