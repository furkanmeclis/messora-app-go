'use client';

import { useId, useRef, useState } from 'react';
import { Braces, X } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuLabel,
    DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { Field, FieldDescription, FieldLabel } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { NumberField } from '@/components/ui/number-field';
import {
    Popover,
    PopoverAnchor,
    PopoverContent,
} from '@/components/ui/popover';
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from '@/components/ui/select';
import { Textarea } from '@/components/ui/textarea';
import { useT } from '@/i18n/client';
import { cn } from '@/lib/utils';
import {
    DURATION_UNITS,
    type DurationUnit,
    formatDuration,
    mk,
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
 * Text input (or textarea) with an "Insert placeholder" menu of chips that
 * insert {{name}}, {{var1}} … at the cursor.
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
}) {
    const t = useT();
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
        readOnly,
        'aria-invalid': error ? true : undefined,
        'aria-describedby': describedBy(id, error, hint),
    };
    return (
        <Field data-invalid={error ? true : undefined}>
            <div className="flex items-center justify-between gap-2">
                <FieldLabel htmlFor={id}>{label}</FieldLabel>
                {!readOnly && (
                    <DropdownMenu>
                        <DropdownMenuTrigger asChild>
                            <Button
                                type="button"
                                variant="ghost"
                                size="xs"
                                aria-label={t('automations.inputs.insertInto', {
                                    label,
                                })}
                                className="font-mono text-muted-foreground"
                            >
                                <Braces /> {t('automations.inputs.insert')}
                            </Button>
                        </DropdownMenuTrigger>
                        <DropdownMenuContent
                            align="end"
                            className="w-56"
                            onCloseAutoFocus={(e) => e.preventDefault()}
                        >
                            <DropdownMenuLabel>
                                {t('automations.inputs.placeholders')}
                            </DropdownMenuLabel>
                            <div className="flex flex-wrap gap-1 p-1">
                                {PLACEHOLDERS.map((ph) => (
                                    <DropdownMenuItem
                                        key={ph}
                                        onSelect={() => insert(ph)}
                                        className="rounded-md border bg-muted px-1.5 py-0.5 font-mono text-xs"
                                    >
                                        {ph === '{{attr:key}}'
                                            ? '{{attr:…}}'
                                            : ph}
                                    </DropdownMenuItem>
                                ))}
                            </div>
                        </DropdownMenuContent>
                    </DropdownMenu>
                )}
            </div>
            {multiline ? (
                <Textarea
                    {...common}
                    rows={3}
                    onChange={(e) => onChange(e.target.value)}
                />
            ) : (
                <Input {...common} onChange={(e) => onChange(e.target.value)} />
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
    disabled,
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
    disabled?: boolean;
}) {
    const t = useT();
    return (
        <Field data-invalid={error ? true : undefined} className={className}>
            <FieldLabel htmlFor={id}>{label}</FieldLabel>
            <Select value={value} onValueChange={onChange} disabled={disabled}>
                <SelectTrigger
                    id={id}
                    className="w-full"
                    aria-invalid={error ? true : undefined}
                    aria-describedby={describedBy(id, error, hint)}
                >
                    <SelectValue
                        placeholder={placeholder ?? t('common.select')}
                    />
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

/**
 * Text input suggesting existing values (e.g. tag names) in a list below
 * it; any other value can be typed too.
 */
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
    readOnly,
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
    readOnly?: boolean;
}) {
    const listId = useId();
    const [open, setOpen] = useState(false);
    const [active, setActive] = useState(0);
    const q = value.trim().toLocaleLowerCase();
    const matches = suggestions
        .filter(
            (s) =>
                s.toLocaleLowerCase().includes(q) &&
                s.toLocaleLowerCase() !== q,
        )
        .slice(0, 8);
    const show = open && !readOnly && matches.length > 0;
    const pick = (s: string) => {
        onChange(s);
        setOpen(false);
    };
    return (
        <Field data-invalid={error ? true : undefined}>
            <FieldLabel htmlFor={id}>{label}</FieldLabel>
            <Popover open={show} onOpenChange={setOpen}>
                <PopoverAnchor asChild>
                    <Input
                        id={id}
                        value={value}
                        maxLength={maxLength}
                        placeholder={placeholder}
                        readOnly={readOnly}
                        autoComplete="off"
                        role="combobox"
                        aria-autocomplete="list"
                        aria-expanded={show}
                        aria-controls={listId}
                        aria-activedescendant={
                            show ? `${listId}-${active}` : undefined
                        }
                        aria-invalid={error ? true : undefined}
                        aria-describedby={describedBy(id, error, hint)}
                        onFocus={() => setOpen(true)}
                        onBlur={() => setOpen(false)}
                        onChange={(e) => {
                            onChange(e.target.value);
                            setOpen(true);
                            setActive(0);
                        }}
                        onKeyDown={(e) => {
                            if (!show) return;
                            if (e.key === 'ArrowDown') {
                                e.preventDefault();
                                setActive((a) =>
                                    Math.min(matches.length - 1, a + 1),
                                );
                            } else if (e.key === 'ArrowUp') {
                                e.preventDefault();
                                setActive((a) => Math.max(0, a - 1));
                            } else if (e.key === 'Enter') {
                                e.preventDefault();
                                const m = matches[active];
                                if (m) pick(m);
                            } else if (e.key === 'Escape') {
                                setOpen(false);
                            }
                        }}
                    />
                </PopoverAnchor>
                <PopoverContent
                    className="w-(--radix-popover-trigger-width) min-w-48 p-1"
                    onOpenAutoFocus={(e) => e.preventDefault()}
                    onCloseAutoFocus={(e) => e.preventDefault()}
                >
                    <div id={listId} role="listbox" aria-label={label}>
                        {matches.map((m, i) => (
                            <div
                                key={m}
                                id={`${listId}-${i}`}
                                role="option"
                                aria-selected={i === active}
                                onMouseDown={(e) => e.preventDefault()}
                                onMouseEnter={() => setActive(i)}
                                onClick={() => pick(m)}
                                className={cn(
                                    'cursor-pointer truncate rounded-md px-2 py-1.5 text-sm',
                                    i === active &&
                                        'bg-accent text-accent-foreground',
                                )}
                            >
                                {m}
                            </div>
                        ))}
                    </div>
                </PopoverContent>
            </Popover>
            {hint && !error && (
                <FieldDescription id={`${id}-hint`}>{hint}</FieldDescription>
            )}
            <FieldMessage id={id} error={error} />
        </Field>
    );
}

/** A whole number plus a unit, stored as seconds. */
export function DurationInput({
    id,
    label,
    seconds,
    onChange,
    min,
    max,
    error,
    hint,
    disabled,
}: {
    id: string;
    label: string;
    seconds: number;
    onChange: (seconds: number) => void;
    min: number;
    max: number;
    error?: string;
    hint?: string;
    disabled?: boolean;
}) {
    const t = useT();
    const [chosen, setUnit] = useState<DurationUnit>(
        () => splitDuration(seconds).unit,
    );
    // After an undo the value may not fit the chosen unit any more.
    const unit =
        seconds % unitSeconds(chosen) === 0
            ? chosen
            : splitDuration(seconds).unit;
    const per = unitSeconds(unit);
    const amount = Math.round(seconds / per);
    const outOfRange =
        !Number.isFinite(seconds) || seconds < min || seconds > max
            ? t('automations.inputs.durationRange', {
                  min: min ? formatDuration(min, t) : '0',
                  max: formatDuration(max, t),
              })
            : undefined;
    const message = error ?? outOfRange;
    return (
        <Field data-invalid={message ? true : undefined}>
            <FieldLabel htmlFor={id}>{label}</FieldLabel>
            <div className="flex gap-2">
                <NumberField
                    id={id}
                    className="w-32"
                    min={0}
                    disabled={disabled}
                    value={amount}
                    aria-label={label}
                    aria-invalid={message ? true : undefined}
                    onValueChange={(v) => onChange((v ?? 0) * per)}
                />
                <Select
                    value={unit}
                    disabled={disabled}
                    onValueChange={(u) => {
                        const next = u as DurationUnit;
                        setUnit(next);
                        onChange(amount * unitSeconds(next));
                    }}
                >
                    <SelectTrigger
                        className="w-32"
                        aria-label={t('automations.inputs.unitOf', { label })}
                    >
                        <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                        {DURATION_UNITS.map((u) => (
                            <SelectItem key={u.value} value={u.value}>
                                {t(mk(`automations.units.${u.value}`))}
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
    const t = useT();
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
                                        aria-label={t(
                                            'automations.inputs.removeKeyword',
                                            { keyword: k },
                                        )}
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
            {!readOnly && (
                <div className="flex gap-2">
                    <Input
                        id={id}
                        value={draft}
                        placeholder={t('automations.inputs.keywordPlaceholder')}
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
                        {t('common.add')}
                    </Button>
                </div>
            )}
            {hint && !error && (
                <FieldDescription id={`${id}-hint`}>{hint}</FieldDescription>
            )}
            <FieldMessage id={id} error={error} />
        </Field>
    );
}
