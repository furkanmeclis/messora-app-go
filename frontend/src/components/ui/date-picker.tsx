'use client';

import * as React from 'react';
import { cn } from 'cn';
import { CalendarDays, ChevronLeft, ChevronRight, Clock } from 'lucide-react';
import { useLocale, useT } from '@/i18n/client';
import { intlLocale } from '@/i18n/config';
import { Popover, PopoverContent, PopoverTrigger } from './popover';

/** Calendar dates as YYYY-MM-DD, free of time zones. */
type Ymd = { y: number; m: number; d: number };
const pad = (n: number) => String(n).padStart(2, '0');
const fmt = ({ y, m, d }: Ymd) => `${y}-${pad(m + 1)}-${pad(d)}`;
function parse(v: string | undefined | null): Ymd | null {
    const m = /^(\d{4})-(\d{2})-(\d{2})/.exec(v ?? '');
    return m ? { y: +m[1]!, m: +m[2]! - 1, d: +m[3]! } : null;
}
function todayYmd(): Ymd {
    const n = new Date();
    return { y: n.getFullYear(), m: n.getMonth(), d: n.getDate() };
}
const same = (a: Ymd | null, b: Ymd | null) =>
    !!a && !!b && a.y === b.y && a.m === b.m && a.d === b.d;
const cmp = (a: Ymd, b: Ymd) => fmt(a).localeCompare(fmt(b));

function Calendar({
    selected,
    onSelect,
    min,
    max,
}: {
    selected: Ymd | null;
    onSelect: (d: Ymd) => void;
    min?: Ymd | null;
    max?: Ymd | null;
}) {
    const locale = intlLocale(useLocale());
    const [view, setView] = React.useState(() => {
        const s = selected ?? todayYmd();
        return { y: s.y, m: s.m };
    });
    const [focus, setFocus] = React.useState<Ymd>(selected ?? todayYmd());
    const grid = React.useRef<HTMLDivElement>(null);
    const today = todayYmd();

    // Monday-first weeks (en-GB and tr-TR).
    const first = new Date(Date.UTC(view.y, view.m, 1));
    const offset = (first.getUTCDay() + 6) % 7;
    const days = new Date(Date.UTC(view.y, view.m + 1, 0)).getUTCDate();
    const cells: (Ymd | null)[] = [
        ...Array.from({ length: offset }, () => null),
        ...Array.from({ length: days }, (_, i) => ({
            y: view.y,
            m: view.m,
            d: i + 1,
        })),
    ];
    const weekdays = Array.from({ length: 7 }, (_, i) =>
        new Intl.DateTimeFormat(locale, {
            weekday: 'short',
            timeZone: 'UTC',
        }).format(new Date(Date.UTC(2024, 0, 1 + i))),
    );
    const title = new Intl.DateTimeFormat(locale, {
        month: 'long',
        year: 'numeric',
        timeZone: 'UTC',
    }).format(first);
    const disabled = (d: Ymd) =>
        (!!min && cmp(d, min) < 0) || (!!max && cmp(d, max) > 0);
    const shift = (months: number) =>
        setView((v) => {
            const dt = new Date(Date.UTC(v.y, v.m + months, 1));
            return { y: dt.getUTCFullYear(), m: dt.getUTCMonth() };
        });

    const move = (days: number) => {
        const dt = new Date(Date.UTC(focus.y, focus.m, focus.d + days));
        const next = {
            y: dt.getUTCFullYear(),
            m: dt.getUTCMonth(),
            d: dt.getUTCDate(),
        };
        setFocus(next);
        setView({ y: next.y, m: next.m });
        requestAnimationFrame(() =>
            grid.current
                ?.querySelector<HTMLButtonElement>(`[data-day="${fmt(next)}"]`)
                ?.focus(),
        );
    };

    return (
        <div className="w-64 select-none">
            <div className="mb-2 flex items-center justify-between">
                <button
                    type="button"
                    aria-label="Previous month"
                    onClick={() => shift(-1)}
                    className="flex size-7 items-center justify-center rounded-md hover:bg-accent"
                >
                    <ChevronLeft className="size-4" />
                </button>
                <span className="text-sm font-medium capitalize">{title}</span>
                <button
                    type="button"
                    aria-label="Next month"
                    onClick={() => shift(1)}
                    className="flex size-7 items-center justify-center rounded-md hover:bg-accent"
                >
                    <ChevronRight className="size-4" />
                </button>
            </div>
            <div className="grid grid-cols-7 gap-0.5 text-center text-[11px] text-muted-foreground">
                {weekdays.map((w) => (
                    <span key={w} className="py-1 capitalize">
                        {w}
                    </span>
                ))}
            </div>
            <div
                ref={grid}
                role="grid"
                className="grid grid-cols-7 gap-0.5"
                onKeyDown={(e) => {
                    const k = {
                        ArrowLeft: -1,
                        ArrowRight: 1,
                        ArrowUp: -7,
                        ArrowDown: 7,
                    }[e.key];
                    if (k) {
                        e.preventDefault();
                        move(k);
                    }
                }}
            >
                {cells.map((c, i) =>
                    c ? (
                        <button
                            key={fmt(c)}
                            type="button"
                            data-day={fmt(c)}
                            tabIndex={same(c, focus) ? 0 : -1}
                            disabled={disabled(c)}
                            aria-pressed={same(c, selected)}
                            onClick={() => onSelect(c)}
                            className={cn(
                                'flex h-8 items-center justify-center rounded-md text-sm tabular-nums transition-colors outline-none hover:bg-accent focus-visible:ring-2 focus-visible:ring-ring disabled:pointer-events-none disabled:opacity-30',
                                same(c, today) && 'font-semibold text-primary',
                                same(c, selected) &&
                                    'bg-primary text-primary-foreground hover:bg-primary',
                            )}
                        >
                            {c.d}
                        </button>
                    ) : (
                        <span key={`e${i}`} />
                    ),
                )}
            </div>
        </div>
    );
}

