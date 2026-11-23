'use server';

import { revalidatePath } from 'next/cache';
import { api } from '@/lib/server/api';
import { field, runAction } from '@/lib/server/actions';
import { getT } from '@/i18n/server';
import type {
    ActionResult,
    Contact,
    ContactGroup,
    ContactImport,
    Note,
    Segment,
    SegmentPreview,
    Schemas,
    Tag,
} from '@/lib/types';

const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;

function check(...ids: string[]) {
    for (const id of ids) if (!UUID.test(id)) throw new Error('invalid id');
}

/** Only the list filters the API knows are passed on. */
function contactQuery(raw: string, cursor?: string): string {
    const src = new URLSearchParams(raw);
    const q = new URLSearchParams({ limit: '50' });
    const text = (src.get('q') ?? '').trim().slice(0, 100);
    if (text) q.set('q', text);
    for (const k of ['tag_id', 'group_id', 'segment_id']) {
        const v = src.get(k);
        if (v && UUID.test(v)) q.set(k, v);
    }
    const opted = src.get('opted_out');
    if (opted === 'true' || opted === 'false') q.set('opted_out', opted);
    if (cursor && UUID.test(cursor)) q.set('cursor', cursor);
    return q.toString();
}

// ── Contacts ───────────────────────────────────────────────────────────────

export async function loadContactsAction(
    query: string,
    cursor: string,
): Promise<ActionResult<{ contacts: Contact[]; next?: string }>> {
    return runAction(async () => {
        const r = await api<Contact[]>(
            `/contacts?${contactQuery(query, cursor)}`,
        );
        return { contacts: r.data, next: r.nextCursor };
    });
}

export async function createContactAction(
    _prev: ActionResult<Contact> | null,
    form: FormData,
): Promise<ActionResult<Contact>> {
    const optional = (k: string) => field(form, k) || undefined;
    const res = await runAction(
        async () =>
            (
                await api<Contact>('/contacts', {
                    method: 'POST',
                    idempotencyKey: field(form, 'idempotency_key') || undefined,
                    body: {
                        phone: field(form, 'phone'),
                        name: optional('name'),
                        email: optional('email'),
                    },
                })
            ).data,
        (await getT())('contacts.toast.contactAdded'),
    );
    if (res.ok) revalidatePath('/contacts');
    return res;
}

export type ContactPatch = {
    name: string;
    email: string;
    var1: string;
    var2: string;
    var3: string;
    var4: string;
    var5: string;
    attributes: Record<string, string>;
    tags: string[];
    group_ids: string[];
};

export async function updateContactAction(
    id: string,
    patch: ContactPatch,
): Promise<ActionResult<Contact>> {
    check(id, ...patch.group_ids);
    const res = await runAction(
        async () =>
            (
                await api<Contact>(`/contacts/${id}`, {
                    method: 'PATCH',
                    body: patch,
                })
            ).data,
        (await getT())('contacts.toast.contactSaved'),
    );
    if (res.ok) {
        revalidatePath('/contacts');
        revalidatePath(`/contacts/${id}`);
    }
    return res;
}

export async function deleteContactAction(id: string): Promise<ActionResult> {
    check(id);
    const res = await runAction(
        () =>
            api(`/contacts/${id}`, { method: 'DELETE' }).then(() => undefined),
        (await getT())('contacts.toast.contactDeleted'),
    );
    if (res.ok) revalidatePath('/contacts');
    return res;
}

export async function consentAction(
    id: string,
    kind: 'opt_in' | 'opt_out',
    note: string,
): Promise<ActionResult<Contact>> {
    check(id);
    const res = await runAction(
        async () =>
            (
                await api<Contact>(`/contacts/${id}/consent`, {
                    method: 'POST',
                    body: { kind, note: note.trim() || undefined },
                })
            ).data,
        (await getT())(
            kind === 'opt_in'
                ? 'contacts.toast.optIn'
                : 'contacts.toast.optOut',
        ),
    );
    if (res.ok) revalidatePath(`/contacts/${id}`);
    return res;
}

export async function addContactNoteAction(
    id: string,
    body: string,
): Promise<ActionResult<Note>> {
    check(id);
    const res = await runAction(
        async () =>
            (
                await api<Note>(`/contacts/${id}/notes`, {
                    method: 'POST',
                    body: { body },
                })
            ).data,
        (await getT())('contacts.toast.noteAdded'),
    );
    if (res.ok) revalidatePath(`/contacts/${id}`);
    return res;
}

export async function deleteContactNoteAction(
    id: string,
    noteId: string,
): Promise<ActionResult> {
    check(id, noteId);
    const res = await runAction(
        () =>
            api(`/contacts/${id}/notes/${noteId}`, {
                method: 'DELETE',
            }).then(() => undefined),
        (await getT())('contacts.toast.noteDeleted'),
    );
    if (res.ok) revalidatePath(`/contacts/${id}`);
    return res;
}

// ── Groups ─────────────────────────────────────────────────────────────────

export async function groupMembersAction(
    groupId: string,
    action: 'add' | 'remove',
    contactIds: string[],
): Promise<ActionResult<number>> {
    check(groupId, ...contactIds);
    const res = await runAction(
        async () =>
            (
                await api<{ count: number }>(`/groups/${groupId}/members`, {
                    method: 'POST',
                    body: { action, contact_ids: contactIds },
                })
            ).data.count,
    );
    if (res.ok) {
        revalidatePath('/contacts');
        revalidatePath('/contacts/groups');
    }
    return res;
}

