'use client';

import { Loader2 } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { useT } from '@/i18n/client';
import { cn } from '@/lib/utils';

/** A settings card with a save bar that appears when something changed. */
export function Section({
    title,
    description,
    children,
    dirty,
    saving,
    onSave,
    onReset,
    readOnly,
    className,
}: {
    title: React.ReactNode;
    description?: React.ReactNode;
    children: React.ReactNode;
    dirty?: boolean;
    saving?: boolean;
    onSave?: () => void;
    onReset?: () => void;
    readOnly?: boolean;
    className?: string;
}) {
    const t = useT();
    return (
        <section
            className={cn(
                'overflow-hidden rounded-2xl border bg-card shadow-xs',
                className,
            )}
        >
            <header className="border-b bg-muted/30 px-5 py-4">
                <h2 className="font-semibold tracking-tight">{title}</h2>
                {description && (
                    <p className="mt-1 text-sm text-muted-foreground">
                        {description}
                    </p>
                )}
            </header>
            <div className="space-y-6 px-5 py-5">{children}</div>
            {onSave && !readOnly && (
                <footer
                    className={cn(
                        'flex items-center justify-end gap-2 border-t px-5 py-3 transition-colors',
                        dirty ? 'bg-accent/40' : 'bg-transparent',
                    )}
                >
                    {dirty && (
                        <span className="mr-auto text-xs text-muted-foreground">
                            {t('common.unsaved')}
                        </span>
                    )}
                    {onReset && dirty && (
                        <Button
                            type="button"
                            variant="ghost"
                            size="sm"
                            onClick={onReset}
                            disabled={saving}
                        >
                            {t('common.discard')}
                        </Button>
                    )}
                    <Button
                        type="button"
                        size="sm"
                        onClick={onSave}
                        disabled={!dirty || saving}
                    >
                        {saving && <Loader2 className="animate-spin" />}
                        {saving ? t('common.saving') : t('common.save')}
                    </Button>
                </footer>
            )}
        </section>
    );
}

/** A labelled row inside a section. */
export function Row({
    label,
    hint,
    badge,
    htmlFor,
    children,
}: {
    label: React.ReactNode;
    hint?: React.ReactNode;
    badge?: React.ReactNode;
    htmlFor?: string;
    children: React.ReactNode;
}) {
    return (
        <div className="space-y-2">
            <div className="flex flex-wrap items-center gap-2">
                <label htmlFor={htmlFor} className="text-sm font-medium">
                    {label}
                </label>
                {badge}
            </div>
            {children}
            {hint && <p className="text-xs text-muted-foreground">{hint}</p>}
        </div>
    );
}
