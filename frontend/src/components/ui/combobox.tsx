'use client';

import * as React from 'react';
import { cn } from 'cn';
import { Check, ChevronsUpDown, Search } from 'lucide-react';
import { Popover, PopoverContent, PopoverTrigger } from './popover';

export type ComboboxOption = {
    value: string;
    label: string;
    /** Extra words to match (e.g. a city for a time zone). */
    keywords?: string;
    hint?: string;
};

/** Searchable single choice with keyboard navigation. */
function Combobox({
    options,
    value,
    defaultValue,
    onValueChange,
    name,
    id,
    placeholder,
    searchPlaceholder = 'Search…',
    emptyText = 'No results',
    className,
    disabled,
    'aria-label': ariaLabel,
}: {
    options: ComboboxOption[];
    value?: string;
    defaultValue?: string;
    onValueChange?: (v: string) => void;
    name?: string;
    id?: string;
    placeholder?: string;
    searchPlaceholder?: string;
    emptyText?: string;
    className?: string;
    disabled?: boolean;
    'aria-label'?: string;
}) {
    const [open, setOpen] = React.useState(false);
    const [inner, setInner] = React.useState(defaultValue ?? '');
    const current = value ?? inner;
    const [query, setQuery] = React.useState('');
    const [active, setActive] = React.useState(0);
    const listRef = React.useRef<HTMLDivElement>(null);
    const listId = React.useId();

    const filtered = React.useMemo(() => {
        const q = query.trim().toLocaleLowerCase();
        if (!q) return options;
        return options.filter((o) =>
            `${o.label} ${o.value} ${o.keywords ?? ''}`
                .toLocaleLowerCase()
                .includes(q),
        );
    }, [options, query]);

    const choose = (v: string) => {
        if (value === undefined) setInner(v);
        onValueChange?.(v);
        setOpen(false);
    };
    const selected = options.find((o) => o.value === current);

    React.useEffect(() => {
        listRef.current
            ?.querySelector(`[data-index="${active}"]`)
            ?.scrollIntoView({ block: 'nearest' });
    }, [active]);

    return (
        <Popover
            open={open}
            onOpenChange={(o) => {
                setOpen(o);
                if (o) {
                    setQuery('');
                    setActive(
                        Math.max(
                            0,
                            options.findIndex((x) => x.value === current),
                        ),
                    );
                }
            }}
        >
            <PopoverTrigger asChild>
                <button
                    type="button"
                    id={id}
                    disabled={disabled}
                    role="combobox"
                    aria-expanded={open}
                    aria-controls={listId}
                    aria-label={ariaLabel}
                    className={cn(
                        'flex h-8 w-full items-center justify-between gap-2 rounded-lg border border-input bg-transparent px-2.5 text-left text-sm transition-colors outline-none hover:bg-accent/40 focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 disabled:opacity-50 dark:bg-input/30',
                        className,
                    )}
                >
                    <span
                        className={cn(
                            'truncate',
                            !selected && 'text-muted-foreground',
                        )}
                    >
                        {selected?.label ?? placeholder}
                    </span>
                    <ChevronsUpDown className="size-4 shrink-0 text-muted-foreground" />
                </button>
            </PopoverTrigger>
            <PopoverContent className="w-(--radix-popover-trigger-width) min-w-64 p-0">
                <div className="flex items-center gap-2 border-b px-3">
                    <Search className="size-4 text-muted-foreground" />
                    <input
                        autoFocus
                        value={query}
                        placeholder={searchPlaceholder}
                        aria-controls={listId}
                        aria-activedescendant={`${listId}-${active}`}
                        className="h-10 w-full bg-transparent text-sm outline-none placeholder:text-muted-foreground"
                        onChange={(e) => {
                            setQuery(e.target.value);
                            setActive(0);
                        }}
                        onKeyDown={(e) => {
                            if (e.key === 'ArrowDown') {
                                e.preventDefault();
                                setActive((a) =>
                                    Math.min(filtered.length - 1, a + 1),
                                );
                            } else if (e.key === 'ArrowUp') {
                                e.preventDefault();
                                setActive((a) => Math.max(0, a - 1));
                            } else if (e.key === 'Enter') {
                                e.preventDefault();
                                const o = filtered[active];
                                if (o) choose(o.value);
                            }
                        }}
                    />
                </div>
                <div
                    ref={listRef}
                    id={listId}
                    role="listbox"
                    className="max-h-64 overflow-y-auto p-1"
                >
                    {filtered.length === 0 && (
                        <p className="px-2 py-6 text-center text-sm text-muted-foreground">
                            {emptyText}
                        </p>
                    )}
                    {filtered.map((o, i) => (
                        <div
                            key={o.value}
                            id={`${listId}-${i}`}
                            data-index={i}
                            role="option"
                            aria-selected={o.value === current}
                            onMouseEnter={() => setActive(i)}
                            onClick={() => choose(o.value)}
                            className={cn(
                                'flex cursor-pointer items-center gap-2 rounded-md px-2 py-1.5 text-sm',
                                i === active &&
                                    'bg-accent text-accent-foreground',
                            )}
                        >
                            <Check
                                className={cn(
                                    'size-4 shrink-0',
                                    o.value === current
                                        ? 'opacity-100'
                                        : 'opacity-0',
                                )}
                            />
                            <span className="truncate">{o.label}</span>
                            {o.hint && (
                                <span className="ml-auto shrink-0 text-xs text-muted-foreground">
                                    {o.hint}
                                </span>
                            )}
                        </div>
                    ))}
                </div>
            </PopoverContent>
            {name && <input type="hidden" name={name} value={current} />}
        </Popover>
    );
}

export { Combobox };
