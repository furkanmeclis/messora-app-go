'use client';

import * as React from 'react';
import { cn } from 'cn';
import { FileUp, X } from 'lucide-react';
import { useT } from '@/i18n/client';

/**
 * Drop area for one file. The browser's file control stays hidden (it only
 * opens the system dialog); the area shows the chosen file.
 */
function FileDrop({
    accept,
    name,
    onFile,
    file,
    hint,
    disabled,
    className,
    inputRef,
    compact,
}: {
    accept?: string;
    name?: string;
    onFile?: (f: File | null) => void;
    /** The current file, when the parent keeps it. */
    file?: File | null;
    hint?: React.ReactNode;
    disabled?: boolean;
    className?: string;
    inputRef?: React.Ref<HTMLInputElement>;
    compact?: boolean;
}) {
    const t = useT();
    const local = React.useRef<HTMLInputElement>(null);
    const [over, setOver] = React.useState(false);
    const [inner, setInner] = React.useState<File | null>(null);
    const current = file === undefined ? inner : file;

    const set = (f: File | null) => {
        setInner(f);
        onFile?.(f);
        const input = local.current;
        if (input) {
            const dt = new DataTransfer();
            if (f) dt.items.add(f);
            input.files = dt.files;
        }
    };
    React.useImperativeHandle(
        inputRef,
        () => local.current as HTMLInputElement,
    );

    return (
        <div
            role="button"
            tabIndex={disabled ? -1 : 0}
            aria-disabled={disabled}
            onClick={() => !disabled && local.current?.click()}
            onKeyDown={(e) => {
                if ((e.key === 'Enter' || e.key === ' ') && !disabled) {
                    e.preventDefault();
                    local.current?.click();
                }
            }}
            onDragOver={(e) => {
                e.preventDefault();
                if (!disabled) setOver(true);
            }}
            onDragLeave={() => setOver(false)}
            onDrop={(e) => {
                e.preventDefault();
                setOver(false);
                const f = e.dataTransfer.files[0];
                if (f && !disabled) set(f);
            }}
            className={cn(
                'group relative flex cursor-pointer flex-col items-center justify-center gap-2 rounded-xl border-2 border-dashed border-input bg-muted/30 px-4 text-center transition-all outline-none hover:border-primary/50 hover:bg-accent/30 focus-visible:ring-3 focus-visible:ring-ring/50',
                compact ? 'py-4' : 'py-8',
                over && 'scale-[1.01] border-primary bg-accent/50',
                disabled && 'pointer-events-none opacity-50',
                className,
            )}
        >
            {current ? (
                <div className="flex max-w-full items-center gap-2 rounded-lg border bg-background px-3 py-2 text-sm shadow-sm">
                    <FileUp className="size-4 shrink-0 text-primary" />
                    <span className="truncate font-medium">{current.name}</span>
                    <span className="shrink-0 text-xs text-muted-foreground">
                        {(current.size / 1024).toFixed(
                            current.size > 102400 ? 0 : 1,
                        )}{' '}
                        KB
                    </span>
                    <button
                        type="button"
                        aria-label={t('common.remove')}
                        onClick={(e) => {
                            e.stopPropagation();
                            set(null);
                        }}
                        className="rounded p-0.5 text-muted-foreground hover:bg-muted hover:text-foreground"
                    >
                        <X className="size-3.5" />
                    </button>
                </div>
            ) : (
                <>
                    <span className="flex size-10 items-center justify-center rounded-full bg-primary/10 text-primary transition-transform group-hover:-translate-y-0.5">
                        <FileUp className="size-5" />
                    </span>
                    <p className="text-sm">
                        {t('common.dropFile', { action: '' }).replace(
                            /\s*$/,
                            ' ',
                        )}
                        <span className="font-medium text-primary underline-offset-4 group-hover:underline">
                            {t('common.browse')}
                        </span>
                    </p>
                </>
            )}
            {hint && <p className="text-xs text-muted-foreground">{hint}</p>}
            {/* The hidden system file dialog. */}
            <input
                ref={local}
                // eslint-disable-next-line no-restricted-syntax
                type="file"
                name={name}
                accept={accept}
                className="sr-only"
                tabIndex={-1}
                disabled={disabled}
                onClick={(e) => e.stopPropagation()}
                onChange={(e) => set(e.target.files?.[0] ?? null)}
            />
        </div>
    );
}

export { FileDrop };
