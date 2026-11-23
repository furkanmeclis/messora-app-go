import type { Palette } from './tokens';

export const PRESETS = [
    'messora',
    'midnight',
    'sand',
    'forest',
    'graphite',
    'ocean',
    'rose',
] as const;
export type Preset = (typeof PRESETS)[number];

type Seed = {
    /** Brand hue, chroma and lightness (light scheme). */
    h: number;
    c: number;
    l: number;
    /** Neutral tint: hue and chroma of greys. */
    nh: number;
    nc: number;
    /** Second hue for highlights and charts. */
    ah: number;
};

const f = (n: number) => Number(n.toFixed(3));
const ok = (l: number, c: number, h: number, a?: number) =>
    `oklch(${f(l)} ${f(Math.max(0, c))} ${f(((h % 360) + 360) % 360)}${a === undefined ? '' : ` / ${a}%`})`;

function build(s: Seed): { light: Palette; dark: Palette } {
    const onBrand = s.l > 0.7 ? ok(0.2, 0.03, s.h) : ok(0.985, 0.012, s.h);
    const charts = (l: number, c: number) => ({
        'chart-1': ok(l, c, s.h),
        'chart-2': ok(l + 0.05, c * 0.85, s.ah),
        'chart-3': ok(l + 0.12, c * 0.9, s.h + 110),
        'chart-4': ok(l, c, s.h + 180),
        'chart-5': ok(l + 0.04, c, s.h - 70),
    });
    const light: Palette = {
        background: ok(0.992, s.nc * 0.35, s.nh),
        foreground: ok(0.19, s.nc * 2, s.nh),
        card: ok(1, 0, 0),
        'card-foreground': ok(0.19, s.nc * 2, s.nh),
        popover: ok(1, 0, 0),
        'popover-foreground': ok(0.19, s.nc * 2, s.nh),
        primary: ok(s.l, s.c, s.h),
        'primary-foreground': onBrand,
        secondary: ok(0.958, s.nc * 1.4, s.nh),
        'secondary-foreground': ok(0.25, s.nc * 2, s.nh),
        muted: ok(0.962, s.nc * 1.2, s.nh),
        'muted-foreground': ok(0.5, s.nc * 2.5, s.nh),
        accent: ok(0.95, Math.min(0.035, s.c * 0.25), s.h),
        'accent-foreground': ok(0.36, s.c * 0.75, s.h),
        destructive: ok(0.577, 0.215, 27),
        success: ok(0.6, 0.14, 155),
        warning: ok(0.74, 0.155, 72),
        border: ok(0.912, s.nc * 1.4, s.nh),
        input: ok(0.89, s.nc * 1.6, s.nh),
        ring: ok(Math.min(0.75, s.l + 0.06), s.c, s.h),
        sidebar: ok(0.976, s.nc * 1.6, s.nh),
        'sidebar-foreground': ok(0.24, s.nc * 2, s.nh),
        'sidebar-primary': ok(s.l, s.c, s.h),
        'sidebar-primary-foreground': onBrand,
        'sidebar-accent': ok(0.935, Math.min(0.045, s.c * 0.3), s.h),
        'sidebar-accent-foreground': ok(0.3, s.c * 0.6, s.h),
        'sidebar-border': ok(0.918, s.nc * 1.4, s.nh),
        ...charts(0.6, Math.max(0.12, s.c)),
    };
    const dl = Math.min(0.78, Math.max(0.62, s.l + 0.15));
    const dark: Palette = {
        background: ok(0.155, s.nc * 1.8, s.nh),
        foreground: ok(0.965, s.nc * 0.6, s.nh),
        card: ok(0.195, s.nc * 2, s.nh),
        'card-foreground': ok(0.965, s.nc * 0.6, s.nh),
        popover: ok(0.215, s.nc * 2, s.nh),
        'popover-foreground': ok(0.965, s.nc * 0.6, s.nh),
        primary: ok(dl, s.c * 0.85, s.h),
        'primary-foreground': ok(0.17, 0.03, s.h),
        secondary: ok(0.26, s.nc * 2, s.nh),
        'secondary-foreground': ok(0.95, s.nc, s.nh),
        muted: ok(0.245, s.nc * 2, s.nh),
        'muted-foreground': ok(0.7, s.nc * 2, s.nh),
        accent: ok(0.29, Math.min(0.07, s.c * 0.4), s.h),
        'accent-foreground': ok(0.92, Math.min(0.04, s.c * 0.3), s.h),
        destructive: ok(0.68, 0.19, 22),
        success: ok(0.7, 0.14, 155),
        warning: ok(0.8, 0.145, 78),
        border: ok(1, 0, 0, 10),
        input: ok(1, 0, 0, 14),
        ring: ok(dl, s.c * 0.85, s.h),
        sidebar: ok(0.18, s.nc * 2, s.nh),
        'sidebar-foreground': ok(0.94, s.nc * 0.6, s.nh),
        'sidebar-primary': ok(dl, s.c * 0.85, s.h),
        'sidebar-primary-foreground': ok(0.17, 0.03, s.h),
        'sidebar-accent': ok(0.26, Math.min(0.06, s.c * 0.35), s.h),
        'sidebar-accent-foreground': ok(0.95, s.nc, s.nh),
        'sidebar-border': ok(1, 0, 0, 8),
        ...charts(0.68, Math.max(0.12, s.c * 0.9)),
    };
    return { light, dark };
}

