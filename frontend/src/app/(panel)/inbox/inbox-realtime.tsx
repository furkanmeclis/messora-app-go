'use client';

import {
    RESYNCED,
    useDebouncedRefresh,
    useRealtimeChannel,
    useRealtimeEvents,
} from '@/components/realtime';
import { isInboxChannel } from '@/lib/realtime-sync';

/**
 * Keeps the inbox current: the list and the open thread re-render when a
 * conversation changes or a message arrives. Delivery ticks (message.status)
 * change only the thread they belong to. The open thread's own channel is
 * subscribed too: campaign messages announce their ticks only there, so a
 * campaign's read receipts never reach the inbox channels.
 */
export function InboxRealtime({ selected }: { selected?: string }) {
    const refresh = useDebouncedRefresh();
    const own = selected ? `conversation:${selected}` : undefined;
    useRealtimeChannel(own);
    useRealtimeEvents((channel, e) => {
        if (!isInboxChannel(channel) && channel !== own) return;
        switch (e.type) {
            case 'conversation.updated':
            case 'message.created':
            case RESYNCED:
                refresh();
                break;
            case 'message.status':
                if (selected && e.conversation_id === selected) refresh();
                break;
        }
    });
    return null;
}
