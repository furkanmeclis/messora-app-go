'use client';

import { useTransition } from 'react';
import { Globe, LogOut, Monitor, Smartphone, Tablet } from 'lucide-react';
import { useConfirm } from '@/components/confirm';
import { cn } from '@/lib/utils';
import { notify } from '@/components/notify';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { describeUserAgent } from '@/lib/format';
import { useFormatDateTime } from '@/components/display-preferences';
import { useT } from '@/i18n/client';
import type { SessionInfo } from '@/lib/types';
import { revokeOtherSessionsAction, revokeSessionAction } from '../../actions';

export function SessionList({ sessions }: { sessions: SessionInfo[] }) {
    const t = useT();
    const formatDateTime = useFormatDateTime();
    const [pending, startTransition] = useTransition();
    const others = sessions.filter((s) => !s.current).length;
    const ask = useConfirm();
    const deviceIcon = (ua: string | null | undefined) =>
        /iPad|Tablet/i.test(ua ?? '')
            ? Tablet
            : /Mobile|Android|iPhone/i.test(ua ?? '')
              ? Smartphone
              : Monitor;
    return (
        <div className="space-y-4">
            <ul className="grid gap-2">
                {sessions.map((s) => {
                    const Device = deviceIcon(s.user_agent);
                    return (
                        <li
                            key={s.id}
                            className={cn(
                                'flex items-center gap-3 rounded-xl border p-3 transition-colors',
                                s.current && 'border-primary/30 bg-primary/5',
                            )}
                        >
                            <span
                                className={cn(
                                    'flex size-10 shrink-0 items-center justify-center rounded-xl',
                                    s.current
                                        ? 'bg-primary/15 text-primary'
                                        : 'bg-muted text-muted-foreground',
                                )}
                            >
                                <Device className="size-5" />
                            </span>
                            <div className="min-w-0 flex-1 text-sm">
                                <div className="flex items-center gap-2 font-medium">
                                    {describeUserAgent(s.user_agent, {
                                        unknown: t(
                                            'account.sessions.unknownDevice',
                                        ),
                                        browser: t('account.sessions.browser'),
                                        on: (browser, os) =>
                                            t('account.sessions.browserOn', {
                                                browser,
                                                os,
                                            }),
                                    })}
                                    {s.current && (
                                        <Badge variant="secondary">
                                            {t('account.sessions.thisDevice')}
                                        </Badge>
                                    )}
                                </div>
                                <div className="flex items-center gap-1.5 truncate text-muted-foreground">
                                    <Globe className="size-3.5 shrink-0" />
                                    {t('account.sessions.lastActive', {
                                        ip:
                                            s.ip ??
                                            t(
                                                'account.sessions.unknownAddress',
                                            ),
                                        time: formatDateTime(s.last_seen_at),
                                    })}
                                </div>
                            </div>
                            {!s.current && (
                                <Button
                                    size="sm"
                                    variant="outline"
                                    disabled={pending}
                                    onClick={() =>
                                        startTransition(
                                            async () =>
                                                void notify(
                                                    await revokeSessionAction(
                                                        s.id,
                                                    ),
                                                ),
                                        )
                                    }
                                >
                                    <LogOut />
                                    {t('account.sessions.signOut')}
                                </Button>
                            )}
                        </li>
                    );
                })}
            </ul>
            {others > 0 && (
                <Button
                    variant="destructive"
                    disabled={pending}
                    onClick={() =>
                        startTransition(async () => {
                            if (
                                !(await ask({
                                    destructive: true,
                                    title: t('account.sessions.signOutOthers'),
                                    confirmLabel: t(
                                        'account.sessions.signOutOthers',
                                    ),
                                }))
                            )
                                return;
                            notify(await revokeOtherSessionsAction());
                        })
                    }
                >
                    <LogOut />
                    {t('account.sessions.signOutOthers')}
                </Button>
            )}
        </div>
    );
}
