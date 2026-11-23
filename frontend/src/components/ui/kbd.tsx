import { cn } from 'cn';

/** A keyboard key hint, e.g. <Kbd>N</Kbd>. */
function Kbd({ className, ...props }: React.ComponentProps<'kbd'>) {
    return (
        <kbd
            lang="en"
            data-slot="kbd"
            className={cn(
                'pointer-events-none inline-flex h-5 min-w-5 items-center justify-center rounded-md border border-current/20 bg-current/10 px-1 font-sans text-[10px] leading-none font-medium uppercase opacity-80 select-none',
                className,
            )}
            {...props}
        />
    );
}

export { Kbd };
