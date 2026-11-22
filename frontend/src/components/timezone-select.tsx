'use client';

import { useMemo } from 'react';
import { Field, FieldLabel } from '@/components/ui/field';
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from '@/components/ui/select';

/** IANA time zone picker backed by the browser's list. */
export function TimezoneSelect({
    name,
    label,
    defaultValue,
}: {
    name: string;
    label: string;
    defaultValue: string;
}) {
    const zones = useMemo(() => {
        const list =
            typeof Intl.supportedValuesOf === 'function'
                ? Intl.supportedValuesOf('timeZone')
                : [];
        return Array.from(new Set(['UTC', ...list, defaultValue]));
    }, [defaultValue]);
    return (
        <Field>
            <FieldLabel htmlFor={`field-${name}`}>{label}</FieldLabel>
            <Select name={name} defaultValue={defaultValue}>
                <SelectTrigger id={`field-${name}`} className="w-full">
                    <SelectValue />
                </SelectTrigger>
                <SelectContent className="max-h-72">
                    {zones.map((z) => (
                        <SelectItem key={z} value={z}>
                            {z.replaceAll('_', ' ')}
                        </SelectItem>
                    ))}
                </SelectContent>
            </Select>
        </Field>
    );
}