export async function saveGroupAction(
    id: string | null,
    _prev: ActionResult<ContactGroup> | null,
    form: FormData,
): Promise<ActionResult<ContactGroup>> {
    if (id) check(id);
    const res = await runAction(
        async () =>
            (
                await api<ContactGroup>(id ? `/groups/${id}` : '/groups', {
                    method: id ? 'PATCH' : 'POST',
                    body: {
                        name: field(form, 'name'),
                        description: field(form, 'description'),
                    },
                })
            ).data,
        (await getT())(
            id ? 'contacts.toast.groupSaved' : 'contacts.toast.groupCreated',
        ),
    );
    if (res.ok) revalidatePath('/contacts/groups');
    return res;
}

export async function deleteGroupAction(id: string): Promise<ActionResult> {
    check(id);
    const res = await runAction(
        () => api(`/groups/${id}`, { method: 'DELETE' }).then(() => undefined),
        (await getT())('contacts.toast.groupDeleted'),
    );
    if (res.ok) revalidatePath('/contacts/groups');
    return res;
}

// ── Tags ───────────────────────────────────────────────────────────────────

export async function saveTagAction(
    id: string | null,
    _prev: ActionResult<Tag> | null,
    form: FormData,
): Promise<ActionResult<Tag>> {
    if (id) check(id);
    const res = await runAction(
        async () =>
            (
                await api<Tag>(id ? `/tags/${id}` : '/tags', {
                    method: id ? 'PATCH' : 'POST',
                    body: {
                        name: field(form, 'name'),
                        color: field(form, 'color') || undefined,
                    },
                })
            ).data,
        (await getT())(
            id ? 'contacts.toast.tagSaved' : 'contacts.toast.tagCreated',
        ),
    );
    if (res.ok) revalidatePath('/contacts/tags');
    return res;
}

export async function deleteTagAction(id: string): Promise<ActionResult> {
    check(id);
    const res = await runAction(
        () => api(`/tags/${id}`, { method: 'DELETE' }).then(() => undefined),
        (await getT())('contacts.toast.tagDeleted'),
    );
    if (res.ok) revalidatePath('/contacts/tags');
    return res;
}

// ── Segments ───────────────────────────────────────────────────────────────

export async function previewSegmentAction(
    definition: unknown,
): Promise<ActionResult<SegmentPreview>> {
    return runAction(
        async () =>
            (
                await api<SegmentPreview>('/segments/preview', {
                    method: 'POST',
                    body: { definition },
                })
            ).data,
    );
}

export async function saveSegmentAction(
    id: string | null,
    input: { name: string; description: string; definition: unknown },
): Promise<ActionResult<Segment>> {
    if (id) check(id);
    const res = await runAction(
        async () =>
            (
                await api<Segment>(id ? `/segments/${id}` : '/segments', {
                    method: id ? 'PATCH' : 'POST',
                    body: {
                        name: input.name.trim(),
                        description: input.description.trim(),
                        definition: input.definition,
                    },
                })
            ).data,
        (await getT())('contacts.toast.segmentSaved'),
    );
    if (res.ok) {
        revalidatePath('/contacts/segments');
        if (id) revalidatePath(`/contacts/segments/${id}`);
    }
    return res;
}

export async function deleteSegmentAction(id: string): Promise<ActionResult> {
    check(id);
    const res = await runAction(
        () =>
            api(`/segments/${id}`, { method: 'DELETE' }).then(() => undefined),
        (await getT())('contacts.toast.segmentDeleted'),
    );
    if (res.ok) revalidatePath('/contacts/segments');
    return res;
}

// ── Imports ────────────────────────────────────────────────────────────────

export async function getImportAction(
    id: string,
): Promise<ActionResult<ContactImport>> {
    check(id);
    return runAction(
        async () => (await api<ContactImport>(`/contact-imports/${id}`)).data,
    );
}

export async function startImportAction(
    id: string,
    options: {
        mapping: Record<string, string>;
        duplicate_policy: 'skip' | 'update';
        add_tags: string[];
        group_id?: string;
    },
): Promise<ActionResult> {
    check(id);
    if (options.group_id) check(options.group_id);
    // The API answers 202 Accepted; progress is polled separately.
    const res = await runAction(
        () =>
            api(`/contact-imports/${id}/start`, {
                method: 'POST',
                body: options,
            }).then(() => undefined),
        (await getT())('contacts.toast.importStarted'),
    );
    if (res.ok) revalidatePath('/contacts/import');
    return res;
}

export async function cancelImportAction(
    id: string,
): Promise<ActionResult<ContactImport>> {
    check(id);
    const res = await runAction(
        async () =>
            (
                await api<ContactImport>(`/contact-imports/${id}/cancel`, {
                    method: 'POST',
                })
            ).data,
        (await getT())('contacts.toast.importCancelled'),
    );
    if (res.ok) revalidatePath('/contacts/import');
    return res;
}

export type BulkBody = Schemas['ContactBulkRequest'];

/** Applies one action to the selected contacts (or all matching ones). */
export async function bulkContactsAction(
    body: BulkBody,
): Promise<ActionResult<Schemas['ContactBulkResult']>> {
    const res = await runAction(
        async () =>
            (
                await api<Schemas['ContactBulkResult']>('/contacts/bulk', {
                    method: 'POST',
                    body,
                })
            ).data,
    );
    revalidatePath('/contacts');
    return res;
}