const triggerClass =
    'flex h-8 w-full items-center gap-2 rounded-lg border border-input bg-transparent px-2.5 text-left text-sm transition-colors outline-none hover:bg-accent/40 focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 disabled:opacity-50 dark:bg-input/30';

/** Date field (YYYY-MM-DD), submitted under `name`. */
function DatePicker({
    name,
    value,
    defaultValue,
    onValueChange,
    min,
    max,
    id,
    className,
    disabled,
    placeholder,
}: {
    name?: string;
    value?: string;
    defaultValue?: string;
    onValueChange?: (v: string) => void;
    min?: string;
    max?: string;
    id?: string;
    className?: string;
    disabled?: boolean;
    placeholder?: string;
}) {
    const t = useT();
    const locale = intlLocale(useLocale());
    const [open, setOpen] = React.useState(false);
    const [inner, setInner] = React.useState(defaultValue ?? '');
    const current = value ?? inner;
    const sel = parse(current);
    const set = (v: string) => {
        if (value === undefined) setInner(v);
        onValueChange?.(v);
    };
    const label = sel
        ? new Intl.DateTimeFormat(locale, {
              dateStyle: 'medium',
              timeZone: 'UTC',
          }).format(new Date(Date.UTC(sel.y, sel.m, sel.d)))
        : (placeholder ?? t('common.selectDate'));
    return (
        <Popover open={open} onOpenChange={setOpen}>
            <PopoverTrigger asChild>
                <button
                    type="button"
                    id={id}
                    disabled={disabled}
                    className={cn(triggerClass, className)}
                >
                    <CalendarDays className="size-4 text-muted-foreground" />
                    <span
                        className={cn(
                            'truncate',
                            !sel && 'text-muted-foreground',
                        )}
                    >
                        {label}
                    </span>
                </button>
            </PopoverTrigger>
            <PopoverContent className="w-auto">
                <Calendar
                    selected={sel}
                    min={parse(min)}
                    max={parse(max)}
                    onSelect={(d) => {
                        set(fmt(d));
                        setOpen(false);
                    }}
                />
                <div className="mt-2 flex justify-between border-t pt-2">
                    <button
                        type="button"
                        className="text-xs text-muted-foreground hover:text-foreground"
                        onClick={() => {
                            set('');
                            setOpen(false);
                        }}
                    >
                        {t('common.clear')}
                    </button>
                    <button
                        type="button"
                        className="text-xs font-medium text-primary"
                        onClick={() => {
                            set(fmt(todayYmd()));
                            setOpen(false);
                        }}
                    >
                        {t('common.today')}
                    </button>
                </div>
            </PopoverContent>
            {name && <input type="hidden" name={name} value={current} />}
        </Popover>
    );
}

