'use client';

import * as React from 'react';
import { cn } from 'cn';
import { RadioGroup as RadioGroupPrimitive } from 'radix-ui';

export type SegmentedOption<T extends string> = {
    value: T;
    label: React.ReactNode;
    icon?: React.ReactNode;
};

/** A compact single choice (a row of joined buttons). */
function Segmented<T extends string>({
    options,
    value,
    defaultValue,
    onValueChange,
    name,
    className,
    disabled,
    'aria-label': ariaLabel,
}: {
    options: SegmentedOption<T>[];
    value?: T;
    defaultValue?: T;
    onValueChange?: (v: T) => void;
    name?: string;
    className?: string;
    disabled?: boolean;
    'aria-label'?: string;
}) {
    return (
        <RadioGroupPrimitive.Root
            name={name}
            value={value}
            defaultValue={defaultValue}
            onValueChange={(v) => onValueChange?.(v as T)}
            disabled={disabled}
            aria-label={ariaLabel}
            orientation="horizontal"
            className={cn(
                'inline-flex flex-wrap gap-1 rounded-xl bg-muted p-1',
                className,
            )}
        >
            {options.map((o) => (
                <RadioGroupPrimitive.Item
                    key={o.value}
                    value={o.value}
                    className="inline-flex h-8 items-center gap-1.5 rounded-lg px-3 text-sm font-medium text-muted-foreground transition-all outline-none hover:text-foreground focus-visible:ring-3 focus-visible:ring-ring/50 disabled:opacity-50 data-[state=checked]:bg-background data-[state=checked]:text-foreground data-[state=checked]:shadow-sm [&_svg]:size-4"
                >
                    {o.icon}
                    {o.label}
                </RadioGroupPrimitive.Item>
            ))}
        </RadioGroupPrimitive.Root>
    );
}

export { Segmented };
