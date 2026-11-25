import 'server-only';

import { api } from '@/lib/server/api';
import { can } from '@/lib/server/auth';
import type {
    Contact,
    ContactGroup,
    Me,
    PhoneNumber,
    Segment,
    Tag,
    Template,
} from '@/lib/types';

export type WizardData = {
    /** Null when the user may not list numbers (channels.read). */
    numbers: PhoneNumber[] | null;
    templates: Template[];
    segments: Segment[] | null;
    groups: ContactGroup[] | null;
    tags: Tag[] | null;
    timezone: string;
    canStart: boolean;
    canEstimate: boolean;
    /** May open Settings › Subscription & consent (settings.read). */
    canConsentSettings: boolean;
    /** Custom attribute keys for {{attr:key}} mapping. */
    attributeKeys: string[];
    /** A contact whose values the preview shows. */
    sample: Contact | null;
};

/** Everything the campaign wizard offers, limited to what the role sees. */
export async function loadWizardData(me: Me): Promise<WizardData> {
    const maybe = <T>(perm: string, path: string): Promise<T | null> =>
        can(me, perm)
            ? api<T>(path)
                  .then((r) => r.data)
                  .catch(() => null)
            : Promise.resolve(null);
    const [numbers, templates, segments, groups, tags, attributeKeys, sample] =
        await Promise.all([
            maybe<PhoneNumber[]>('channels.read', '/channels/phone-numbers'),
            maybe<Template[]>('templates.read', '/templates?status=APPROVED'),
            maybe<Segment[]>('segments.read', '/segments'),
            maybe<ContactGroup[]>('contacts.read', '/groups'),
            maybe<Tag[]>('contacts.read', '/tags'),
            maybe<string[]>('contacts.read', '/contacts/attribute-keys'),
            maybe<Contact[]>(
                'contacts.read',
                '/contacts?limit=1&opted_out=false',
            ),
        ]);
    return {
        numbers,
        templates: templates ?? [],
        segments,
        groups,
        tags,
        timezone: me.user.timezone || 'UTC',
        canStart: can(me, 'campaigns.start'),
        canEstimate: can(me, 'segments.read'),
        canConsentSettings: can(me, 'settings.read'),
        attributeKeys: attributeKeys ?? [],
        sample: sample?.[0] ?? null,
    };
}
