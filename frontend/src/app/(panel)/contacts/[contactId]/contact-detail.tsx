'use client';

import { useRouter } from 'next/navigation';
import { useState, useTransition } from 'react';
import { Plus, Trash2 } from 'lucide-react';
import { TextField } from '@/components/forms';
import { notify } from '@/components/notify';
import {
    AlertDialog,
    AlertDialogAction,
    AlertDialogCancel,
    AlertDialogContent,
    AlertDialogDescription,
    AlertDialogFooter,
    AlertDialogHeader,
    AlertDialogTitle,
    AlertDialogTrigger,
} from '@/components/ui/alert-dialog';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
    Card,
    CardContent,
    CardDescription,
    CardFooter,
    CardHeader,
    CardTitle,
} from '@/components/ui/card';
import { Checkbox } from '@/components/ui/checkbox';
import {
    Field,
    FieldDescription,
    FieldGroup,
    FieldLabel,
    FieldLegend,
    FieldSet,
} from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Spinner } from '@/components/ui/spinner';
import { Textarea } from '@/components/ui/textarea';
import { formatDateTime } from '@/lib/format';
import type { Contact, Note } from '@/lib/types';
import {
    addContactNoteAction,
    consentAction,
    deleteContactAction,
    deleteContactNoteAction,
    updateContactAction,
} from '../actions';
import { TagsInput } from '../tags-input';

type TagOption = { id: string; name: string; color: string };
type Option = { id: string; name: string };
type Attr = { key: string; value: string; id: number };

const VARS = ['var1', 'var2', 'var3', 'var4', 'var5'] as const;

function ErrorText({ id, text }: { id: string; text?: string }) {
    if (!text) return null;
    return (
        <p id={id} role="alert" className="text-sm text-destructive">
            {text}
        </p>
    );
}

// ── Edit form ──────────────────────────────────────────────────────────────