const SEEDS: Record<Preset, Seed> = {
    messora: { h: 277, c: 0.205, l: 0.52, nh: 277, nc: 0.006, ah: 172 },
    midnight: { h: 258, c: 0.17, l: 0.54, nh: 262, nc: 0.018, ah: 305 },
    sand: { h: 42, c: 0.14, l: 0.6, nh: 70, nc: 0.014, ah: 150 },
    forest: { h: 158, c: 0.12, l: 0.5, nh: 140, nc: 0.01, ah: 85 },
    graphite: { h: 265, c: 0.012, l: 0.26, nh: 265, nc: 0.003, ah: 230 },
    ocean: { h: 228, c: 0.15, l: 0.55, nh: 222, nc: 0.012, ah: 190 },
    rose: { h: 4, c: 0.185, l: 0.585, nh: 12, nc: 0.01, ah: 330 },
};

export const PALETTES: Record<Preset, { light: Palette; dark: Palette }> =
    Object.fromEntries(
        Object.entries(SEEDS).map(([k, s]) => [k, build(s)]),
    ) as Record<Preset, { light: Palette; dark: Palette }>;

// Character beyond the shared recipe.
// Midnight: a deep navy sidebar even in the light scheme.
Object.assign(PALETTES.midnight.light, {
    sidebar: ok(0.24, 0.06, 262),
    'sidebar-foreground': ok(0.93, 0.015, 262),
    'sidebar-accent': ok(0.32, 0.08, 262),
    'sidebar-accent-foreground': ok(0.98, 0.01, 262),
    'sidebar-border': ok(1, 0, 0, 8),
    'sidebar-primary': ok(0.7, 0.15, 258),
    'sidebar-primary-foreground': ok(0.17, 0.03, 258),
});
// Graphite: monochrome, the dark primary is near-white.
Object.assign(PALETTES.graphite.dark, {
    primary: ok(0.93, 0.005, 265),
    'primary-foreground': ok(0.19, 0.005, 265),
    ring: ok(0.7, 0.005, 265),
    'sidebar-primary': ok(0.93, 0.005, 265),
    'sidebar-primary-foreground': ok(0.19, 0.005, 265),
});
Object.assign(PALETTES.graphite.light, {
    accent: ok(0.95, 0.004, 265),
    'accent-foreground': ok(0.25, 0.008, 265),
    'sidebar-accent': ok(0.93, 0.004, 265),
    'sidebar-accent-foreground': ok(0.22, 0.008, 265),
});
