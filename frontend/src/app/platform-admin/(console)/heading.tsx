/** The title row of a platform admin page. */
export function Heading({
    title,
    description,
    icon,
    actions,
}: {
    title: string;
    description?: string;
    icon: React.ReactNode;
    actions?: React.ReactNode;
}) {
    return (
        <div className="mb-6 flex flex-wrap items-center justify-between gap-4">
            <div className="flex min-w-0 items-center gap-4">
                <span className="flex size-11 shrink-0 items-center justify-center rounded-2xl bg-linear-to-br from-primary/20 to-primary/5 text-primary ring-1 ring-primary/15 [&_svg]:size-5">
                    {icon}
                </span>
                <div className="min-w-0">
                    <h1 className="truncate text-2xl font-semibold tracking-tight">
                        {title}
                    </h1>
                    {description && (
                        <p className="text-sm text-muted-foreground">
                            {description}
                        </p>
                    )}
                </div>
            </div>
            {actions && <div className="flex gap-2">{actions}</div>}
        </div>
    );
}