export function ContactForm({
    contact,
    tags,
    groups,
    canUpdate,
}: {
    contact: Contact;
    tags: TagOption[];
    groups: Option[];
    canUpdate: boolean;
}) {
    const router = useRouter();
    const [name, setName] = useState(contact.name);
    const [email, setEmail] = useState(contact.email ?? '');
    const [vars, setVars] = useState<string[]>(
        VARS.map((k) => contact[k] ?? ''),
    );
    const [attrs, setAttrs] = useState<Attr[]>(() =>
        Object.entries(contact.attributes).map(([key, value], id) => ({
            key,
            value,
            id,
        })),
    );
    const [tagNames, setTagNames] = useState(contact.tags.map((t) => t.name));
    const [groupIds, setGroupIds] = useState(contact.group_ids);
    const [errors, setErrors] = useState<Record<string, string>>({});
    const [pending, startTransition] = useTransition();
    const ro = !canUpdate;

    const save = (e: React.FormEvent) => {
        e.preventDefault();
        const local: Record<string, string> = {};
        const seen = new Set<string>();
        const attributes: Record<string, string> = {};
        for (const a of attrs) {
            const k = a.key.trim();
            if (!k && !a.value.trim()) continue;
            if (!k) local.attributes = 'Every custom field needs a name.';
            else if (seen.has(k))
                local.attributes = `The field name “${k}” is used twice.`;
            seen.add(k);
            attributes[k] = a.value;
        }
        if (Object.keys(local).length > 0) {
            setErrors(local);
            return;
        }
        const [var1 = '', var2 = '', var3 = '', var4 = '', var5 = ''] = vars;
        startTransition(async () => {
            const r = await updateContactAction(contact.id, {
                name,
                email: email.trim(),
                var1,
                var2,
                var3,
                var4,
                var5,
                attributes,
                tags: tagNames,
                group_ids: groupIds,
            });
            setErrors(r.ok ? {} : (r.fieldErrors ?? {}));
            if (notify(r)) router.refresh();
        });
    };

    return (
        <form onSubmit={save} noValidate aria-label="Contact details">
            <Card>
                <CardHeader>
                    <CardTitle>Details</CardTitle>
                    <CardDescription>
                        Variables 1–5 fill template placeholders in campaigns.
                    </CardDescription>
                </CardHeader>
                <CardContent className="space-y-8">
                    <FieldGroup className="grid gap-4 sm:grid-cols-2">
                        <TextField
                            label="Name"
                            name="name"
                            value={name}
                            maxLength={200}
                            readOnly={ro}
                            onChange={(e) => setName(e.target.value)}
                            error={errors.name}
                            hint={
                                contact.profile_name
                                    ? `WhatsApp profile: ${contact.profile_name}`
                                    : undefined
                            }
                        />
                        <TextField
                            label="E-mail"
                            name="email"
                            type="email"
                            value={email}
                            maxLength={254}
                            readOnly={ro}
                            onChange={(e) => setEmail(e.target.value)}
                            error={errors.email}
                        />
                        {VARS.map((k, i) => (
                            <TextField
                                key={k}
                                label={`Variable ${i + 1}`}
                                name={k}
                                value={vars[i]}
                                maxLength={500}
                                readOnly={ro}
                                onChange={(e) =>
                                    setVars((v) =>
                                        v.map((x, j) =>
                                            j === i ? e.target.value : x,
                                        ),
                                    )
                                }
                                error={errors[k]}
                            />
                        ))}
                    </FieldGroup>

                    <Field data-invalid={errors.tags ? true : undefined}>
                        <FieldLabel htmlFor="contact-tags">Tags</FieldLabel>
                        <TagsInput
                            id="contact-tags"
                            value={tagNames}
                            onChange={setTagNames}
                            suggestions={tags}
                            disabled={ro}
                            invalid={!!errors.tags}
                            describedBy={
                                errors.tags ? 'contact-tags-error' : undefined
                            }
                        />
                        {!ro && tagNames.length === 0 && (
                            <FieldDescription>
                                New names create new tags.
                            </FieldDescription>
                        )}
                        <ErrorText id="contact-tags-error" text={errors.tags} />
                    </Field>

                    <FieldSet>
                        <FieldLegend variant="label">Groups</FieldLegend>
                        {groups.length === 0 ? (
                            <FieldDescription>
                                No groups yet. Create them under Groups.
                            </FieldDescription>
                        ) : (
                            <ul className="grid max-h-48 gap-2 overflow-y-auto rounded-md border p-3 sm:grid-cols-2">
                                {groups.map((g) => (
                                    <li
                                        key={g.id}
                                        className="flex items-center gap-2"
                                    >
                                        <Checkbox
                                            id={`group-${g.id}`}
                                            checked={groupIds.includes(g.id)}
                                            disabled={ro}
                                            onCheckedChange={(v) =>
                                                setGroupIds((ids) =>
                                                    v === true
                                                        ? [...ids, g.id]
                                                        : ids.filter(
                                                              (x) => x !== g.id,
                                                          ),
                                                )
                                            }
                                        />
                                        <Label
                                            htmlFor={`group-${g.id}`}
                                            className="font-normal"
                                        >
                                            {g.name}
                                        </Label>
                                    </li>
                                ))}
                            </ul>
                        )}
                        <ErrorText id="groups-error" text={errors.group_ids} />
                    </FieldSet>

                    <FieldSet>
                        <FieldLegend variant="label">Custom fields</FieldLegend>
                        <FieldDescription>
                            Extra information as name and value, up to 50.
                        </FieldDescription>
                        {attrs.length > 0 && (
                            <ul className="space-y-2">
                                {attrs.map((a, i) => (
                                    <li
                                        key={a.id}
                                        className="flex items-center gap-2"
                                    >
                                        <Input
                                            aria-label={`Custom field ${i + 1} name`}
                                            placeholder="Name"
                                            className="w-40"
                                            value={a.key}
                                            maxLength={60}
                                            readOnly={ro}
                                            onChange={(e) =>
                                                setAttrs((l) =>
                                                    l.map((x) =>
                                                        x.id === a.id
                                                            ? {
                                                                  ...x,
                                                                  key: e.target
                                                                      .value,
                                                              }
                                                            : x,
                                                    ),
                                                )
                                            }
                                        />
                                        <Input
                                            aria-label={`Custom field ${i + 1} value`}
                                            placeholder="Value"
                                            value={a.value}
                                            maxLength={1000}
                                            readOnly={ro}
                                            onChange={(e) =>
                                                setAttrs((l) =>
                                                    l.map((x) =>
                                                        x.id === a.id
                                                            ? {
                                                                  ...x,
                                                                  value: e
                                                                      .target
                                                                      .value,
                                                              }
                                                            : x,
                                                    ),
                                                )
                                            }
                                        />
                                        {!ro && (
                                            <Button
                                                type="button"
                                                variant="ghost"
                                                size="icon"
                                                aria-label={`Remove custom field ${a.key || i + 1}`}
                                                onClick={() =>
                                                    setAttrs((l) =>
                                                        l.filter(
                                                            (x) =>
                                                                x.id !== a.id,
                                                        ),
                                                    )
                                                }
                                            >
                                                <Trash2 />
                                            </Button>
                                        )}
                                    </li>
                                ))}
                            </ul>
                        )}
                        {!ro && attrs.length < 50 && (
                            <Button
                                type="button"
                                variant="outline"
                                size="sm"
                                className="w-fit"
                                onClick={() =>
                                    setAttrs((l) => [
                                        ...l,
                                        {
                                            key: '',
                                            value: '',
                                            id:
                                                Math.max(
                                                    -1,
                                                    ...l.map((x) => x.id),
                                                ) + 1,
                                        },
                                    ])
                                }
                            >
                                <Plus /> Add field
                            </Button>
                        )}
                        {ro && attrs.length === 0 && (
                            <p className="text-sm text-muted-foreground">
                                None.
                            </p>
                        )}
                        <ErrorText id="attrs-error" text={errors.attributes} />
                    </FieldSet>
                </CardContent>
                {canUpdate && (
                    <CardFooter className="justify-end">
                        <Button type="submit" disabled={pending}>
                            {pending && <Spinner />} Save changes
                        </Button>
                    </CardFooter>
                )}
            </Card>
        </form>
    );
}

