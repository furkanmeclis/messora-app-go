'use client';

import * as React from 'react';
import { cn } from 'cn';
import { Slider as SliderPrimitive } from 'radix-ui';

function Slider({
    className,
    ...props
}: React.ComponentProps<typeof SliderPrimitive.Root>) {
    const count = (props.value ?? props.defaultValue ?? [0]).length;
    return (
        <SliderPrimitive.Root
            data-slot="slider"
            className={cn(
                'relative flex w-full touch-none items-center select-none data-disabled:opacity-50',
                className,
            )}
            {...props}
        >
            <SliderPrimitive.Track className="relative h-1.5 grow overflow-hidden rounded-full bg-muted">
                <SliderPrimitive.Range className="absolute h-full bg-primary" />
            </SliderPrimitive.Track>
            {Array.from({ length: count }, (_, i) => (
                <SliderPrimitive.Thumb
                    key={i}
                    className="block size-4 rounded-full border-2 border-primary bg-background shadow-sm transition-[box-shadow,transform] outline-none hover:scale-110 focus-visible:ring-4 focus-visible:ring-ring/40"
                />
            ))}
        </SliderPrimitive.Root>
    );
}

export { Slider };
