'use client';

import { useRef, useState } from 'react';
import { X } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Field, FieldDescription, FieldLabel } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from '@/components/ui/select';
import { Textarea } from '@/components/ui/textarea';
import {
    DURATION_UNITS,
    type DurationUnit,
    formatDuration,
    PLACEHOLDERS,
    splitDuration,
    unitSeconds,
} from './model';

export function FieldMessage({ id, error }: { id: string; error?: string }) {
    if (!error) return null;
    return (
        <p id={`${id}-error`} role="alert" className="text-sm text-destructive">
            {error}
        </p>
    );
}

function describedBy(id: string, error?: string, hint?: string) {
    return error ? `${id}-error` : hint ? `${id}-hint` : undefined;
}

/**
 * Text input (or textarea) with placeholder chips that insert {{name}},
 * {{var1}} … at the cursor. The chips appear while the field or a chip
 * has focus, so tabbing from the field reaches them.
 */
export function PlaceholderText({
    id,
    label,
    value,
    onChange,
    multiline,
    error,
    hint,
    placeholder,
    maxLength,
    readOnly,
    type,
}: {
    id: string;
    label: string;
    value: string;
    onChange: (v: string) => void;
    multiline?: boolean;
    error?: string;
    hint?: string;
    placeholder?: string;
    maxLength?: number;
    readOnly?: boolean;
    type?: string;
}) {
    const ref = useRef<HTMLInputElement & HTMLTextAreaElement>(null);
    const insert = (ph: string) => {
        const el = ref.current;
        const start = el?.selectionStart ?? value.length;
        const end = el?.selectionEnd ?? start;
        const token = ph === '{{attr:key}}' ? '{{attr:}}' : ph;
        const caret =
            ph === '{{attr:key}}'
                ? start + '{{attr:'.length
                : start + token.length;
        onChange(value.slice(0, start) + token + value.slice(end));
        requestAnimationFrame(() => {
            el?.focus();
            el?.setSelectionRange(caret, caret);
        });
    };
    const common = {
        id,
        ref,
        value,
        placeholder,
        maxLength,
        'aria-invalid': error ? true : undefined,
        'aria-describedby': describedBy(id, error, hint),
    };
    return (
        <Field data-invalid={error ? true : undefined} className="group/ph">
            <FieldLabel htmlFor={id}>{label}</FieldLabel>
            {multiline ? (
                <Textarea
                    {...common}
                    rows={3}
                    onChange={(e) => onChange(e.target.value)}
                />
            ) : (
                <Input
                    {...common}
                    type={type}
                    onChange={(e) => onChange(e.target.value)}
                />
            )}
            {!readOnly && (
                <div
                    role="group"
                    aria-label={`Insert a placeholder into ${label}`}
                    className="hidden flex-wrap gap-1 group-focus-within/ph:flex"
                >
                    {PLACEHOLDERS.map((ph) => (
                        <button
                            key={ph}
                            type="button"
                            onClick={() => insert(ph)}
                            className="rounded-md border bg-muted px-1.5 py-0.5 font-mono text-xs hover:bg-accent focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
                        >
                            {ph === '{{attr:key}}' ? '{{attr:…}}' : ph}
                        </button>
                    ))}
                </div>
            )}
            {hint && !error && (
                <FieldDescription id={`${id}-hint`}>{hint}</FieldDescription>
            )}
            <FieldMessage id={id} error={error} />
        </Field>
    );
}

/** Labelled select over simple options. */
export function SelectField({
    id,
    label,
    value,
    onChange,
    options,
    error,
    hint,
    placeholder,
    className,
}: {
    id: string;
    label: string;
    value: string;
    onChange: (v: string) => void;
    options: readonly { value: string; label: string }[];
    error?: string;
    hint?: string;
    placeholder?: string;
    className?: string;
}) {
    return (
        <Field data-invalid={error ? true : undefined} className={className}>
            <FieldLabel htmlFor={id}>{label}</FieldLabel>
            <Select value={value} onValueChange={onChange}>
                <SelectTrigger
                    id={id}
                    className="w-full"
                    aria-invalid={error ? true : undefined}
                    aria-describedby={describedBy(id, error, hint)}
                >
                    <SelectValue placeholder={placeholder ?? 'Choose…'} />
                </SelectTrigger>
                <SelectContent>
                    {options.map((o) => (
                        <SelectItem key={o.value} value={o.value}>
                            {o.label}
                        </SelectItem>
                    ))}
                </SelectContent>
            </Select>
            {hint && !error && (
                <FieldDescription id={`${id}-hint`}>{hint}</FieldDescription>
            )}
            <FieldMessage id={id} error={error} />
        </Field>
    );
}

/** Text input suggesting existing values (e.g. tag names). */
export function SuggestInput({
    id,
    label,
    value,
    onChange,
    suggestions,
    error,
    hint,
    maxLength,
    placeholder,
}: {
    id: string;
    label: string;
    value: string;
    onChange: (v: string) => void;
    suggestions: string[];
    error?: string;
    hint?: string;
    maxLength?: number;
    placeholder?: string;
}) {
    return (
        <Field data-invalid={error ? true : undefined}>
            <FieldLabel htmlFor={id}>{label}</FieldLabel>
            <Input
                id={id}
                value={value}
                list={suggestions.length ? `${id}-list` : undefined}
                maxLength={maxLength}
                placeholder={placeholder}
                autoComplete="off"
                aria-invalid={error ? true : undefined}
                aria-describedby={describedBy(id, error, hint)}
                onChange={(e) => onChange(e.target.value)}
            />
            {suggestions.length > 0 && (
                <datalist id={`${id}-list`}>
                    {suggestions.map((s) => (
                        <option key={s} value={s} />
                    ))}
                </datalist>
            )}
            {hint && !error && (
                <FieldDescription id={`${id}-hint`}>{hint}</FieldDescription>
            )}
            <FieldMessage id={id} error={error} />
        </Field>
    );
}

