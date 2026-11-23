'use client';

import { useMemo } from 'react';
import { Check } from 'lucide-react';
import { Combobox } from '@/components/ui/combobox';
import { RadioCard, RadioCards } from '@/components/ui/radio-cards';
import { Segmented } from '@/components/ui/segmented';
import { useT } from '@/i18n/client';
import { localeNames, locales, type Locale } from '@/i18n/config';
import { formatDate, formatTime, prefsFor } from '@/lib/format';
import type { RoleKey, Schemas } from '@/lib/types';
import { cn } from '@/lib/utils';

export const DATE_FORMATS: Schemas['DateFormat'][] = [
    'dd.MM.yyyy',
    'dd/MM/yyyy',
    'MM/dd/yyyy',
    'yyyy-MM-dd',
    'd MMM yyyy',
];

// A fixed sample (25 March, 14:30 UTC) so server and browser agree.
const SAMPLE = '2026-03-25T14:30:00Z';

function zoneOffset(zone: string): string {
    try {
        const part = new Intl.DateTimeFormat('en-US', {
            timeZone: zone,
            timeZoneName: 'shortOffset',
        })
            .formatToParts(new Date(SAMPLE))
            .find((p) => p.type === 'timeZoneName')?.value;
        return part === 'GMT' ? 'GMT+0' : (part ?? '');
    } catch {
        return '';
    }
}

/** Searchable IANA time zone with UTC offsets. */
export function TimezoneField({
    value,
    onChange,
    disabled,
    id,
}: {
    value: string;
    onChange: (v: string) => void;
    disabled?: boolean;
    id?: string;
}) {
    const t = useT();
    const options = useMemo(() => {
        const list =
            typeof Intl.supportedValuesOf === 'function'
                ? Intl.supportedValuesOf('timeZone')
                : [];
        return Array.from(new Set(['UTC', ...list, value])).map((z) => ({
            value: z,
            label: z.replaceAll('_', ' '),
            hint: zoneOffset(z),
            keywords: z.split('/').pop(),
        }));
    }, [value]);
    return (
        <Combobox
            id={id}
            options={options}
            value={value}
            onValueChange={onChange}
            disabled={disabled}
            searchPlaceholder={t('common.searchPlaceholder')}
            emptyText={t('common.noResults')}
        />
    );
}

export function LocaleField({
    value,
    onChange,
    disabled,
}: {
    value: Locale;
    onChange: (v: Locale) => void;
    disabled?: boolean;
}) {
    return (
        <RadioCards
            value={value}
            onValueChange={(v) => onChange(v as Locale)}
            disabled={disabled}
            className="sm:grid-cols-2"
        >
            {locales.map((l) => (
                <RadioCard key={l} value={l}>
                    <span className="flex items-center gap-2 font-medium">
                        <span
                            aria-hidden
                            className="flex h-5 w-7 items-center justify-center rounded bg-muted text-[10px] font-bold tracking-wide uppercase"
                        >
                            {l}
                        </span>
                        {localeNames[l]}
                    </span>
                </RadioCard>
            ))}
        </RadioCards>
    );
}

export function DateFormatField({
    value,
    onChange,
    locale,
    disabled,
}: {
    value: Schemas['DateFormat'];
    onChange: (v: Schemas['DateFormat']) => void;
    locale: Locale;
    disabled?: boolean;
}) {
    return (
        <RadioCards
            value={value}
            onValueChange={(v) => onChange(v as Schemas['DateFormat'])}
            disabled={disabled}
            className="grid-cols-2 sm:grid-cols-3"
        >
            {DATE_FORMATS.map((f) => (
                <RadioCard key={f} value={f}>
                    <span className="block font-medium tabular-nums">
                        {formatDate(SAMPLE, prefsFor('UTC', locale, f))}
                    </span>
                    <span className="block font-mono text-xs text-muted-foreground">
                        {f}
                    </span>
                </RadioCard>
            ))}
        </RadioCards>
    );
}

export function TimeFormatField({
    value,
    onChange,
    locale,
    disabled,
}: {
    value: Schemas['TimeFormat'];
    onChange: (v: Schemas['TimeFormat']) => void;
    locale: Locale;
    disabled?: boolean;
}) {
    const sample = (f: '24h' | '12h') =>
        formatTime(SAMPLE, prefsFor('UTC', locale, undefined, f));
    return (
        <Segmented
            value={value}
            onValueChange={onChange}
            disabled={disabled}
            options={(['24h', '12h'] as const).map((f) => ({
                value: f,
                label: (
                    <span className="tabular-nums">
                        {f} · {sample(f)}
                    </span>
                ),
            }))}
        />
    );
}

const ROLES: RoleKey[] = ['owner', 'admin', 'manager', 'agent'];

/** Roles that must use two-factor authentication. */
export function MfaRolesField({
    value,
    onChange,
    disabled,
}: {
    value: RoleKey[];
    onChange: (v: RoleKey[]) => void;
    disabled?: boolean;
}) {
    const t = useT();
    return (
        <div className="grid grid-cols-2 gap-2 sm:grid-cols-4">
            {ROLES.map((r) => {
                const on = value.includes(r);
                return (
                    <button
                        key={r}
                        type="button"
                        role="checkbox"
                        aria-checked={on}
                        disabled={disabled}
                        onClick={() =>
                            onChange(
                                on
                                    ? value.filter((x) => x !== r)
                                    : [...value, r],
                            )
                        }
                        className={cn(
                            'flex items-center gap-2 rounded-xl border bg-card px-3 py-2.5 text-sm font-medium transition-all outline-none hover:border-primary/40 focus-visible:ring-3 focus-visible:ring-ring/50 disabled:opacity-50',
                            on &&
                                'border-primary bg-accent/60 shadow-[0_0_0_1px_var(--primary)]',
                        )}
                    >
                        <span
                            className={cn(
                                'flex size-4 items-center justify-center rounded border border-input transition-colors',
                                on && 'border-primary bg-primary',
                            )}
                        >
                            {on && (
                                <Check className="size-3 text-primary-foreground" />
                            )}
                        </span>
                        {t(`settings.security.roles.${r}`)}
                    </button>
                );
            })}
        </div>
    );
}
