'use client';

import * as React from 'react';
import { cn } from 'cn';
import {
    hsvToRgb,
    parseColour,
    rgbToHsv,
    toHex,
    type HSV,
} from '@/theme/colour';
import { Popover, PopoverContent, PopoverTrigger } from './popover';

/** Drag handling shared by the square and the hue strip. */
function useDrag(
    onMove: (x: number, y: number) => void,
): React.PointerEventHandler<HTMLDivElement> {
    return (e) => {
        const el = e.currentTarget;
        el.setPointerCapture(e.pointerId);
        const move = (ev: PointerEvent | React.PointerEvent) => {
            const r = el.getBoundingClientRect();
            onMove(
                Math.min(1, Math.max(0, (ev.clientX - r.left) / r.width)),
                Math.min(1, Math.max(0, (ev.clientY - r.top) / r.height)),
            );
        };
        move(e);
        const up = () => {
            el.removeEventListener('pointermove', move);
            el.removeEventListener('pointerup', up);
        };
        el.addEventListener('pointermove', move);
        el.addEventListener('pointerup', up);
    };
}

/**
 * Colour picker: saturation/value square, hue strip, hex field and
 * suggestions. Emits #rrggbb.
 */
function ColorPicker({
    value,
    onChange,
    swatches = [],
    className,
    'aria-label': ariaLabel,
}: {
    value: string;
    onChange: (hex: string) => void;
    swatches?: string[];
    className?: string;
    'aria-label'?: string;
}) {
    const rgb = parseColour(value) ?? { r: 0, g: 0, b: 0, a: 1 };
    const [hsv, setHsv] = React.useState<HSV>(() => rgbToHsv(rgb));
    const [hexDraft, setHexDraft] = React.useState<string | null>(null);
    const hex = toHex({ ...rgb, a: 1 });

    const emit = (next: HSV) => {
        setHsv(next);
        onChange(toHex(hsvToRgb(next)));
    };
    const dragSquare = useDrag((x, y) => emit({ ...hsv, s: x, v: 1 - y }));
    const dragHue = useDrag((x) => emit({ ...hsv, h: x * 360 }));
    const hueColour = toHex(hsvToRgb({ h: hsv.h, s: 1, v: 1 }));

    const keySquare = (e: React.KeyboardEvent) => {
        const d = e.shiftKey ? 0.1 : 0.02;
        const k = {
            ArrowLeft: { s: -d, v: 0 },
            ArrowRight: { s: d, v: 0 },
            ArrowUp: { s: 0, v: d },
            ArrowDown: { s: 0, v: -d },
        }[e.key];
        if (!k) return;
        e.preventDefault();
        emit({
            ...hsv,
            s: Math.min(1, Math.max(0, hsv.s + k.s)),
            v: Math.min(1, Math.max(0, hsv.v + k.v)),
        });
    };

    return (
        <Popover
            onOpenChange={(o) => {
                if (o) {
                    setHsv(rgbToHsv(rgb));
                    setHexDraft(null);
                }
            }}
        >
            <PopoverTrigger asChild>
                <button
                    type="button"
                    aria-label={ariaLabel}
                    className={cn(
                        'size-8 shrink-0 rounded-lg border shadow-sm ring-offset-2 ring-offset-background transition-transform outline-none hover:scale-105 focus-visible:ring-3 focus-visible:ring-ring/60',
                        className,
                    )}
                    style={{
                        background: `linear-gradient(${value}, ${value}), repeating-conic-gradient(#ccc 0 25%, #fff 0 50%) 0 0 / 8px 8px`,
                    }}
                />
            </PopoverTrigger>
            <PopoverContent className="w-64 space-y-3">
                <div
                    role="slider"
                    tabIndex={0}
                    aria-label="Saturation and brightness"
                    aria-valuenow={Math.round(hsv.v * 100)}
                    aria-valuetext={hex}
                    onPointerDown={dragSquare}
                    onKeyDown={keySquare}
                    className="relative h-36 cursor-crosshair touch-none rounded-lg outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
                    style={{
                        background: `linear-gradient(to top, #000, transparent), linear-gradient(to right, #fff, ${hueColour})`,
                    }}
                >
                    <span
                        className="pointer-events-none absolute size-4 -translate-x-1/2 -translate-y-1/2 rounded-full border-2 border-white shadow-[0_0_0_1px_rgba(0,0,0,.3)]"
                        style={{
                            left: `${hsv.s * 100}%`,
                            top: `${(1 - hsv.v) * 100}%`,
                            background: hex,
                        }}
                    />
                </div>
                <div
                    role="slider"
                    tabIndex={0}
                    aria-label="Hue"
                    aria-valuemin={0}
                    aria-valuemax={360}
                    aria-valuenow={Math.round(hsv.h)}
                    onPointerDown={dragHue}
                    onKeyDown={(e) => {
                        const d =
                            e.key === 'ArrowRight'
                                ? 5
                                : e.key === 'ArrowLeft'
                                  ? -5
                                  : 0;
                        if (!d) return;
                        e.preventDefault();
                        emit({ ...hsv, h: (hsv.h + d + 360) % 360 });
                    }}
                    className="relative h-3 cursor-pointer touch-none rounded-full outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
                    style={{
                        background:
                            'linear-gradient(to right,#f00,#ff0,#0f0,#0ff,#00f,#f0f,#f00)',
                    }}
                >
                    <span
                        className="pointer-events-none absolute top-1/2 size-4 -translate-x-1/2 -translate-y-1/2 rounded-full border-2 border-white shadow-[0_0_0_1px_rgba(0,0,0,.3)]"
                        style={{
                            left: `${(hsv.h / 360) * 100}%`,
                            background: hueColour,
                        }}
                    />
                </div>
                <div className="flex items-center gap-2">
                    <span
                        className="size-8 shrink-0 rounded-md border"
                        style={{ background: hex }}
                    />
                    <input
                        aria-label="Hex"
                        spellCheck={false}
                        value={hexDraft ?? hex}
                        onChange={(e) => {
                            const v = e.target.value.trim();
                            setHexDraft(v);
                            const c = /^#?[0-9a-f]{6}$/i.test(v)
                                ? parseColour(v.startsWith('#') ? v : `#${v}`)
                                : null;
                            if (c) {
                                setHsv(rgbToHsv(c));
                                onChange(toHex(c));
                            }
                        }}
                        onBlur={() => setHexDraft(null)}
                        className="h-8 w-full rounded-lg border border-input bg-transparent px-2 font-mono text-sm uppercase outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50"
                    />
                </div>
                {swatches.length > 0 && (
                    <div className="flex flex-wrap gap-1.5">
                        {swatches.map((s) => (
                            <button
                                key={s}
                                type="button"
                                aria-label={s}
                                onClick={() => {
                                    const c = parseColour(s);
                                    if (c) {
                                        setHsv(rgbToHsv(c));
                                        onChange(toHex({ ...c, a: 1 }));
                                    }
                                }}
                                className="size-6 rounded-md border transition-transform hover:scale-110"
                                style={{ background: s }}
                            />
                        ))}
                    </div>
                )}
            </PopoverContent>
        </Popover>
    );
}

export { ColorPicker };
