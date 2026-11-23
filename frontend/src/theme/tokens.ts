/** Colour tokens a theme defines (mirrors the API's allowed list). */
export const TOKENS = [
    'background',
    'foreground',
    'card',
    'card-foreground',
    'popover',
    'popover-foreground',
    'primary',
    'primary-foreground',
    'secondary',
    'secondary-foreground',
    'muted',
    'muted-foreground',
    'accent',
    'accent-foreground',
    'destructive',
    'success',
    'warning',
    'border',
    'input',
    'ring',
    'sidebar',
    'sidebar-foreground',
    'sidebar-primary',
    'sidebar-primary-foreground',
    'sidebar-accent',
    'sidebar-accent-foreground',
    'sidebar-border',
    'chart-1',
    'chart-2',
    'chart-3',
    'chart-4',
    'chart-5',
] as const;

export type Token = (typeof TOKENS)[number];
export type Palette = Record<Token, string>;

/** Groups for the theme editor. */
export const TOKEN_GROUPS: Record<
    'surfaces' | 'brand' | 'feedback' | 'navigation' | 'charts',
    Token[]
> = {
    surfaces: [
        'background',
        'foreground',
        'card',
        'card-foreground',
        'popover',
        'popover-foreground',
        'muted',
        'muted-foreground',
        'border',
        'input',
    ],
    brand: [
        'primary',
        'primary-foreground',
        'secondary',
        'secondary-foreground',
        'accent',
        'accent-foreground',
        'ring',
    ],
    feedback: ['destructive', 'success', 'warning'],
    navigation: [
        'sidebar',
        'sidebar-foreground',
        'sidebar-primary',
        'sidebar-primary-foreground',
        'sidebar-accent',
        'sidebar-accent-foreground',
        'sidebar-border',
    ],
    charts: ['chart-1', 'chart-2', 'chart-3', 'chart-4', 'chart-5'],
};

/** Text tokens and the surface they sit on, for contrast warnings. */
export const CONTRAST_PAIRS: [Token, Token][] = [
    ['foreground', 'background'],
    ['card-foreground', 'card'],
    ['popover-foreground', 'popover'],
    ['primary-foreground', 'primary'],
    ['secondary-foreground', 'secondary'],
    ['muted-foreground', 'background'],
    ['accent-foreground', 'accent'],
    ['sidebar-foreground', 'sidebar'],
    ['sidebar-primary-foreground', 'sidebar-primary'],
    ['sidebar-accent-foreground', 'sidebar-accent'],
];

/** Same rule as the API: hex or oklch() only. */
export const COLOUR_RE =
    /^(#[0-9a-fA-F]{3}|#[0-9a-fA-F]{6}|#[0-9a-fA-F]{8}|oklch\(\s*[0-9]{1,3}(\.[0-9]{1,4})?%?\s+[0-9](\.[0-9]{1,4})?\s+[0-9]{1,3}(\.[0-9]{1,4})?(\s*\/\s*[0-9]{1,3}(\.[0-9]{1,4})?%?)?\s*\))$/;
