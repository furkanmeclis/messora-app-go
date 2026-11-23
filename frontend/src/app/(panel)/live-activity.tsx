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
import {
    useRealtimeEvents,
    useRealtimeState,
    type RealtimeEvent,
} from '@/components/realtime';

type Item = { key: string; at: Date; text: string };

const MAX_ITEMS = 8;

function describe(e: RealtimeEvent): string | null {
    const dir = e.direction === 'inbound' ? 'Incoming' : 'Outgoing';
    const kind = typeof e.message_type === 'string' ? e.message_type : '';
    switch (e.type) {
        case 'message.created':
            return `${dir} ${kind} message · ${String(e.status)}`;
        case 'message.status':
            return `Message ${String(e.status)}`;
        case 'campaign.updated':
            return `Campaign ${String(e.status)}`;
        default:
            return null;
    }
}

const STATE_LABEL = {
    connected: 'Live',
    connecting: 'Connecting…',
    disconnected: 'Offline',
} as const;

/**
 * Message and campaign activity as it happens, straight from the realtime
 * connection (no polling). Events carry no message content.
 */
export function LiveActivity() {
    const state = useRealtimeState();
    const [items, setItems] = useState<Item[]>([]);

    useRealtimeEvents((channel, e) => {
        const text = describe(e);
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
                        Live activity
                    </CardTitle>
                    <CardDescription>
                        New messages and delivery updates appear here as they
                        happen.
                    </CardDescription>
                </div>
                <Badge
                    variant={
                        state === 'disconnected' ? 'destructive' : 'secondary'
                    }
                    aria-live="polite"
                    data-testid="realtime-state"
                >
                    {STATE_LABEL[state]}
                </Badge>
            </CardHeader>
            <CardContent>
                {items.length === 0 ? (
                    <p className="text-sm text-muted-foreground">
                        No activity since this page was opened.
                    </p>
                ) : (
                    <ul
                        className="space-y-1 text-sm"
                        aria-label="Recent activity"
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
                                    {i.at.toLocaleTimeString()}
                                </time>
                            </li>
                        ))}
                    </ul>
                )}
            </CardContent>
        </Card>
    );
}