// ── Facts ──────────────────────────────────────────────────────────────────

const SOURCE: Record<Contact['source'], string> = {
    webhook: 'Wrote to you on WhatsApp',
    manual: 'Added in the panel',
    import: 'Imported from a file',
    api: 'Added through the API',
};

export function ContactFacts({ contact }: { contact: Contact }) {
    return (
        <Card>
            <CardHeader>
                <CardTitle>About</CardTitle>
            </CardHeader>
            <CardContent>
                <dl className="grid gap-2 text-sm">
                    <div>
                        <dt className="text-muted-foreground">Source</dt>
                        <dd>{SOURCE[contact.source]}</dd>
                    </div>
                    <div>
                        <dt className="text-muted-foreground">Added</dt>
                        <dd>{formatDateTime(contact.created_at)}</dd>
                    </div>
                    <div>
                        <dt className="text-muted-foreground">
                            Last message from them
                        </dt>
                        <dd>{formatDateTime(contact.last_inbound_at)}</dd>
                    </div>
                </dl>
            </CardContent>
        </Card>
    );
}

// ── Consent ────────────────────────────────────────────────────────────────

export function ConsentCard({
    contact,
    canUpdate,
}: {
    contact: Contact;
    canUpdate: boolean;
}) {
    const router = useRouter();
    const [note, setNote] = useState('');
    const [pending, startTransition] = useTransition();
    const kind = contact.opted_out ? 'opt_in' : 'opt_out';
    return (
        <Card>
            <CardHeader>
                <CardTitle>Consent</CardTitle>
                <CardDescription>
                    Opted-out contacts receive no campaign or automation
                    messages.
                </CardDescription>
            </CardHeader>
            <CardContent className="space-y-3 text-sm">
                <p data-testid="consent-state">
                    {contact.opted_out ? (
                        <Badge variant="destructive">Opted out</Badge>
                    ) : (
                        <Badge variant="secondary">Subscribed</Badge>
                    )}
                    {contact.opted_out && contact.opted_out_at && (
                        <span className="ml-2 text-muted-foreground">
                            since {formatDateTime(contact.opted_out_at)}
                        </span>
                    )}
                </p>
                {canUpdate && (
                    <form
                        className="space-y-2"
                        onSubmit={(e) => {
                            e.preventDefault();
                            startTransition(async () => {
                                const r = await consentAction(
                                    contact.id,
                                    kind,
                                    note,
                                );
                                if (notify(r)) {
                                    setNote('');
                                    router.refresh();
                                }
                            });
                        }}
                    >
                        <Label htmlFor="consent-note">
                            Note{' '}
                            <span className="font-normal text-muted-foreground">
                                (optional)
                            </span>
                        </Label>
                        <Textarea
                            id="consent-note"
                            rows={2}
                            maxLength={500}
                            value={note}
                            placeholder={
                                contact.opted_out
                                    ? 'e.g. Asked to receive offers again'
                                    : 'e.g. Asked by phone to stop messages'
                            }
                            onChange={(e) => setNote(e.target.value)}
                        />
                        <Button
                            type="submit"
                            size="sm"
                            variant={contact.opted_out ? 'default' : 'outline'}
                            disabled={pending}
                        >
                            {pending && <Spinner />}
                            {contact.opted_out
                                ? 'Record opt-in'
                                : 'Record opt-out'}
                        </Button>
                    </form>
                )}
            </CardContent>
        </Card>
    );
}

