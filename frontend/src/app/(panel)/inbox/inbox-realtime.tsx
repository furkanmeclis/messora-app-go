'use client';

import {
    RESYNCED,
    useDebouncedRefresh,
    useRealtimeEvents,
} from '@/components/realtime';
import { isInboxChannel } from '@/lib/realtime-sync';

/**
 * Keeps the inbox current: the list and the open thread re-render when a
 * conversation changes or a message arrives. Delivery ticks (message.status)
 * change only the thread they belong to, so ticks of other conversations,
 * such as a campaign's read receipts, are ignored.
 */
export function InboxRealtime({ selected }: { selected?: string }) {
    const refresh = useDebouncedRefresh();
    useRealtimeEvents((channel, e) => {
        if (!isInboxChannel(channel)) return;
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
