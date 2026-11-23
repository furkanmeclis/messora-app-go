import type { Theme } from '@/lib/types';
import { PALETTES, type Preset } from './presets';
import { COLOUR_RE, TOKENS, type Palette } from './tokens';

export const FONTS: Record<Theme['font'], { label: string; family: string }> = {
    geist: { label: 'Geist', family: "'Geist Variable'" },
    inter: { label: 'Inter', family: "'Inter Variable'" },
    manrope: { label: 'Manrope', family: "'Manrope Variable'" },
    'plus-jakarta': {
        label: 'Plus Jakarta Sans',
        family: "'Plus Jakarta Sans Variable'",
    },
    'ibm-plex': { label: 'IBM Plex Sans', family: "'IBM Plex Sans Variable'" },
    'space-grotesk': {
        label: 'Space Grotesk',
        family: "'Space Grotesk Variable'",
    },
};

/** Root font size per density; every rem-based size follows it. */
export const DENSITY_PX: Record<Theme['density'], number> = {
    compact: 15,
    comfortable: 16,
    spacious: 17,
};

/** The full palette of a scheme: preset plus valid overrides. */
export function resolvePalette(
    theme: Theme,
    scheme: 'light' | 'dark',
): Palette {
    const base = PALETTES[theme.preset as Preset] ?? PALETTES.messora;
    const out = { ...base[scheme] };
    const overrides = theme[scheme] ?? {};
    for (const t of TOKENS) {
        const v = overrides[t];
        if (v && COLOUR_RE.test(v)) out[t] = v;
    }
    return out;
}

function declarations(p: Palette): string {
    return TOKENS.map((t) => `--${t}:${p[t]};`).join('');
}

/**
 * Style sheet for a theme. Values are re-checked here even though the API
 * validated them: they are written into a <style> element.
 */
export function themeCss(
    theme: Theme,
    scope: { light: string; dark: string } = {
        light: ':root',
        dark: '.dark',
    },
): string {
    const font = FONTS[theme.font] ?? FONTS.geist;
    const radius = Math.min(2, Math.max(0, Number(theme.radius) || 0));
    const shared =
        `--radius:${radius}rem;` +
        `--app-font:${font.family},ui-sans-serif,system-ui,sans-serif;`;
    return (
        `${scope.light}{${shared}${declarations(resolvePalette(theme, 'light'))}}` +
        `${scope.dark}{${declarations(resolvePalette(theme, 'dark'))}}` +
        (scope.light === ':root'
            ? `html{font-size:${DENSITY_PX[theme.density] ?? 16}px}`
            : '')
    );
}
