'use client';

import { useMemo } from 'react';
import { Field, FieldLabel } from '@/components/ui/field';
import { Combobox, type ComboboxOption } from '@/components/ui/combobox';
import { useT } from '@/i18n/client';

/** Searchable IANA time zone picker backed by the browser's list. */
export function TimezoneSelect({
    name,
    label,
    defaultValue,
}: {
    name: string;
    label: string;
    defaultValue: string;
}) {
    const t = useT();
    const zones = useMemo<ComboboxOption[]>(() => {
        const list =
            typeof Intl.supportedValuesOf === 'function'
                ? Intl.supportedValuesOf('timeZone')
                : [];
        return Array.from(new Set(['UTC', ...list, defaultValue])).map((z) => ({
            value: z,
            label: z.replaceAll('_', ' '),
            // Match the city alone ("istanbul") as well as the full id.
            keywords: z.split('/').pop()?.replaceAll('_', ' '),
        }));
    }, [defaultValue]);
    return (
        <Field>
            <FieldLabel htmlFor={`field-${name}`}>{label}</FieldLabel>
            <Combobox
                id={`field-${name}`}
                name={name}
                defaultValue={defaultValue}
                options={zones}
                placeholder={t('common.select')}
                searchPlaceholder={t('common.searchPlaceholder')}
                emptyText={t('common.noResults')}
            />
        </Field>
    );
}
