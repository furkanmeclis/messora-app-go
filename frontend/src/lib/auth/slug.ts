/**
 * Workspace address rules (`<slug>.messora.tech`). The API checks the same
 * rules on sign-up; the form checks them first so people see problems while
 * they type.
 */

export const SLUG_MIN = 3;
export const SLUG_MAX = 30;

/** Sub-domains the platform keeps for itself. */
export const RESERVED_SLUGS: readonly string[] = [
    'www',
    'admin',
    'api',
    'app',
    'hooks',
    'dokploy',
    'cdn',
    'mail',
    'docs',
    'status',
    'help',
    'support',
];

export type SlugError =
    | 'required'
    | 'too_short'
    | 'too_long'
    | 'invalid_chars'
    | 'edge_hyphen'
    | 'double_hyphen'
    | 'reserved';

const TURKISH: Record<string, string> = {
    ç: 'c',
    ğ: 'g',
    ı: 'i',
    ö: 'o',
    ş: 's',
    ü: 'u',
};

/** Lower case with Turkish letters and accents folded to a–z. */
function fold(input: string): string {
    return input
        .toLocaleLowerCase('tr')
        .replace(/[çğıöşü]/g, (c) => TURKISH[c] ?? c)
        .normalize('NFKD')
        .replace(/[̀-ͯ]/g, '');
}

/**
 * Cleans what someone types into the address field: lower case, spaces,
 * dots and underscores become hyphens, anything else outside a–z0–9- is
 * dropped. A trailing hyphen stays so typing "acme-" keeps working.
 */
export function sanitizeSlugInput(input: string): string {
    return fold(input)
        .replace(/[\s_.]+/g, '-')
        .replace(/[^a-z0-9-]/g, '')
        .slice(0, SLUG_MAX);
}

/** A suggested address for a company name ("Easy Trade A.Ş." → "easy-trade-a-s"). */
export function slugify(name: string): string {
    return fold(name)
        .replace(/[^a-z0-9]+/g, '-')
        .replace(/^-+|-+$/g, '')
        .slice(0, SLUG_MAX)
        .replace(/-+$/, '');
}

/** The first rule the address breaks, or null when it is valid. */
export function validateSlug(slug: string): SlugError | null {
    if (!slug) return 'required';
    if (!/^[a-z0-9-]+$/.test(slug)) return 'invalid_chars';
    if (slug.length < SLUG_MIN) return 'too_short';
    if (slug.length > SLUG_MAX) return 'too_long';
    if (slug.startsWith('-') || slug.endsWith('-')) return 'edge_hyphen';
    if (slug.includes('--')) return 'double_hyphen';
    if (RESERVED_SLUGS.includes(slug)) return 'reserved';
    return null;
}
