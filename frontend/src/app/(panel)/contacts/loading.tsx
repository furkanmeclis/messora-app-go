import { Card, CardContent } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';

/** Placeholder while a contacts page loads. */
export default function ContactsLoading() {
    return (
        <div aria-busy="true" aria-label="Loading">
            <div className="mb-6 flex flex-wrap items-end justify-between gap-4">
                <div className="space-y-2">
                    <Skeleton className="h-7 w-40" />
                    <Skeleton className="h-4 w-72" />
                </div>
                <Skeleton className="h-8 w-32" />
            </div>
            <Card>
                <CardContent className="space-y-3">
                    {Array.from({ length: 8 }, (_, i) => (
                        <div key={i} className="flex items-center gap-4">
                            <Skeleton className="size-4" />
                            <Skeleton className="h-4 flex-1" />
                            <Skeleton className="h-4 w-32" />
                            <Skeleton className="h-4 w-24" />
                        </div>
                    ))}
                </CardContent>
            </Card>
        </div>
    );
}
