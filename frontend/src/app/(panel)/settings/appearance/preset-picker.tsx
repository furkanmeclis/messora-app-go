'use client';

import { Check } from 'lucide-react';
import { useT } from '@/i18n/client';
import type { Theme } from '@/lib/types';
import { cn } from '@/lib/utils';
import { PALETTES, PRESETS } from '@/theme/presets';

/** Theme presets as swatch cards. */
export function PresetPicker({
    value,
    onChange,
    scheme,
    disabled,
}: {
    value: Theme['preset'];
    onChange: (p: Theme['preset']) => void;
    scheme: 'light' | 'dark';
    disabled?: boolean;
}) {
    const t = useT();
    return (
        <div
            role="radiogroup"
            className="grid grid-cols-2 gap-2 sm:grid-cols-4"
        >
            {PRESETS.map((p) => {
                const pal = PALETTES[p][scheme];
                const on = p === value;
                return (
                    <button
                        key={p}
                        type="button"
                        role="radio"
                        aria-checked={on}
                        disabled={disabled}
                        onClick={() => onChange(p)}
                        className={cn(
                            'group relative overflow-hidden rounded-xl border bg-card p-1.5 text-left transition-all outline-none hover:-translate-y-0.5 hover:shadow-md focus-visible:ring-3 focus-visible:ring-ring/50 disabled:pointer-events-none disabled:opacity-60',
                            on &&
                                'border-primary shadow-[0_0_0_1px_var(--primary)]',
                        )}
                    >
                        <span
                            className="flex h-14 overflow-hidden rounded-lg border"
                            style={{
                                background: pal.background,
                                borderColor: pal.border,
                            }}
                        >
                            <span
                                className="w-1/4"
                                style={{ background: pal.sidebar }}
                            />
                            <span className="flex flex-1 flex-col justify-center gap-1 p-1.5">
                                <span
                                    className="h-1.5 w-3/4 rounded-full"
                                    style={{
                                        background: pal.foreground,
                                        opacity: 0.8,
                                    }}
                                />
                                <span
                                    className="h-1.5 w-1/2 rounded-full"
                                    style={{
                                        background: pal['muted-foreground'],
                                        opacity: 0.6,
                                    }}
                                />
                                <span className="mt-0.5 flex gap-1">
                                    <span
                                        className="h-2.5 w-6 rounded"
                                        style={{ background: pal.primary }}
                                    />
                                    <span
                                        className="h-2.5 w-3 rounded"
                                        style={{ background: pal['chart-2'] }}
                                    />
                                </span>
                            </span>
                        </span>
                        <span className="flex items-center justify-between px-1 pt-1.5 pb-0.5 text-xs font-medium">
                            {t(`appearance.presetNames.${p}`)}
                            {on && <Check className="size-3.5 text-primary" />}
                        </span>
                    </button>
                );
            })}
        </div>
    );
}
