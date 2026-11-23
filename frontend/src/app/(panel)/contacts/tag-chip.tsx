import { Badge } from '@/components/ui/badge';
import { cn } from '@/lib/utils';

/** Colours a tag may have (the API accepts any lowercase word). */
export const TAG_COLORS: Record<string, string> = {
    gray: 'bg-zinc-400',
    red: 'bg-red-500',
    orange: 'bg-orange-500',
    amber: 'bg-amber-500',
    yellow: 'bg-yellow-400',
    green: 'bg-green-500',
    teal: 'bg-teal-500',
    cyan: 'bg-cyan-500',
    blue: 'bg-blue-500',
    indigo: 'bg-indigo-500',
    violet: 'bg-violet-500',
    pink: 'bg-pink-500',
};

export function TagDot({ color }: { color: string }) {
    return (
        <span
            aria-hidden
            className={cn(
                'inline-block size-2 shrink-0 rounded-full',
                TAG_COLORS[color] ?? TAG_COLORS.gray,
            )}
        />
    );
}

export function TagChip({
    name,
    color,
    children,
}: {
    name: string;
    color: string;
    children?: React.ReactNode;
}) {
    return (
        <Badge variant="outline" className="gap-1.5">
            <TagDot color={color} />
            {name}
            {children}
        </Badge>
    );
}
