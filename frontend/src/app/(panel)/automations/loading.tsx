import { Skeleton } from '@/components/ui/skeleton';

/** Shown while an automation screen loads. */
export default function AutomationsLoading() {
    return (
        <div className="space-y-4" aria-busy="true">
            <span className="sr-only">Loading…</span>
            <Skeleton className="h-8 w-48" />
            <Skeleton className="h-4 w-80" />
            <Skeleton className="h-40 w-full" />
            <Skeleton className="h-40 w-full" />
        </div>
    );
}
