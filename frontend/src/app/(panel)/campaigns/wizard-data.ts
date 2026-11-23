import 'server-only';

import { api } from '@/lib/server/api';
import { can } from '@/lib/server/auth';
import type {
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
};

/** Everything the campaign wizard offers, limited to what the role sees. */
export async function loadWizardData(me: Me): Promise<WizardData> {
    const maybe = <T>(perm: string, path: string): Promise<T | null> =>
        can(me, perm)
            ? api<T>(path)
                  .then((r) => r.data)
                  .catch(() => null)
            : Promise.resolve(null);
    const [numbers, templates, segments, groups, tags] = await Promise.all([
        maybe<PhoneNumber[]>('channels.read', '/channels/phone-numbers'),
        maybe<Template[]>('templates.read', '/templates?status=APPROVED'),
        maybe<Segment[]>('segments.read', '/segments'),
        maybe<ContactGroup[]>('contacts.read', '/groups'),
        maybe<Tag[]>('contacts.read', '/tags'),
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
    };
}
