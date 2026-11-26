import 'server-only';

import { notFound } from 'next/navigation';
import { api, ApiError } from '@/lib/server/api';
import { can } from '@/lib/server/auth';
import type { Automation, Me, Schemas, Template, User } from '@/lib/types';

const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;

/** The automation, or the 404 page when it does not exist. */
export async function loadAutomation(id: string): Promise<Automation> {
    if (!UUID.test(id)) notFound();
    try {
        return (await api<Automation>(`/automations/${id}`)).data;
    } catch (err) {
        if (err instanceof ApiError && err.status === 404) notFound();
        throw err;
    }
}

export type Option = { id: string; label: string };

/** Picker data the user may read; anything else is simply left out. */
export type EditorOptions = {
    phones: Option[];
    campaigns: Option[];
    tags: string[];
    groups: Option[];
    templates: {
        name: string;
        language: string;
        parameter_format: string;
        components: Record<string, unknown>[];
    }[];
    /** Custom attribute keys for {{attr:key}} mapping. */
    attributeKeys: string[];
    /** Null when the user list is not visible (assign to me / unassign). */
    users: Option[] | null;
    me: Option;
    canReadTemplates: boolean;
    /**
     * The company's AI switch (AI steps fail while it is off); null when
     * the user cannot read the AI settings.
     */
    aiEnabled: boolean | null;
};

function optional<T>(
    allowed: boolean,
    load: () => Promise<T>,
    fallback: T,
): Promise<T> {
    return allowed ? load().catch(() => fallback) : Promise.resolve(fallback);
}

/** Names for trigger summaries (phone numbers and campaigns). */
export async function loadNames(me: Me) {
    const [phones, campaigns] = await Promise.all([
        optional(
            can(me, 'channels.read'),
            async () =>
                (
                    await api<Schemas['PhoneNumber'][]>(
                        '/channels/phone-numbers',
                    )
                ).data.map((p) => ({
                    id: p.id,
                    label: `${p.display_phone_number}${p.verified_name ? ` (${p.verified_name})` : ''}`,
                })),
            [] as Option[],
        ),
        optional(
            can(me, 'campaigns.read'),
            async () =>
                (
                    await api<Schemas['Campaign'][]>('/campaigns?limit=200')
                ).data.map((c) => ({ id: c.id, label: c.name })),
            [] as Option[],
        ),
    ]);
    return { phones, campaigns };
}

export async function loadEditorOptions(me: Me): Promise<EditorOptions> {
    const contacts = can(me, 'contacts.read');
    const [names, tags, groups, templates, users, attributeKeys, aiEnabled] =
        await Promise.all([
            loadNames(me),
            optional(
                contacts,
                async () =>
                    (await api<Schemas['Tag'][]>('/tags')).data.map(
                        (t) => t.name,
                    ),
                [] as string[],
            ),
            optional(
                contacts,
                async () =>
                    (await api<Schemas['Group'][]>('/groups')).data.map(
                        (g) => ({
                            id: g.id,
                            label: g.name,
                        }),
                    ),
                [] as Option[],
            ),
            optional(
                can(me, 'templates.read'),
                async () =>
                    (
                        await api<Template[]>('/templates?status=APPROVED')
                    ).data.map((t) => ({
                        name: t.name,
                        language: t.language,
                        parameter_format: t.parameter_format,
                        components: t.components,
                    })),
                [] as EditorOptions['templates'],
            ),
            optional<Option[] | null>(
                can(me, 'users.manage'),
                async () =>
                    (await api<User[]>('/users')).data
                        .filter((u) => u.status === 'active')
                        .map((u) => ({ id: u.id, label: u.name })),
                null,
            ),
            optional(
                can(me, 'contacts.read'),
                async () =>
                    (await api<string[]>('/contacts/attribute-keys')).data,
                [] as string[],
            ),
            optional<boolean | null>(
                can(me, 'ai.read'),
                async () =>
                    (await api<Schemas['AISettings']>('/ai/settings')).data
                        .enabled,
                null,
            ),
        ]);
    return {
        ...names,
        aiEnabled,
        tags,
        groups,
        templates,
        users,
        me: { id: me.user.id, label: me.user.name },
        canReadTemplates: can(me, 'templates.read'),
        attributeKeys,
    };
}