// ── Notes ──────────────────────────────────────────────────────────────────

export function NotesCard({
    contactId,
    notes,
    canUpdate,
}: {
    contactId: string;
    notes: Note[];
    canUpdate: boolean;
}) {
    const router = useRouter();
    const [body, setBody] = useState('');
    const [pending, startTransition] = useTransition();
    return (
        <Card>
            <CardHeader>
                <CardTitle>Notes</CardTitle>
                <CardDescription>Only your team sees these.</CardDescription>
            </CardHeader>
            <CardContent className="space-y-4 text-sm">
                {canUpdate && (
                    <form
                        className="space-y-2"
                        onSubmit={(e) => {
                            e.preventDefault();
                            if (!body.trim()) return;
                            startTransition(async () => {
                                const r = await addContactNoteAction(
                                    contactId,
                                    body.trim(),
                                );
                                if (notify(r)) {
                                    setBody('');
                                    router.refresh();
                                }
                            });
                        }}
                    >
                        <Label htmlFor="new-note" className="sr-only">
                            New note
                        </Label>
                        <Textarea
                            id="new-note"
                            rows={3}
                            maxLength={5000}
                            value={body}
                            placeholder="Write a note"
                            onChange={(e) => setBody(e.target.value)}
                        />
                        <Button
                            type="submit"
                            size="sm"
                            disabled={pending || !body.trim()}
                        >
                            {pending && <Spinner />} Add note
                        </Button>
                    </form>
                )}
                {notes.length === 0 ? (
                    <p className="text-muted-foreground">No notes yet.</p>
                ) : (
                    <ul className="divide-y">
                        {notes.map((n) => (
                            <li
                                key={n.id}
                                className="flex gap-2 py-2 first:pt-0"
                            >
                                <div className="min-w-0 flex-1">
                                    <p className="break-words whitespace-pre-wrap">
                                        {n.body}
                                    </p>
                                    <p className="text-xs text-muted-foreground">
                                        {n.author_name || 'Someone'} ·{' '}
                                        {formatDateTime(n.created_at)}
                                    </p>
                                </div>
                                {canUpdate && (
                                    <Button
                                        variant="ghost"
                                        size="icon-sm"
                                        aria-label="Delete note"
                                        disabled={pending}
                                        onClick={() =>
                                            startTransition(async () => {
                                                if (
                                                    notify(
                                                        await deleteContactNoteAction(
                                                            contactId,
                                                            n.id,
                                                        ),
                                                    )
                                                ) {
                                                    router.refresh();
                                                }
                                            })
                                        }
                                    >
                                        <Trash2 />
                                    </Button>
                                )}
                            </li>
                        ))}
                    </ul>
                )}
            </CardContent>
        </Card>
    );
}

// ── Delete ─────────────────────────────────────────────────────────────────

export function DeleteContactButton({
    id,
    name,
}: {
    id: string;
    name: string;
}) {
    const router = useRouter();
    const [pending, startTransition] = useTransition();
    return (
        <AlertDialog>
            <AlertDialogTrigger asChild>
                <Button variant="destructive" disabled={pending}>
                    <Trash2 /> Delete
                </Button>
            </AlertDialogTrigger>
            <AlertDialogContent>
                <AlertDialogHeader>
                    <AlertDialogTitle>Delete {name}?</AlertDialogTitle>
                    <AlertDialogDescription>
                        The contact, its notes and its whole message history are
                        removed for good. This cannot be undone.
                    </AlertDialogDescription>
                </AlertDialogHeader>
                <AlertDialogFooter>
                    <AlertDialogCancel>Cancel</AlertDialogCancel>
                    <AlertDialogAction
                        variant="destructive"
                        onClick={() =>
                            startTransition(async () => {
                                if (notify(await deleteContactAction(id))) {
                                    router.push('/contacts');
                                }
                            })
                        }
                    >
                        Delete contact
                    </AlertDialogAction>
                </AlertDialogFooter>
            </AlertDialogContent>
        </AlertDialog>
    );
}