/** A number plus a unit select, stored as seconds. */
export function DurationInput({
    id,
    label,
    seconds,
    onChange,
    min,
    max,
    error,
    hint,
}: {
    id: string;
    label: string;
    seconds: number;
    onChange: (seconds: number) => void;
    min: number;
    max: number;
    error?: string;
    hint?: string;
}) {
    const [state, setState] = useState(() => {
        const d = splitDuration(seconds);
        return { amount: seconds ? String(d.amount) : '0', unit: d.unit };
    });
    const set = (amount: string, unit: DurationUnit) => {
        setState({ amount, unit });
        const n = Number(amount);
        onChange(
            Number.isFinite(n) ? Math.round(n * unitSeconds(unit)) : Number.NaN,
        );
    };
    const outOfRange =
        !Number.isFinite(seconds) || seconds < min || seconds > max
            ? `Choose between ${min ? formatDuration(min) : '0'} and ${formatDuration(max)}.`
            : undefined;
    const message = error ?? outOfRange;
    return (
        <Field data-invalid={message ? true : undefined}>
            <FieldLabel htmlFor={id}>{label}</FieldLabel>
            <div className="flex gap-2">
                <Input
                    id={id}
                    type="number"
                    inputMode="numeric"
                    min={0}
                    step="any"
                    className="w-28"
                    value={state.amount}
                    aria-invalid={message ? true : undefined}
                    aria-describedby={describedBy(id, message, hint)}
                    onChange={(e) => set(e.target.value, state.unit)}
                />
                <Select
                    value={state.unit}
                    onValueChange={(u) => set(state.amount, u as DurationUnit)}
                >
                    <SelectTrigger
                        className="w-32"
                        aria-label={`${label} unit`}
                    >
                        <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                        {DURATION_UNITS.map((u) => (
                            <SelectItem key={u.value} value={u.value}>
                                {u.label}
                            </SelectItem>
                        ))}
                    </SelectContent>
                </Select>
            </div>
            {hint && !message && (
                <FieldDescription id={`${id}-hint`}>{hint}</FieldDescription>
            )}
            <FieldMessage id={id} error={message} />
        </Field>
    );
}

/** Keywords as removable chips; Enter or comma adds one. */
export function KeywordsInput({
    id,
    label,
    value,
    onChange,
    max,
    error,
    hint,
    readOnly,
}: {
    id: string;
    label: string;
    value: string[];
    onChange: (v: string[]) => void;
    max: number;
    error?: string;
    hint?: string;
    readOnly?: boolean;
}) {
    const [draft, setDraft] = useState('');
    const add = () => {
        const words = draft
            .split(',')
            .map((w) => w.trim())
            .filter(Boolean)
            .map((w) => w.slice(0, 200));
        const next = [...value];
        for (const w of words) {
            if (
                next.length < max &&
                !next.some((x) => x.toLowerCase() === w.toLowerCase())
            ) {
                next.push(w);
            }
        }
        onChange(next);
        setDraft('');
    };
    return (
        <Field data-invalid={error ? true : undefined}>
            <FieldLabel htmlFor={id}>{label}</FieldLabel>
            {value.length > 0 && (
                <ul className="flex flex-wrap gap-1" aria-label={label}>
                    {value.map((k) => (
                        <li key={k}>
                            <Badge variant="secondary" className="gap-1 pr-1">
                                {k}
                                {!readOnly && (
                                    <button
                                        type="button"
                                        className="rounded-sm hover:bg-muted-foreground/20 focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
                                        aria-label={`Remove keyword ${k}`}
                                        onClick={() =>
                                            onChange(
                                                value.filter((x) => x !== k),
                                            )
                                        }
                                    >
                                        <X className="size-3" />
                                    </button>
                                )}
                            </Badge>
                        </li>
                    ))}
                </ul>
            )}
            <div className="flex gap-2">
                <Input
                    id={id}
                    value={draft}
                    placeholder="Type a keyword and press Enter"
                    disabled={value.length >= max}
                    aria-invalid={error ? true : undefined}
                    aria-describedby={describedBy(id, error, hint)}
                    onChange={(e) => setDraft(e.target.value)}
                    onKeyDown={(e) => {
                        if (e.key === 'Enter' || e.key === ',') {
                            e.preventDefault();
                            add();
                        } else if (
                            e.key === 'Backspace' &&
                            !draft &&
                            value.length
                        ) {
                            onChange(value.slice(0, -1));
                        }
                    }}
                    onBlur={() => draft.trim() && add()}
                />
                <Button
                    type="button"
                    variant="outline"
                    disabled={!draft.trim() || value.length >= max}
                    onClick={add}
                >
                    Add
                </Button>
            </div>
            {hint && !error && (
                <FieldDescription id={`${id}-hint`}>{hint}</FieldDescription>
            )}
            <FieldMessage id={id} error={error} />
        </Field>
    );
}
