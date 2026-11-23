'use client';

import * as React from 'react';
import { cn } from 'cn';
import { Minus, Plus } from 'lucide-react';

/**
 * Whole-number input with steppers. A text field with a numeric keyboard,
 * not the browser's number control; the value is submitted under `name`.
 */
function NumberField({
    name,
    value,
    defaultValue,
    onValueChange,
    min = Number.MIN_SAFE_INTEGER,
    max = Number.MAX_SAFE_INTEGER,
    step = 1,
    allowEmpty = false,
    className,
    id,
    disabled,
    required,
    placeholder,
    'aria-label': ariaLabel,
    'aria-invalid': ariaInvalid,
}: {
    name?: string;
    value?: number | null;
    defaultValue?: number | null;
    onValueChange?: (v: number | null) => void;
    min?: number;
    max?: number;
    step?: number;
    allowEmpty?: boolean;
    className?: string;
    id?: string;
    disabled?: boolean;
    required?: boolean;
    placeholder?: string;
    'aria-label'?: string;
    'aria-invalid'?: boolean;
}) {
    const controlled = value !== undefined;
    const [inner, setInner] = React.useState<number | null>(
        defaultValue ?? null,
    );
    const current = controlled ? value : inner;
    // While typing, the raw text; otherwise the committed value.
    const [draft, setDraft] = React.useState<string | null>(null);
    const text = draft ?? current?.toString() ?? '';

    const commit = (v: number | null) => {
        const next =
            v === null
                ? allowEmpty
                    ? null
                    : Math.max(min, 0)
                : Math.min(max, Math.max(min, v));
        if (!controlled) setInner(next);
        onValueChange?.(next);
        setDraft(null);
    };
    const bump = (d: number) => commit((current ?? Math.max(min, 0)) + d);

    return (
        <div
            className={cn(
                'flex h-8 w-full min-w-0 items-center rounded-lg border border-input bg-transparent transition-colors focus-within:border-ring focus-within:ring-3 focus-within:ring-ring/50 has-aria-invalid:border-destructive dark:bg-input/30',
                disabled && 'opacity-50',
                className,
            )}
        >
            <button
                type="button"
                tabIndex={-1}
                disabled={disabled || (current !== null && current <= min)}
                onClick={() => bump(-step)}
                className="flex h-full w-8 shrink-0 items-center justify-center text-muted-foreground hover:text-foreground disabled:opacity-40"
                aria-hidden
            >
                <Minus className="size-3.5" />
            </button>
            <input
                id={id}
                inputMode="numeric"
                autoComplete="off"
                className="h-full w-full min-w-0 bg-transparent text-center text-sm tabular-nums outline-none"
                value={text}
                placeholder={placeholder}
                disabled={disabled}
                required={required}
                aria-label={ariaLabel}
                aria-invalid={ariaInvalid}
                role="spinbutton"
                aria-valuenow={current ?? undefined}
                aria-valuemin={min}
                aria-valuemax={max}
                onChange={(e) =>
                    setDraft(e.target.value.replace(/[^\d-]/g, ''))
                }
                onBlur={() =>
                    commit(
                        text.trim() === ''
                            ? null
                            : Number.parseInt(text, 10) || 0,
                    )
                }
                onKeyDown={(e) => {
                    if (e.key === 'ArrowUp') {
                        e.preventDefault();
                        bump(step);
                    } else if (e.key === 'ArrowDown') {
                        e.preventDefault();
                        bump(-step);
                    } else if (e.key === 'Enter') {
                        commit(
                            text.trim() === ''
                                ? null
                                : Number.parseInt(text, 10) || 0,
                        );
                    }
                }}
            />
            <button
                type="button"
                tabIndex={-1}
                disabled={disabled || (current !== null && current >= max)}
                onClick={() => bump(step)}
                className="flex h-full w-8 shrink-0 items-center justify-center text-muted-foreground hover:text-foreground disabled:opacity-40"
                aria-hidden
            >
                <Plus className="size-3.5" />
            </button>
            {name && (
                <input
                    type="hidden"
                    name={name}
                    value={current?.toString() ?? ''}
                />
            )}
        </div>
    );
}

export { NumberField };
