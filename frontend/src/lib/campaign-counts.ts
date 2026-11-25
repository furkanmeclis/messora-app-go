export type Counts = Record<string, number>;

/**
 * Aggregates a campaign's counters: recipients_* (recipient states) and
 * message statuses.
 *
 * - skipped: recipients skipped when they were collected plus messages
 *   consent stopped right before sending (message status "skipped").
 * - deferred: messages Meta withheld for ecosystem health (131049); they
 *   are retried later and do not occupy the in-flight window, so they are
 *   neither in flight nor done.
 */
export function summarize(counts: Counts, total: number) {
    const n = (k: string) => counts[k] ?? 0;
    const recipients = Math.max(n('recipients'), total);
    const skipped = n('recipients_skipped') + n('skipped');
    const cancelled = n('recipients_cancelled') + n('cancelled');
    const delivered = n('delivered');
    const read = n('read');
    const sent = n('sent');
    const failed = n('failed');
    const unknown = n('unknown');
    const held = n('held');
    const deferred = n('deferred');
    const inFlight = n('pending') + n('sending');
    const waiting = n('recipients_pending');
    const done =
        skipped + cancelled + sent + delivered + read + failed + unknown;
    return {
        recipients,
        skipped,
        cancelled,
        sent,
        delivered,
        read,
        failed,
        unknown,
        held,
        deferred,
        inFlight,
        waiting,
        done: Math.min(done, recipients),
        percent:
            recipients > 0
                ? Math.min(100, Math.round((done / recipients) * 100))
                : 0,
    };
}