/**
 * Date and time field. The value has the datetime-local shape
 * (YYYY-MM-DDTHH:mm, wall-clock time), submitted under `name`.
 */
function DateTimePicker({
    name,
    value,
    defaultValue,
    onValueChange,
    min,
    id,
    className,
    disabled,
    minuteStep = 5,
}: {
    name?: string;
    value?: string;
    defaultValue?: string;
    onValueChange?: (v: string) => void;
    min?: string;
    id?: string;
    className?: string;
    disabled?: boolean;
    minuteStep?: number;
}) {
    const t = useT();
    const locale = intlLocale(useLocale());
    const [open, setOpen] = React.useState(false);
    const [inner, setInner] = React.useState(defaultValue ?? '');
    const current = value ?? inner;
    const sel = parse(current);
    const time = /T(\d{2}):(\d{2})/.exec(current);
    const hh = time ? +time[1]! : 9;
    const mm = time ? +time[2]! : 0;
    const set = (d: Ymd | null, h: number, m: number) => {
        const day = d ?? sel ?? todayYmd();
        const v = `${fmt(day)}T${pad(h)}:${pad(m)}`;
        if (value === undefined) setInner(v);
        onValueChange?.(v);
    };
    const label = sel
        ? new Intl.DateTimeFormat(locale, {
              dateStyle: 'medium',
              timeStyle: 'short',
              timeZone: 'UTC',
          }).format(new Date(Date.UTC(sel.y, sel.m, sel.d, hh, mm)))
        : t('common.selectDate');
    const column = (
        count: number,
        step: number,
        now: number,
        pick: (n: number) => void,
    ) => (
        <div className="flex h-64 w-12 flex-col overflow-y-auto rounded-md border p-0.5">
            {Array.from(
                { length: Math.ceil(count / step) },
                (_, i) => i * step,
            ).map((n) => (
                <button
                    key={n}
                    type="button"
                    onClick={() => pick(n)}
                    ref={(el) => {
                        if (el && n === now)
                            el.scrollIntoView({ block: 'center' });
                    }}
                    className={cn(
                        'shrink-0 rounded py-1 text-sm tabular-nums hover:bg-accent',
                        n === now &&
                            'bg-primary text-primary-foreground hover:bg-primary',
                    )}
                >
                    {pad(n)}
                </button>
            ))}
        </div>
    );
    return (
        <Popover open={open} onOpenChange={setOpen}>
            <PopoverTrigger asChild>
                <button
                    type="button"
                    id={id}
                    disabled={disabled}
                    className={cn(triggerClass, className)}
                >
                    <Clock className="size-4 text-muted-foreground" />
                    <span
                        className={cn(
                            'truncate',
                            !sel && 'text-muted-foreground',
                        )}
                    >
                        {label}
                    </span>
                </button>
            </PopoverTrigger>
            <PopoverContent className="flex w-auto gap-3">
                <Calendar
                    selected={sel}
                    min={parse(min)}
                    onSelect={(d) => set(d, hh, mm)}
                />
                <div className="flex gap-1">
                    {column(24, 1, hh, (h) => set(null, h, mm))}
                    {column(60, minuteStep, mm - (mm % minuteStep), (m) =>
                        set(null, hh, m),
                    )}
                </div>
            </PopoverContent>
            {name && <input type="hidden" name={name} value={current} />}
        </Popover>
    );
}

export { Calendar, DatePicker, DateTimePicker };
