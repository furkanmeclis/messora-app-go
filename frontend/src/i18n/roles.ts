import type { Translate } from './translate';

const BUILT_IN = ['owner', 'admin', 'manager', 'agent'] as const;
type BuiltInRole = (typeof BUILT_IN)[number];

function isBuiltIn(key: string): key is BuiltInRole {
    return (BUILT_IN as readonly string[]).includes(key);
}

/**
 * The label of a role: built-in roles are translated, custom roles show the
 * name their workspace gave them (or the key when no name is known).
 */
export function roleLabel(
    t: Translate,
    role: { key: string; name?: string | null } | string,
): string {
    const key = typeof role === 'string' ? role : role.key;
    if (isBuiltIn(key)) return t(`settings.security.roles.${key}`);
    return (typeof role === 'string' ? undefined : role.name) || key;
}
