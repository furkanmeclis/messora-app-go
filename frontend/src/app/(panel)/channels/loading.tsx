import { Skeleton } from '@/components/ui/skeleton';
import { getT } from '@/i18n/server';

export default async function Loading() {
    const t = await getT();
    return (
        <div
            className="space-y-6"
            role="status"
            aria-label={t('channels.loading')}
        >
            <div className="space-y-2">
                <Skeleton className="h-8 w-48" />
                <Skeleton className="h-4 w-80 max-w-full" />
            </div>
            <Skeleton className="h-40 w-full" />
            <Skeleton className="h-64 w-full" />
        </div>
    );
}
