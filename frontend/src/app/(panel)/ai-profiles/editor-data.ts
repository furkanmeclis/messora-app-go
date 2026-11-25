import 'server-only';

import { api } from '@/lib/server/api';
import { can } from '@/lib/server/auth';
import type { Me, Schemas } from '@/lib/types';

export type EditorData = {
    providers: Schemas['AIProvider'][];
    models: Schemas['AIModel'][];
    numbers: Schemas['PhoneNumber'][];
    automations: { id: string; name: string }[];
    attributeKeys: string[];
    settings: Schemas['AISettings'];
};

/** What the profile editor offers; lists the user may not read stay empty. */
export async function loadEditorData(me: Me): Promise<EditorData> {
    const optional = <T>(perm: string, path: string, fallback: T) =>
        can(me, perm)
            ? api<T>(path)
                  .then((r) => r.data)
                  .catch(() => fallback)
            : Promise.resolve(fallback);
    const [providers, models, settings, numbers, automations, attributeKeys] =
        await Promise.all([
            api<Schemas['AIProvider'][]>('/ai/providers').then((r) => r.data),
            api<Schemas['AIModel'][]>('/ai/models').then((r) => r.data),
            api<Schemas['AISettings']>('/ai/settings').then((r) => r.data),
            optional<Schemas['PhoneNumber'][]>(
                'channels.read',
                '/channels/phone-numbers',
                [],
            ),
            optional<Schemas['Automation'][]>(
                'automations.read',
                '/automations',
                [],
            ),
            optional<string[]>('contacts.read', '/contacts/attribute-keys', []),
        ]);
    return {
        providers,
        models,
        settings,
        numbers,
        automations: automations.map((a) => ({ id: a.id, name: a.name })),
        attributeKeys,
    };
}
