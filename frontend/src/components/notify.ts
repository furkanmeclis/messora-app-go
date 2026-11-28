'use client';

import { toast } from 'sonner';
import type { ActionResult } from '@/lib/types';

/** Shows an action's outcome as a toast; returns whether it succeeded. */
export function notify<T>(
    r: ActionResult<T>,
): r is { ok: true; data: T; message?: string } {
    if (r.ok) {
        if (r.message) toast.success(r.message);
        return true;
    }
    const up = r.upgrade;
    toast.error(
        r.message,
        up
            ? {
                  action: {
                      label: up.label,
                      onClick: () => window.location.assign(up.href),
                  },
              }
            : undefined,
    );
    return false;
}

/** Fresh Idempotency-Key per user intent (retries of one submit share it). */
export function newIdempotencyKey(): string {
    return crypto.randomUUID();
}
