'use client';

import { useTransition } from 'react';
import { Monitor } from 'lucide-react';
import { notify } from '@/components/notify';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { describeUserAgent } from '@/lib/format';
import { useFormatDateTime } from '@/components/display-preferences';
import type { SessionInfo } from '@/lib/types';
import { revokeOtherSessionsAction, revokeSessionAction } from '../../actions';

export function SessionList({ sessions }: { sessions: SessionInfo[] }) {
    const formatDateTime = useFormatDateTime();
    const [pending, startTransition] = useTransition();
    const others = sessions.filter((s) => !s.current).length;
    return (
        <div className="space-y-4">
            <ul className="divide-y rounded-md border">
                {sessions.map((s) => (
                    <li key={s.id} className="flex items-center gap-3 p-3">
                        <Monitor className="size-5 shrink-0 text-muted-foreground" />
                        <div className="min-w-0 flex-1 text-sm">
                            <div className="flex items-center gap-2 font-medium">
                                {describeUserAgent(s.user_agent)}
                                {s.current && (
                                    <Badge variant="secondary">
                                        This device
                                    </Badge>
                                )}
                            </div>
                            <div className="truncate text-muted-foreground">
                                {s.ip ?? 'Unknown address'} · last active{' '}
                                {formatDateTime(s.last_seen_at)}
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
                                                await revokeSessionAction(s.id),
                                            ),
                                    )
                                }
                            >
                                Sign out
                            </Button>
                        )}
                    </li>
                ))}
            </ul>
            {others > 0 && (
                <Button
                    variant="outline"
                    disabled={pending}
                    onClick={() =>
                        startTransition(
                            async () =>
                                void notify(await revokeOtherSessionsAction()),
                        )
                    }
                >
                    Sign out all other sessions
                </Button>
            )}
        </div>
    );
}
