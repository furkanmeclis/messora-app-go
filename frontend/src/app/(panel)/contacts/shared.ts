import type { Contact } from '@/lib/types';

/** How a contact is shown: saved name, WhatsApp profile name or a fallback. */
export function contactName(c: Pick<Contact, 'name' | 'profile_name'>) {
    return c.name || c.profile_name || 'Unnamed';
}

export const UUID_RE =
    /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;
