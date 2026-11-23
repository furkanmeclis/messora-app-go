import { Skeleton } from '@/components/ui/skeleton';
import { LoadingLabel } from './automations-client';

/** Shown while an automation screen loads. */
export default function AutomationsLoading() {
    return (
        <div className="space-y-4" aria-busy="true">
            <LoadingLabel />
            <Skeleton className="h-8 w-48" />
            <Skeleton className="h-4 w-80" />
            <Skeleton className="h-40 w-full" />
            <Skeleton className="h-40 w-full" />
        </div>
    );
}
