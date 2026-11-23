import { SectionIcon } from './section-icon';
import { Separator } from '@/components/ui/separator';
import { SidebarTrigger } from '@/components/ui/sidebar';

export function PanelHeader() {
    return (
        <header className="flex h-14 shrink-0 items-center gap-2 border-b px-4">
            <SidebarTrigger className="-ml-1" />
            <Separator orientation="vertical" className="mr-2 h-4" />
        </header>
    );
}

export function PageHeading({
    title,
    description,
    actions,
    icon,
}: {
    title: string;
    description?: string;
    actions?: React.ReactNode;
    /** Overrides the section icon; false hides it. */
    icon?: React.ReactNode | false;
}) {
    return (
        <div className="mb-6 flex flex-wrap items-center justify-between gap-4">
            <div className="flex min-w-0 items-center gap-4">
                {icon === undefined ? (
                    <SectionIcon />
                ) : icon === false ? null : (
                    <span className="flex size-11 shrink-0 items-center justify-center rounded-2xl bg-linear-to-br from-primary/20 to-primary/5 text-primary ring-1 ring-primary/15 [&_svg]:size-5">
                        {icon}
                    </span>
                )}
                <div className="min-w-0">
                    <h1 className="truncate text-2xl font-semibold tracking-tight">
                        {title}
                    </h1>
                    {description && (
                        <p className="mt-0.5 text-sm text-muted-foreground">
                            {description}
                        </p>
                    )}
                </div>
            </div>
            {actions && (
                <div className="flex flex-wrap items-center gap-2">
                    {actions}
                </div>
            )}
        </div>
    );
}
