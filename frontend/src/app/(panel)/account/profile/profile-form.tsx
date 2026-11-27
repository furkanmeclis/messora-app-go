'use client';

import { useEffect, useState, useTransition } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { useTheme } from 'next-themes';
import {
    ChevronRight,
    Clock,
    KeyRound,
    Keyboard,
    Laptop,
    Mail,
    Monitor,
    Moon,
    ShieldCheck,
    Sun,
    UserRound,
} from 'lucide-react';
import { Input } from '@/components/ui/input';
import { Segmented } from '@/components/ui/segmented';
import { notify } from '@/components/notify';
import { useT } from '@/i18n/client';
import type { Locale } from '@/i18n/config';
import { formatTime, prefsFor } from '@/lib/format';
import type { User } from '@/lib/types';
import { updateProfileAction } from '../../actions';
import { LocaleField, TimezoneField } from '../../settings/fields';
import { EmailChange } from './email-change';
import { Row, Section } from '../../settings/section';

export function ProfileForm({ user }: { user: User }) {
    const t = useT();
    const router = useRouter();
    const initial = {
        name: user.name,
        timezone: user.timezone,
        locale: (user.locale === 'tr' ? 'tr' : 'en') as Locale,
    };
    const [saved, setSaved] = useState(initial);
    const [v, setV] = useState(initial);
    const [saving, start] = useTransition();
    const [now, setNow] = useState<string | null>(null);

    useEffect(() => {
        const tick = () => setNow(new Date().toISOString());
        tick();
        const id = setInterval(tick, 30_000);
        return () => clearInterval(id);
    }, []);

    const save = () =>
        start(async () => {
            const form = new FormData();
            form.set('name', v.name.trim());
            form.set('timezone', v.timezone);
            form.set('locale', v.locale);
            const r = await updateProfileAction(null, form);
            if (notify(r)) {
                setSaved(v);
                router.refresh();
            }
        });
    const dirtyPersonal = v.name !== saved.name;
    const dirtyRegional =
        v.timezone !== saved.timezone || v.locale !== saved.locale;

    return (
        <div className="grid gap-6">
            <Section
                title={
                    <span className="flex items-center gap-2">
                        <UserRound className="size-4 text-primary" />
                        {t('account.profile.personalDetails')}
                    </span>
                }
                dirty={dirtyPersonal}
                saving={saving}
                onSave={save}
                onReset={() => setV({ ...v, name: saved.name })}
            >
                <div className="grid gap-5 md:grid-cols-2">
                    <Row
                        label={t('account.profile.name')}
                        htmlFor="profile-name"
                    >
                        <Input
                            id="profile-name"
                            value={v.name}
                            maxLength={120}
                            autoComplete="name"
                            onChange={(e) =>
                                setV({ ...v, name: e.target.value })
                            }
                        />
                    </Row>
                    <Row
                        label={t('account.profile.email')}
                        hint={t('account.profile.emailHint')}
                        htmlFor="profile-email"
                    >
                        <div className="relative">
                            <Mail className="pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2 text-muted-foreground" />
                            <Input
                                id="profile-email"
                                value={user.email}
                                readOnly
                                className="pl-8 text-muted-foreground"
                            />
                        </div>
                        <EmailChange current={user.email} />
                    </Row>
                </div>
            </Section>
            <Section
                title={
                    <span className="flex items-center gap-2">
                        <Clock className="size-4 text-primary" />
                        {t('account.profile.regional')}
                    </span>
                }
                description={t('account.profile.regionalHint')}
                dirty={dirtyRegional}
                saving={saving}
                onSave={save}
                onReset={() =>
                    setV({
                        ...v,
                        timezone: saved.timezone,
                        locale: saved.locale,
                    })
                }
            >
                <Row label={t('account.profile.language')}>
                    <LocaleField
                        value={v.locale}
                        onChange={(locale) => setV({ ...v, locale })}
                    />
                </Row>
                <Row
                    label={t('account.profile.timezone')}
                    hint={
                        now
                            ? t('account.profile.localTime', {
                                  time: formatTime(
                                      now,
                                      prefsFor(v.timezone, v.locale),
                                  ),
                              })
                            : undefined
                    }
                >
                    <TimezoneField
                        value={v.timezone}
                        onChange={(timezone) => setV({ ...v, timezone })}
                    />
                </Row>
            </Section>
        </div>
    );
}

/** Side column: personal colour scheme and links to security tasks. */
export function AccountAside({
    permissionCount,
    mfaEnabled,
}: {
    permissionCount: number;
    mfaEnabled: boolean;
}) {
    const t = useT();
    const { theme, setTheme } = useTheme();
    const link = (
        href: string,
        Icon: typeof KeyRound,
        label: string,
        hint?: React.ReactNode,
    ) => (
        <Link
            href={href}
            className="flex items-center gap-3 rounded-xl px-3 py-2.5 text-sm transition-colors hover:bg-accent"
        >
            <span className="flex size-8 items-center justify-center rounded-lg bg-muted text-muted-foreground">
                <Icon className="size-4" />
            </span>
            <span className="min-w-0 flex-1">
                <span className="block font-medium">{label}</span>
                {hint && (
                    <span className="block text-xs text-muted-foreground">
                        {hint}
                    </span>
                )}
            </span>
            <ChevronRight className="size-4 text-muted-foreground" />
        </Link>
    );
    return (
        <aside className="space-y-4">
            <div className="space-y-3 rounded-2xl border bg-card p-4 shadow-xs">
                <div>
                    <h3 className="text-sm font-semibold">
                        {t('account.profile.personalTheme')}
                    </h3>
                    <p className="text-xs text-muted-foreground">
                        {t('account.profile.personalThemeHint')}
                    </p>
                </div>
                <Segmented
                    value={theme ?? 'system'}
                    onValueChange={setTheme}
                    className="w-full [&>*]:flex-1 [&>*]:justify-center"
                    options={[
                        {
                            value: 'light',
                            label: t('common.light'),
                            icon: <Sun />,
                        },
                        {
                            value: 'dark',
                            label: t('common.dark'),
                            icon: <Moon />,
                        },
                        {
                            value: 'system',
                            label: t('common.system'),
                            icon: <Monitor />,
                        },
                    ]}
                />
            </div>
            <div className="rounded-2xl border bg-card p-2 shadow-xs">
                <p className="px-3 pt-2 pb-1 text-xs font-medium tracking-wide text-muted-foreground uppercase">
                    {t('account.profile.quickLinks')}
                </p>
                {link(
                    '/account/security#password',
                    KeyRound,
                    t('account.profile.changePassword'),
                )}
                {link(
                    '/account/security#two-factor',
                    ShieldCheck,
                    t('account.profile.manageTwoFactor'),
                    mfaEnabled
                        ? t('account.profile.twoFactorOn')
                        : t('account.profile.twoFactorOff'),
                )}
                {link(
                    '/account/security#sessions',
                    Laptop,
                    t('account.profile.manageSessions'),
                )}
            </div>
            <p className="flex items-center gap-2 px-1 text-xs text-muted-foreground">
                <Keyboard className="size-3.5" />
                {t('account.profile.keyboardHint')}
                <span className="sr-only">
                    {t('account.profile.permissions', {
                        count: permissionCount,
                    })}
                </span>
            </p>
        </aside>
    );
}
