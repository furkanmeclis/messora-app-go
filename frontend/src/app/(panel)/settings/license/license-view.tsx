'use client';

import { useState, useTransition } from 'react';
import { useRouter } from 'next/navigation';
import {
    BadgeCheck,
    CalendarClock,
    Check,
    Copy,
    Download,
    KeyRound,
    ShieldAlert,
    Users,
} from 'lucide-react';
import {
    useFormatDate,
    useFormatDateTime,
} from '@/components/display-preferences';
import { notify } from '@/components/notify';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Textarea } from '@/components/ui/textarea';
import { useT } from '@/i18n/client';
import type { Schemas } from '@/lib/types';
import { cn } from '@/lib/utils';
import { Section } from '../section';
import { setLicenseAction } from './actions';

type Info = Schemas['LicenseInfo'];

const TONE: Record<Info['status'], string> = {
    trial: 'bg-primary/10 text-primary',
    active: 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400',
    expiring: 'bg-amber-500/10 text-amber-600 dark:text-amber-400',
    grace: 'bg-amber-500/10 text-amber-600 dark:text-amber-400',
    restricted: 'bg-destructive/10 text-destructive',
};

export function LicenseView({
    info,
    isOwner,
}: {
    info: Info;
    isOwner: boolean;
}) {
    const t = useT();
    const router = useRouter();
    const date = useFormatDate();
    const dateTime = useFormatDateTime();
    const [key, setKey] = useState('');
    const [saving, start] = useTransition();
    const [copied, setCopied] = useState(false);
    const lic = info.license;
    const rel = info.latest_release;

    const activate = () =>
        start(async () => {
            const r = await setLicenseAction(key.trim());
            if (notify(r)) {
                setKey('');
                router.refresh();
            }
        });

    const copy = async () => {
        try {
            await navigator.clipboard.writeText(info.instance_id);
            setCopied(true);
            setTimeout(() => setCopied(false), 1500);
        } catch {
            /* clipboard blocked: the id stays selectable */
        }
    };

    const facts: { icon: typeof Users; label: string; value: string }[] = lic
        ? [
              {
                  icon: BadgeCheck,
                  label: t('license.customer'),
                  value: lic.customer,
              },
              { icon: KeyRound, label: t('license.plan'), value: lic.plan },
              {
                  icon: CalendarClock,
                  label: t('license.expiresAt'),
                  value: lic.expires_at
                      ? date(lic.expires_at)
                      : t('license.perpetual'),
              },
              {
                  icon: Download,
                  label: t('license.updatesUntil'),
                  value: lic.updates_until
                      ? date(lic.updates_until)
                      : t('license.allUpdates'),
              },
              {
                  icon: ShieldAlert,
                  label: t('license.checkIn'),
                  value:
                      lic.check_in === 'online'
                          ? t('license.checkInOnline')
                          : t('license.checkInOffline'),
              },
              {
                  icon: Users,
                  label: t('license.limits'),
                  value:
                      [
                          lic.max_users
                              ? t('license.users', { count: lic.max_users })
                              : '',
                          lic.max_phone_numbers
                              ? t('license.numbers', {
                                    count: lic.max_phone_numbers,
                                })
                              : '',
                      ]
                          .filter(Boolean)
                          .join(' · ') || t('license.unlimited'),
              },
          ]
        : [];

    return (
        <div className="grid items-start gap-6 xl:grid-cols-[minmax(0,1fr)_22rem]">
            <div className="space-y-6">
                <section className="rounded-xl border bg-card p-5">
                    <div className="flex flex-wrap items-start justify-between gap-3">
                        <div>
                            <h2 className="text-base font-semibold">
                                {t('license.title')}
                            </h2>
                            <p className="mt-1 text-sm text-muted-foreground">
                                {t('license.description')}
                            </p>
                        </div>
                        <Badge className={cn('h-6 px-2.5', TONE[info.status])}>
                            {t(`license.status.${info.status}`)}
                        </Badge>
                    </div>
                    {(info.reason || info.until) && (
                        <p className="mt-4 rounded-lg bg-muted/40 px-3 py-2 text-sm">
                            {info.reason &&
                                t(
                                    `license.reason.${info.reason as 'expired'}`,
                                )}{' '}
                            {info.until &&
                                t(`license.until.${info.status}`, {
                                    date: date(info.until),
                                })}
                        </p>
                    )}
                    {lic ? (
                        <dl className="mt-5 grid gap-3 sm:grid-cols-2">
                            {facts.map(({ icon: Icon, label, value }) => (
                                <div
                                    key={label}
                                    className="flex gap-3 rounded-lg border p-3"
                                >
                                    <Icon className="mt-0.5 size-4 shrink-0 text-muted-foreground" />
                                    <div className="min-w-0">
                                        <dt className="text-xs text-muted-foreground">
                                            {label}
                                        </dt>
                                        <dd className="truncate text-sm font-medium">
                                            {value}
                                        </dd>
                                    </div>
                                </div>
                            ))}
                        </dl>
                    ) : (
                        <p className="mt-5 text-sm text-muted-foreground">
                            {t('license.noLicense')}
                        </p>
                    )}
                    {lic?.check_in === 'online' && (
                        <p className="mt-3 text-xs text-muted-foreground">
                            {info.last_check_in_at
                                ? t('license.lastCheckIn', {
                                      date: dateTime(info.last_check_in_at),
                                  })
                                : t('license.neverCheckedIn')}
                        </p>
                    )}
                </section>

                <Section
                    title={t('license.keyTitle')}
                    description={
                        isOwner
                            ? t('license.keyDescription')
                            : t('license.ownerOnly')
                    }
                    readOnly={!isOwner}
                >
                    <div className="space-y-3">
                        <Textarea
                            value={key}
                            onChange={(e) => setKey(e.target.value)}
                            placeholder={t('license.keyPlaceholder')}
                            disabled={!isOwner || saving}
                            rows={4}
                            spellCheck={false}
                            className="font-mono text-xs break-all"
                        />
                        <div className="flex justify-end">
                            <Button
                                onClick={activate}
                                disabled={
                                    !isOwner || saving || key.trim().length < 20
                                }
                            >
                                <KeyRound /> {t('license.keySave')}
                            </Button>
                        </div>
                    </div>
                </Section>
            </div>

            <aside className="space-y-6">
                <section className="rounded-xl border bg-card p-5">
                    <h3 className="text-sm font-semibold">
                        {t('license.update.title')}
                    </h3>
                    <p className="mt-2 text-sm text-muted-foreground">
                        {t('license.update.current', {
                            version: info.current_version,
                        })}
                    </p>
                    {rel ? (
                        <div className="mt-3 rounded-lg border p-3">
                            <p className="text-sm font-medium">
                                {t('license.update.latest', {
                                    version: rel.version,
                                })}
                            </p>
                            <p
                                className={cn(
                                    'mt-1 text-xs',
                                    rel.update_allowed
                                        ? 'text-emerald-600 dark:text-emerald-400'
                                        : 'text-amber-600 dark:text-amber-400',
                                )}
                            >
                                {rel.update_allowed
                                    ? t('license.update.included')
                                    : t('license.update.notIncluded')}
                            </p>
                        </div>
                    ) : (
                        <p className="mt-2 text-xs text-muted-foreground">
                            {t('license.update.none')}
                        </p>
                    )}
                </section>
                <section className="rounded-xl border bg-card p-5">
                    <h3 className="text-sm font-semibold">
                        {t('license.instance')}
                    </h3>
                    <p className="mt-1 text-xs text-muted-foreground">
                        {t('license.instanceHint')}
                    </p>
                    <div className="mt-3 flex items-center gap-2">
                        <code className="min-w-0 flex-1 truncate rounded-md bg-muted px-2 py-1.5 font-mono text-xs select-all">
                            {info.instance_id}
                        </code>
                        <Button variant="outline" size="sm" onClick={copy}>
                            {copied ? <Check /> : <Copy />}
                            {copied ? t('license.copied') : t('license.copy')}
                        </Button>
                    </div>
                </section>
            </aside>
        </div>
    );
}
