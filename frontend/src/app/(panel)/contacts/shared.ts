import type { Contact } from '@/lib/types';

/**
 * How a contact is shown: saved name, WhatsApp profile name or a fallback
 * (pass the translated contacts.unnamed).
 */
export function contactName(
    c: Pick<Contact, 'name' | 'profile_name'>,
    fallback = 'Unnamed',
) {
    return c.name || c.profile_name || fallback;
}

export const UUID_RE =
    /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;
