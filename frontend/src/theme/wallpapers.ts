import type { Theme } from '@/lib/types';

/**
 * Desktop-mode backgrounds, drawn with theme colours so every wallpaper
 * follows the palette in light and dark.
 */
export const WALLPAPERS: Record<Theme['wallpaper'], string> = {
    aurora: [
        'radial-gradient(60% 50% at 15% 20%, color-mix(in oklch, var(--primary) 55%, transparent), transparent 70%)',
        'radial-gradient(50% 45% at 85% 25%, color-mix(in oklch, var(--chart-2) 45%, transparent), transparent 70%)',
        'radial-gradient(70% 60% at 60% 100%, color-mix(in oklch, var(--chart-4) 35%, transparent), transparent 70%)',
        'linear-gradient(160deg, color-mix(in oklch, var(--background) 85%, var(--primary)), var(--background))',
    ].join(','),
    dunes: [
        'radial-gradient(120% 60% at 50% 120%, color-mix(in oklch, var(--chart-3) 55%, transparent), transparent 60%)',
        'radial-gradient(90% 50% at 10% 110%, color-mix(in oklch, var(--primary) 40%, transparent), transparent 60%)',
        'linear-gradient(180deg, color-mix(in oklch, var(--background) 70%, var(--chart-3)), var(--background) 55%)',
    ].join(','),
    grid: [
        'linear-gradient(color-mix(in oklch, var(--foreground) 7%, transparent) 1px, transparent 1px)',
        'linear-gradient(90deg, color-mix(in oklch, var(--foreground) 7%, transparent) 1px, transparent 1px)',
        'radial-gradient(80% 60% at 50% 0%, color-mix(in oklch, var(--primary) 25%, transparent), transparent 70%)',
        'var(--background)',
    ].join(','),
    mesh: [
        'radial-gradient(at 0% 0%, color-mix(in oklch, var(--primary) 50%, transparent) 0, transparent 50%)',
        'radial-gradient(at 100% 0%, color-mix(in oklch, var(--chart-5) 40%, transparent) 0, transparent 50%)',
        'radial-gradient(at 100% 100%, color-mix(in oklch, var(--chart-2) 45%, transparent) 0, transparent 50%)',
        'radial-gradient(at 0% 100%, color-mix(in oklch, var(--chart-3) 40%, transparent) 0, transparent 50%)',
        'var(--background)',
    ].join(','),
    plain: 'linear-gradient(var(--muted), var(--muted))',
    waves: [
        'repeating-radial-gradient(circle at 50% 130%, transparent 0 38px, color-mix(in oklch, var(--primary) 12%, transparent) 38px 40px)',
        'linear-gradient(180deg, var(--background), color-mix(in oklch, var(--background) 80%, var(--primary)))',
    ].join(','),
};

/** Extra sizing for wallpapers made of repeating lines. */
export const WALLPAPER_SIZE: Partial<Record<Theme['wallpaper'], string>> = {
    grid: '32px 32px, 32px 32px, 100% 100%, 100% 100%',
};
