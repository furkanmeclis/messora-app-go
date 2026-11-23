'use client';

import { Radio } from 'lucide-react';
import { useState } from 'react';
import { Badge } from '@/components/ui/badge';
import {
    Card,
    CardContent,
    CardDescription,
    CardHeader,
    CardTitle,
} from '@/components/ui/card';
import { useDisplayPrefs } from '@/components/display-preferences';
import {
    useRealtimeEvents,
    useRealtimeState,
    type RealtimeEvent,
} from '@/components/realtime';
import { useT } from '@/i18n/client';
import type { MessageKey, Translate } from '@/i18n/translate';
import { formatTime } from '@/lib/format';

type Item = { key: string; at: Date; text: string };

const MAX_ITEMS = 8;

/** A translated label for an API value, or the value itself. */
function label(t: Translate, group: string, raw: unknown): string {
    const value = String(raw ?? '');
    const key = `overview.live.${group}.${value}` as MessageKey;
    const text = t(key);
    return text === key ? value : text;
}

function describe(t: Translate, e: RealtimeEvent): string | null {
    const kind = typeof e.message_type === 'string' ? e.message_type : '';
    switch (e.type) {
        case 'message.created':
            return t(
                e.direction === 'inbound'
                    ? 'overview.live.incoming'
                    : 'overview.live.outgoing',
                {
                    kind: label(t, 'kinds', kind),
                    status: label(t, 'messageStatus', e.status),
                },
            );
        case 'message.status':
            return t('overview.live.message', {
                status: label(t, 'messageStatus', e.status),
            });
        case 'campaign.updated':
            return t('overview.live.campaign', {
                status: label(t, 'campaignStatus', e.status),
            });
        default:
            return null;
    }
}

const STATE_LABEL = {
    connected: 'overview.live.connected',
    connecting: 'overview.live.connecting',
    disconnected: 'overview.live.disconnected',
} as const;

/**
 * Message and campaign activity as it happens, straight from the realtime
 * connection (no polling). Events carry no message content.
 */
export function LiveActivity() {
    const t = useT();
    const prefs = useDisplayPrefs();
    const state = useRealtimeState();
    const [items, setItems] = useState<Item[]>([]);

    useRealtimeEvents((channel, e) => {
        const text = describe(t, e);
        if (!text) return;
        const id =
            typeof e.message_id === 'string'
                ? e.message_id
                : String(e.campaign_id ?? '');
        setItems((prev) =>
            [
                {
                    key: `${channel}:${e.type}:${id}:${Date.now()}`,
                    at: new Date(),
                    text,
                },
                ...prev,
            ].slice(0, MAX_ITEMS),
        );
    });

    return (
        <Card className="mb-6">
            <CardHeader className="flex flex-row items-start justify-between gap-2">
                <div className="space-y-1">
                    <CardTitle className="flex items-center gap-2 text-base">
                        <Radio className="size-4 text-muted-foreground" />
                        {t('overview.live.title')}
                    </CardTitle>
                    <CardDescription>
                        {t('overview.live.description')}
                    </CardDescription>
                </div>
                <Badge
                    variant={
                        state === 'disconnected' ? 'destructive' : 'secondary'
                    }
                    aria-live="polite"
                    data-testid="realtime-state"
                >
                    {t(STATE_LABEL[state])}
                </Badge>
            </CardHeader>
            <CardContent>
                {items.length === 0 ? (
                    <p className="text-sm text-muted-foreground">
                        {t('overview.live.empty')}
                    </p>
                ) : (
                    <ul
                        className="space-y-1 text-sm"
                        aria-label={t('overview.live.recent')}
                    >
                        {items.map((i) => (
                            <li
                                key={i.key}
                                className="flex justify-between gap-4"
                            >
                                <span>{i.text}</span>
                                <time
                                    className="text-muted-foreground tabular-nums"
                                    dateTime={i.at.toISOString()}
                                >
                                    {formatTime(
                                        i.at.toISOString(),
                                        prefs,
                                        true,
                                    )}
                                </time>
                            </li>
                        ))}
                    </ul>
                )}
            </CardContent>
        </Card>
    );
}
