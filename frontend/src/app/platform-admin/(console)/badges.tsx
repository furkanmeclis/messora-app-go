'use client';

import { Badge } from '@/components/ui/badge';
import { useT } from '@/i18n/client';
import type { Schemas } from '@/lib/types';
import { cn } from '@/lib/utils';

export function TenantStatusBadge({
    status,
}: {
    status: Schemas['TenantStatus'];
}) {
    const t = useT();
    return (
        <Badge
            variant="outline"
            className={cn(
                status === 'active' &&
                    'border-success/40 bg-success/10 text-success',
                status === 'suspended' &&
                    'border-destructive/40 bg-destructive/10 text-destructive',
            )}
        >
            {t(`platform.tenants.status.${status}`)}
        </Badge>
    );
}

export function SubscriptionBadge({
    status,
}: {
    status: Schemas['SubscriptionStatus'];
}) {
    const t = useT();
    return (
        <Badge
            variant="outline"
            className={cn(
                status === 'trial' &&
                    'border-warning/40 bg-warning/10 text-warning',
            )}
        >
            {t(`platform.tenants.subscription.${status}`)}
        </Badge>
    );
}
