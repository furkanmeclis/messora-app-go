'use client';

import * as React from 'react';
import { cn } from 'cn';
import { RadioGroup as RadioGroupPrimitive } from 'radix-ui';
import { Check } from 'lucide-react';

/** A single choice shown as selectable cards. */
function RadioCards({
    className,
    ...props
}: React.ComponentProps<typeof RadioGroupPrimitive.Root>) {
    return (
        <RadioGroupPrimitive.Root
            data-slot="radio-cards"
            className={cn('grid gap-2', className)}
            {...props}
        />
    );
}

function RadioCard({
    className,
    children,
    ...props
}: React.ComponentProps<typeof RadioGroupPrimitive.Item>) {
    return (
        <RadioGroupPrimitive.Item
            data-slot="radio-card"
            className={cn(
                'group/card relative flex w-full items-start gap-3 rounded-xl border bg-card p-3 text-left text-sm transition-all outline-none hover:border-primary/40 hover:bg-accent/40 focus-visible:ring-3 focus-visible:ring-ring/50 disabled:cursor-not-allowed disabled:opacity-50 data-[state=checked]:border-primary data-[state=checked]:bg-accent/60 data-[state=checked]:shadow-[0_0_0_1px_var(--primary)]',
                className,
            )}
            {...props}
        >
            <span className="mt-0.5 flex size-4 shrink-0 items-center justify-center rounded-full border border-input transition-colors group-data-[state=checked]/card:border-primary group-data-[state=checked]/card:bg-primary">
                <RadioGroupPrimitive.Indicator>
                    <Check className="size-3 text-primary-foreground" />
                </RadioGroupPrimitive.Indicator>
            </span>
            <span className="min-w-0 flex-1">{children}</span>
        </RadioGroupPrimitive.Item>
    );
}

export { RadioCard, RadioCards };
