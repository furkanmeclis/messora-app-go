'use client';

import { useRouter } from 'next/navigation';
import { useState, useTransition } from 'react';
import { Hand, Plus, SlidersHorizontal, Trash2 } from 'lucide-react';
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
import { Alert, AlertDescription } from '@/components/ui/alert';
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
    Dialog,
    DialogContent,
    DialogDescription,
    DialogHeader,
    DialogTitle,
    DialogTrigger,
} from '@/components/ui/dialog';
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
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from '@/components/ui/select';
import { Spinner } from '@/components/ui/spinner';
import { Textarea } from '@/components/ui/textarea';
import { useFormatDateTime } from '@/components/display-preferences';
import { useT } from '@/i18n/client';
import {
    CONSENT_CATEGORIES,
    CONSENT_STATUSES,
    type ConsentCategory,
    type ConsentStatus,
} from '@/lib/consent';
import type { Contact, Note, Schemas } from '@/lib/types';
import {
    addContactNoteAction,
    consentAction,
    deleteContactAction,
    deleteContactNoteAction,
    updateContactAction,
} from '../actions';
import { ConsentBadge, ReachabilityBadges } from '../consent-badges';
import { TagsInput } from '../tags-input';

type ConsentEvent = Schemas['ConsentEvent'];

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
    const t = useT();
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
            if (!k) local.attributes = t('contacts.detail.errFieldName');
            else if (seen.has(k))
                local.attributes = t('contacts.detail.errFieldTwice', {
                    name: k,
                });
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
        <form
            onSubmit={save}
            noValidate
            aria-label={t('contacts.detail.formLabel')}
        >
            <Card>
                <CardHeader>
                    <CardTitle>{t('contacts.detail.details')}</CardTitle>
                    <CardDescription>
                        {t('contacts.detail.detailsHint')}
                    </CardDescription>
                </CardHeader>
                <CardContent className="space-y-8">
                    <FieldGroup className="grid gap-4 sm:grid-cols-2">
                        <TextField
                            label={t('contacts.fields.name')}
                            name="name"
                            value={name}
                            maxLength={200}
                            readOnly={ro}
                            onChange={(e) => setName(e.target.value)}
                            error={errors.name}
                            hint={
                                contact.profile_name
                                    ? t('contacts.detail.profile', {
                                          name: contact.profile_name,
                                      })
                                    : undefined
                            }
                        />
                        <TextField
                            label={t('contacts.fields.email')}
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
                                label={t('contacts.detail.variable', {
                                    n: i + 1,
                                })}
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
                        <FieldLabel htmlFor="contact-tags">
                            {t('contacts.fields.tags')}
                        </FieldLabel>
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
                                {t('contacts.detail.newTags')}
                            </FieldDescription>
                        )}
                        <ErrorText id="contact-tags-error" text={errors.tags} />
                    </Field>

                    <FieldSet>
                        <FieldLegend variant="label">
                            {t('contacts.detail.groups')}
                        </FieldLegend>
                        {groups.length === 0 ? (
                            <FieldDescription>
                                {t('contacts.detail.noGroups')}
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
                        <FieldLegend variant="label">
                            {t('contacts.detail.customFields')}
                        </FieldLegend>
                        <FieldDescription>
                            {t('contacts.detail.customFieldsHint')}
                        </FieldDescription>
                        {attrs.length > 0 && (
                            <ul className="space-y-2">
                                {attrs.map((a, i) => (
                                    <li
                                        key={a.id}
                                        className="flex items-center gap-2"
                                    >
                                        <Input
                                            aria-label={t(
                                                'contacts.detail.fieldName',
                                                { n: i + 1 },
                                            )}
                                            placeholder={t(
                                                'contacts.fields.name',
                                            )}
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
                                            aria-label={t(
                                                'contacts.detail.fieldValue',
                                                { n: i + 1 },
                                            )}
                                            placeholder={t(
                                                'contacts.detail.value',
                                            )}
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
                                                aria-label={t(
                                                    'contacts.detail.removeField',
                                                    { name: a.key || i + 1 },
                                                )}
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
                                <Plus /> {t('contacts.detail.addField')}
                            </Button>
                        )}
                        {ro && attrs.length === 0 && (
                            <p className="text-sm text-muted-foreground">
                                {t('contacts.detail.none')}
                            </p>
                        )}
                        <ErrorText id="attrs-error" text={errors.attributes} />
                    </FieldSet>
                </CardContent>
                {canUpdate && (
                    <CardFooter className="justify-end">
                        <Button type="submit" disabled={pending}>
                            {pending && <Spinner />} {t('contacts.detail.save')}
                        </Button>
                    </CardFooter>
                )}
            </Card>
        </form>
    );
}

// ── Facts ──────────────────────────────────────────────────────────────────

export function ContactFacts({ contact }: { contact: Contact }) {
    const t = useT();
    const formatDateTime = useFormatDateTime();
    return (
        <Card>
            <CardHeader>
                <CardTitle>{t('contacts.detail.about')}</CardTitle>
            </CardHeader>
            <CardContent>
                <dl className="grid gap-2 text-sm">
                    <div>
                        <dt className="text-muted-foreground">
                            {t('contacts.detail.source')}
                        </dt>
                        <dd>
                            {t(`contacts.detail.sources.${contact.source}`)}
                        </dd>
                    </div>
                    <div>
                        <dt className="text-muted-foreground">
                            {t('contacts.table.added')}
                        </dt>
                        <dd>{formatDateTime(contact.created_at)}</dd>
                    </div>
                    <div>
                        <dt className="text-muted-foreground">
                            {t('contacts.detail.lastInbound')}
                        </dt>
                        <dd>{formatDateTime(contact.last_inbound_at)}</dd>
                    </div>
                </dl>
            </CardContent>
        </Card>
    );
}

// ── Consent ────────────────────────────────────────────────────────────────

type ConsentTarget = ConsentCategory | 'all';

export function ConsentCard({
    contact,
    canUpdate,
}: {
    contact: Contact;
    canUpdate: boolean;
}) {
    const t = useT();
    const formatDateTime = useFormatDateTime();
    const router = useRouter();
    const [pending, startTransition] = useTransition();
    const c = contact.consent;
    const requested = CONSENT_CATEGORIES.filter(
        (k) => c[k].status === 'opt_out_requested',
    );
    const target: ConsentTarget =
        requested.length === 2 ? 'all' : (requested[0] ?? 'all');

    const quick = (status: ConsentStatus) =>
        startTransition(async () => {
            if (notify(await consentAction(contact.id, status, target, '')))
                router.refresh();
        });

    return (
        <Card>
            <CardHeader>
                <CardTitle>{t('consent.card.title')}</CardTitle>
                <CardDescription>
                    {t('consent.card.description')}
                </CardDescription>
            </CardHeader>
            <CardContent className="space-y-4 text-sm">
                <ul className="space-y-3" data-testid="consent-state">
                    {CONSENT_CATEGORIES.map((k) => {
                        const st = c[k];
                        return (
                            <li key={k} className="space-y-1">
                                <ConsentBadge category={k} status={st.status} />
                                {(st.updated_at || st.reason) && (
                                    <p className="text-xs text-muted-foreground">
                                        {st.updated_at &&
                                            t('consent.card.updated', {
                                                time: formatDateTime(
                                                    st.updated_at,
                                                ),
                                            })}
                                        {st.source &&
                                            isSource(st.source) &&
                                            ` · ${t(`consent.sources.${st.source}`)}`}
                                        {st.reason && (
                                            <span className="block break-words">
                                                {t('consent.card.reason', {
                                                    reason: st.reason,
                                                })}
                                            </span>
                                        )}
                                    </p>
                                )}
                            </li>
                        );
                    })}
                </ul>
                {(c.meta_marketing_stopped_at || c.undeliverable_at) && (
                    <div className="flex flex-wrap gap-1.5">
                        <ReachabilityBadges consent={c} withHelp />
                    </div>
                )}
                {requested.length > 0 && (
                    <Alert data-testid="consent-request">
                        <Hand />
                        <AlertDescription className="space-y-2">
                            <p>{t('consent.card.requestPending')}</p>
                            {canUpdate && (
                                <div className="flex flex-wrap gap-2">
                                    <Button
                                        size="sm"
                                        variant="destructive"
                                        disabled={pending}
                                        onClick={() => quick('opted_out')}
                                    >
                                        {t('consent.card.confirmOptOut')}
                                    </Button>
                                    <Button
                                        size="sm"
                                        variant="outline"
                                        disabled={pending}
                                        onClick={() => quick('opted_in')}
                                    >
                                        {t('consent.card.clearRequest')}
                                    </Button>
                                </div>
                            )}
                        </AlertDescription>
                    </Alert>
                )}
                {canUpdate && <ConsentDialog contact={contact} />}
            </CardContent>
        </Card>
    );
}

const SOURCES = [
    'manual',
    'bulk',
    'api',
    'import',
    'keyword',
    'meta_user_preferences',
    'meta_error',
    'automation',
    'inbound',
] as const;

function isSource(s: string): s is (typeof SOURCES)[number] {
    return (SOURCES as readonly string[]).includes(s);
}

function ConsentDialog({ contact }: { contact: Contact }) {
    const t = useT();
    const router = useRouter();
    const [open, setOpen] = useState(false);
    const [category, setCategory] = useState<ConsentTarget>('marketing');
    const current =
        category === 'all' ? undefined : contact.consent[category].status;
    const [status, setStatus] = useState<ConsentStatus>('opted_out');
    const [note, setNote] = useState('');
    const [error, setError] = useState<string | null>(null);
    const [pending, startTransition] = useTransition();

    const submit = (e: React.FormEvent) => {
        e.preventDefault();
        startTransition(async () => {
            const r = await consentAction(contact.id, status, category, note);
            if (!r.ok) {
                setError(
                    r.fieldErrors?.note ?? r.fieldErrors?.status ?? r.message,
                );
                return;
            }
            notify(r);
            setOpen(false);
            setNote('');
            setError(null);
            router.refresh();
        });
    };

    return (
        <Dialog
            open={open}
            onOpenChange={(o) => {
                setOpen(o);
                if (o) setError(null);
            }}
        >
            <DialogTrigger asChild>
                <Button size="sm" variant="outline">
                    <SlidersHorizontal /> {t('consent.card.change')}
                </Button>
            </DialogTrigger>
            <DialogContent>
                <DialogHeader>
                    <DialogTitle>{t('consent.dialog.title')}</DialogTitle>
                    <DialogDescription>
                        {t('consent.dialog.description')}
                    </DialogDescription>
                </DialogHeader>
                <form onSubmit={submit} className="space-y-4">
                    {error && (
                        <p role="alert" className="text-sm text-destructive">
                            {error}
                        </p>
                    )}
                    <FieldGroup>
                        <Field>
                            <FieldLabel htmlFor="consent-category">
                                {t('consent.dialog.category')}
                            </FieldLabel>
                            <Select
                                value={category}
                                onValueChange={(v) =>
                                    setCategory(v as ConsentTarget)
                                }
                            >
                                <SelectTrigger
                                    id="consent-category"
                                    className="w-full"
                                >
                                    <SelectValue />
                                </SelectTrigger>
                                <SelectContent>
                                    {(
                                        ['marketing', 'service', 'all'] as const
                                    ).map((k) => (
                                        <SelectItem key={k} value={k}>
                                            {t(`consent.categories.${k}`)}
                                        </SelectItem>
                                    ))}
                                </SelectContent>
                            </Select>
                            <FieldDescription>
                                {t(`consent.categoryHints.${category}`)}
                            </FieldDescription>
                        </Field>
                        <Field>
                            <FieldLabel htmlFor="consent-status">
                                {t('consent.dialog.status')}
                            </FieldLabel>
                            <Select
                                value={status}
                                onValueChange={(v) =>
                                    setStatus(v as ConsentStatus)
                                }
                            >
                                <SelectTrigger
                                    id="consent-status"
                                    className="w-full"
                                >
                                    <SelectValue />
                                </SelectTrigger>
                                <SelectContent>
                                    {CONSENT_STATUSES.map((s) => (
                                        <SelectItem
                                            key={s}
                                            value={s}
                                            disabled={s === current}
                                        >
                                            {t(`consent.statuses.${s}`)}
                                        </SelectItem>
                                    ))}
                                </SelectContent>
                            </Select>
                        </Field>
                        <Field>
                            <FieldLabel htmlFor="consent-note">
                                {t('consent.dialog.reason')}{' '}
                                <span className="font-normal text-muted-foreground">
                                    {t('contacts.detail.optional')}
                                </span>
                            </FieldLabel>
                            <Textarea
                                id="consent-note"
                                rows={2}
                                maxLength={500}
                                value={note}
                                placeholder={t(
                                    'consent.dialog.reasonPlaceholder',
                                )}
                                onChange={(e) => setNote(e.target.value)}
                            />
                        </Field>
                    </FieldGroup>
                    {(status === 'opted_in' || status === 'resubscribed') &&
                        (contact.consent.meta_marketing_stopped_at ||
                            contact.consent.undeliverable_at) && (
                            <p className="text-xs text-muted-foreground">
                                {t('consent.dialog.hardNote')}
                            </p>
                        )}
                    <Button
                        type="submit"
                        className="w-full"
                        disabled={pending || status === current}
                    >
                        {pending && <Spinner />}
                        {t('consent.dialog.save')}
                    </Button>
                </form>
            </DialogContent>
        </Dialog>
    );
}

/** The append-only consent history (GET /contacts/{id}/consent-events). */
export function ConsentHistoryCard({
    events,
}: {
    events: ConsentEvent[] | null;
}) {
    const t = useT();
    const formatDateTime = useFormatDateTime();
    return (
        <Card>
            <CardHeader>
                <CardTitle>{t('consent.history.title')}</CardTitle>
            </CardHeader>
            <CardContent className="text-sm">
                {events === null ? (
                    <p className="text-muted-foreground">
                        {t('consent.history.loadFailed')}
                    </p>
                ) : events.length === 0 ? (
                    <p className="text-muted-foreground">
                        {t('consent.history.empty')}
                    </p>
                ) : (
                    <ol
                        className="space-y-3 border-l pl-3"
                        data-testid="consent-history"
                    >
                        {events.map((e) => (
                            <li
                                key={e.id}
                                data-testid="consent-event"
                                data-kind={e.kind}
                            >
                                <p className="font-medium">
                                    {t(`consent.events.${e.kind}`)}
                                    <span className="font-normal text-muted-foreground">
                                        {' · '}
                                        {e.category === 'marketing' ||
                                        e.category === 'service'
                                            ? t(
                                                  `consent.categories.${e.category}`,
                                              )
                                            : t('consent.history.both')}
                                    </span>
                                </p>
                                {e.note && (
                                    <p className="break-words whitespace-pre-wrap">
                                        {e.note}
                                    </p>
                                )}
                                <p className="text-xs text-muted-foreground">
                                    {e.actor_name
                                        ? t('consent.history.by', {
                                              name: e.actor_name,
                                          })
                                        : t('consent.history.automatic')}
                                    {' · '}
                                    {t('consent.history.via', {
                                        source: t(
                                            `consent.sources.${e.source}`,
                                        ),
                                    })}
                                    {' · '}
                                    {formatDateTime(e.created_at)}
                                </p>
                            </li>
                        ))}
                    </ol>
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
    const t = useT();
    const formatDateTime = useFormatDateTime();
    const router = useRouter();
    const [body, setBody] = useState('');
    const [pending, startTransition] = useTransition();
    return (
        <Card>
            <CardHeader>
                <CardTitle>{t('contacts.detail.notes')}</CardTitle>
                <CardDescription>
                    {t('contacts.detail.notesHint')}
                </CardDescription>
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
                            {t('contacts.detail.newNote')}
                        </Label>
                        <Textarea
                            id="new-note"
                            rows={3}
                            maxLength={5000}
                            value={body}
                            placeholder={t('contacts.detail.notePlaceholder')}
                            onChange={(e) => setBody(e.target.value)}
                        />
                        <Button
                            type="submit"
                            size="sm"
                            disabled={pending || !body.trim()}
                        >
                            {pending && <Spinner />}{' '}
                            {t('contacts.detail.addNote')}
                        </Button>
                    </form>
                )}
                {notes.length === 0 ? (
                    <p className="text-muted-foreground">
                        {t('contacts.detail.noNotes')}
                    </p>
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
                                        {n.author_name ||
                                            t('contacts.detail.someone')}{' '}
                                        · {formatDateTime(n.created_at)}
                                    </p>
                                </div>
                                {canUpdate && (
                                    <Button
                                        variant="ghost"
                                        size="icon-sm"
                                        aria-label={t(
                                            'contacts.detail.deleteNote',
                                        )}
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
    const t = useT();
    const router = useRouter();
    const [pending, startTransition] = useTransition();
    return (
        <AlertDialog>
            <AlertDialogTrigger asChild>
                <Button variant="destructive" disabled={pending}>
                    <Trash2 /> {t('common.delete')}
                </Button>
            </AlertDialogTrigger>
            <AlertDialogContent>
                <AlertDialogHeader>
                    <AlertDialogTitle>
                        {t('contacts.detail.deleteTitle', { name })}
                    </AlertDialogTitle>
                    <AlertDialogDescription>
                        {t('contacts.detail.deleteBody')}
                    </AlertDialogDescription>
                </AlertDialogHeader>
                <AlertDialogFooter>
                    <AlertDialogCancel>{t('common.cancel')}</AlertDialogCancel>
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
                        {t('contacts.detail.deleteConfirm')}
                    </AlertDialogAction>
                </AlertDialogFooter>
            </AlertDialogContent>
        </AlertDialog>
    );
}
