import { MessagesSquare } from 'lucide-react';
import { cn } from '@/lib/utils';

export function BrandMark({ className }: { className?: string }) {
    return (
        <div
            className={cn(
                'flex size-9 items-center justify-center rounded-lg bg-primary text-primary-foreground',
                className,
            )}
        >
            <MessagesSquare className="size-5" aria-hidden />
        </div>
    );
}

export function Brand() {
    return (
        <div className="flex items-center gap-2">
            <BrandMark />
            <span className="text-lg font-semibold tracking-tight">
                Messora
            </span>
        </div>
    );
}
