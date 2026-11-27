import { cn } from '@/lib/utils';

/**
 * The Messora mark (concept A, "m in a speech bubble"): two arches drawing
 * the letter inside an indigo bubble, with a mint dot for the AI. On dark
 * surfaces the bubble uses the brighter indigo so it keeps its contrast.
 */
export function MessoraMark({
    className,
    tone = 'auto',
    title,
}: {
    className?: string;
    /** "auto" follows the colour scheme; "dark" is for Night surfaces. */
    tone?: 'auto' | 'dark';
    /** Accessible name; the mark is decorative without one. */
    title?: string;
}) {
    return (
        <svg
            viewBox="0 0 64 64"
            className={cn('size-8 shrink-0', className)}
            role={title ? 'img' : undefined}
            aria-label={title}
            aria-hidden={title ? undefined : true}
            focusable="false"
        >
            <path
                d="M14 6h36a10 10 0 0 1 10 10v24a10 10 0 0 1-10 10H26l-12 9v-9a10 10 0 0 1-10-10V16A10 10 0 0 1 14 6z"
                className={
                    tone === 'dark'
                        ? 'fill-indigo-bright'
                        : 'fill-indigo-brand dark:fill-indigo-bright'
                }
            />
            <path
                d="M19 39V24a5 5 0 0 1 10 0v15M29 24a5 5 0 0 1 10 0v15M39 24a5 5 0 0 1 6-4.9"
                fill="none"
                stroke="#fff"
                strokeWidth="4.5"
                strokeLinecap="round"
            />
            <circle cx="47" cy="17" r="4" className="fill-mint" />
        </svg>
    );
}

/** Mark and "messora" wordmark side by side. */
export function MessoraLogo({
    className,
    tone = 'auto',
    size = 'md',
}: {
    className?: string;
    tone?: 'auto' | 'dark';
    size?: 'sm' | 'md' | 'lg';
}) {
    return (
        <span
            className={cn('inline-flex items-center gap-2.5', className)}
            aria-label="Messora"
            role="img"
        >
            <MessoraMark
                tone={tone}
                className={
                    size === 'lg'
                        ? 'size-10'
                        : size === 'sm'
                          ? 'size-7'
                          : 'size-8'
                }
            />
            <span
                aria-hidden
                className={cn(
                    'font-display leading-none font-extrabold tracking-[-0.03em]',
                    size === 'lg'
                        ? 'text-[26px]'
                        : size === 'sm'
                          ? 'text-lg'
                          : 'text-[22px]',
                    tone === 'dark' ? 'text-fog' : 'text-foreground',
                )}
            >
                messora
            </span>
        </span>
    );
}
