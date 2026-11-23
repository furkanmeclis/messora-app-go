'use client';

import Link from 'next/link';
import { useRouter } from 'next/navigation';
import {
    useActionState,
    useEffect,
    useMemo,
    useState,
    useTransition,
} from 'react';
import { ChevronDown, Download, Search, UserPlus, Users } from 'lucide-react';
import { FormStatus, TextField, fieldError } from '@/components/forms';
import { newIdempotencyKey, notify } from '@/components/notify';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
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
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import {
    Empty,
    EmptyDescription,
    EmptyHeader,
    EmptyMedia,
    EmptyTitle,
} from '@/components/ui/empty';
import { FieldGroup } from '@/components/ui/field';
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
import {
    Table,
    TableBody,
    TableCell,
    TableHead,
    TableHeader,
    TableRow,
} from '@/components/ui/table';
import { useFormatDateTime } from '@/components/display-preferences';
import type { ActionResult, Contact } from '@/lib/types';
import {
    createContactAction,
    groupMembersAction,
    loadContactsAction,
} from './actions';
import { contactName } from './shared';
import { TagChip } from './tag-chip';
import { useSubmit } from './use-submit';

type Option = { id: string; name: string };
const ANY = 'any';

// ── Filters ────────────────────────────────────────────────────────────────

type Filters = {
    q: string;
    tag_id: string;
    group_id: string;
    segment_id: string;
    opted_out: string;
};

function FilterSelect({
    id,
    label,
    value,
    options,
    onChange,
}: {
    id: string;
    label: string;
    value: string;
    options: Option[];
    onChange: (v: string) => void;
}) {
    return (
        <div className="grid gap-1.5">
            <Label htmlFor={id} className="text-xs text-muted-foreground">
                {label}
            </Label>
            <Select
                value={value || ANY}
                onValueChange={(v) => onChange(v === ANY ? '' : v)}
            >
                <SelectTrigger id={id} className="w-full min-w-36">
                    <SelectValue />
                </SelectTrigger>
                <SelectContent>
                    <SelectItem value={ANY}>Any</SelectItem>
                    {options.map((o) => (
                        <SelectItem key={o.id} value={o.id}>
                            {o.name}
                        </SelectItem>
                    ))}
                </SelectContent>
            </Select>
        </div>
    );
}

export function ContactFilters({
    filters,
    tags,
    groups,
    segments,
}: {
    filters: Filters;
    tags: Option[];
    groups: Option[];
    segments: Option[];
}) {
    const router = useRouter();
    const [q, setQ] = useState(filters.q);
    const go = (patch: Partial<Filters>) => {
        const next = { ...filters, q, ...patch };
        const sp = new URLSearchParams();
        for (const [k, v] of Object.entries(next)) if (v) sp.set(k, v);
        const s = sp.toString();
        router.push(s ? `/contacts?${s}` : '/contacts');
    };
    const active =
        filters.q ||
        filters.tag_id ||
        filters.group_id ||
        filters.segment_id ||
        filters.opted_out;
    return (
        <div className="flex flex-wrap items-end gap-3">
            <form
                role="search"
                className="grid min-w-56 flex-1 gap-1.5"
                onSubmit={(e) => {
                    e.preventDefault();
                    go({ q: q.trim() });
                }}
            >
                <Label
                    htmlFor="contact-search"
                    className="text-xs text-muted-foreground"
                >
                    Search
                </Label>
                <div className="relative">
                    <Search className="pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2 text-muted-foreground" />
                    <Input
                        id="contact-search"
                        type="search"
                        className="pl-8"
                        placeholder="Name, phone or e-mail"
                        maxLength={100}
                        value={q}
                        onChange={(e) => setQ(e.target.value)}
                    />
                </div>
            </form>
            <FilterSelect
                id="filter-tag"
                label="Tag"
                value={filters.tag_id}
                options={tags}
                onChange={(tag_id) => go({ tag_id })}
            />
            <FilterSelect
                id="filter-group"
                label="Group"
                value={filters.group_id}
                options={groups}
                onChange={(group_id) => go({ group_id })}
            />
            {segments.length > 0 && (
                <FilterSelect
                    id="filter-segment"
                    label="Segment"
                    value={filters.segment_id}
                    options={segments}
                    onChange={(segment_id) => go({ segment_id })}
                />
            )}
            <FilterSelect
                id="filter-consent"
                label="Consent"
                value={filters.opted_out}
                options={[
                    { id: 'false', name: 'Subscribed' },
                    { id: 'true', name: 'Opted out' },
                ]}
                onChange={(opted_out) => go({ opted_out })}
            />
            {active && (
                <Button
                    variant="ghost"
                    onClick={() => {
                        setQ('');
                        router.push('/contacts');
                    }}
                >
                    Clear filters
                </Button>
            )}
        </div>
    );
}

// ── Export ─────────────────────────────────────────────────────────────────

export function ExportMenu({ query }: { query: string }) {
    const href = (format: 'csv' | 'xlsx') => {
        const sp = new URLSearchParams(query);
        // The export has no consent filter; it always includes everyone
        // matching the other filters.
        sp.delete('opted_out');
        sp.set('format', format);
        return `/bff/contacts/export?${sp}`;
    };
    return (
        <DropdownMenu>
            <DropdownMenuTrigger asChild>
                <Button variant="outline">
                    <Download /> Export <ChevronDown />
                </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
                <DropdownMenuItem asChild>
                    <a href={href('csv')} download>
                        CSV (.csv)
                    </a>
                </DropdownMenuItem>
                <DropdownMenuItem asChild>
                    <a href={href('xlsx')} download>
                        Excel (.xlsx)
                    </a>
                </DropdownMenuItem>
            </DropdownMenuContent>
        </DropdownMenu>
    );
}

// ── New contact ────────────────────────────────────────────────────────────

export function NewContactDialog() {
    const router = useRouter();
    const [open, setOpen] = useState(false);
    const [key, setKey] = useState(newIdempotencyKey);
    const [state, action, pending] = useActionState<
        ActionResult<Contact> | null,
        FormData
    >(createContactAction, null);

    // A key belongs to one request body: the API rejects a changed body
    // under a used key, so a corrected submission gets a fresh one.
    const submit = useSubmit((form) => {
        let k = key;
        if (state && !state.ok) {
            k = newIdempotencyKey();
            setKey(k);
        }
        form.set('idempotency_key', k);
        action(form);
    });
    useEffect(() => {
        if (state?.ok) router.push(`/contacts/${state.data.id}`);
    }, [state, router]);

    return (
        <Dialog
            open={open}
            onOpenChange={(o) => {
                setOpen(o);
                if (!o) setKey(newIdempotencyKey());
            }}
        >
            <DialogTrigger asChild>
                <Button>
                    <UserPlus /> New contact
                </Button>
            </DialogTrigger>
            <DialogContent>
                <DialogHeader>
                    <DialogTitle>New contact</DialogTitle>
                    <DialogDescription>
                        Tags, groups and custom fields can be added on the next
                        page.
                    </DialogDescription>
                </DialogHeader>
                <form onSubmit={submit} className="space-y-4">
                    <input type="hidden" name="idempotency_key" value={key} />
                    <FormStatus state={state?.ok ? null : state} />
                    <FieldGroup>
                        <TextField
                            label="Phone number"
                            name="phone"
                            type="tel"
                            required
                            autoFocus
                            placeholder="+905551112233"
                            hint="International format with the country code."
                            minLength={7}
                            maxLength={24}
                            error={fieldError(state, 'phone')}
                        />
                        <TextField
                            label="Name"
                            name="name"
                            maxLength={200}
                            error={fieldError(state, 'name')}
                        />
                        <TextField
                            label="E-mail"
                            name="email"
                            type="email"
                            maxLength={254}
                            error={fieldError(state, 'email')}
                        />
                    </FieldGroup>
                    <Button type="submit" className="w-full" disabled={pending}>
                        {pending && <Spinner />}Add contact
                    </Button>
                </form>
            </DialogContent>
        </Dialog>
    );
}

// ── Table ──────────────────────────────────────────────────────────────────

export function ContactTable({
    initial,
    initialCursor,
    query,
    filtered,
    groups,
    canUpdate,
    canCreate,
}: {
    initial: Contact[];
    initialCursor?: string;
    query: string;
    filtered: boolean;
    groups: Option[];
    canUpdate: boolean;
    canCreate: boolean;
}) {
    const formatDateTime = useFormatDateTime();
    const [contacts, setContacts] = useState(initial);
    const [cursor, setCursor] = useState(initialCursor);
    const [selected, setSelected] = useState<Set<string>>(new Set());
    const [group, setGroup] = useState('');
    const [loading, startLoading] = useTransition();
    const [saving, startSaving] = useTransition();
    const selectable = canUpdate && groups.length > 0;

    const allSelected =
        contacts.length > 0 && contacts.every((c) => selected.has(c.id));
    const someSelected = !allSelected && selected.size > 0;
    const ids = useMemo(() => [...selected], [selected]);

    const more = () =>
        startLoading(async () => {
            if (!cursor) return;
            const r = await loadContactsAction(query, cursor);
            if (notify(r)) {
                setContacts((c) => [...c, ...r.data.contacts]);
                setCursor(r.data.next);
            }
        });

    const toggle = (id: string, on: boolean) =>
        setSelected((s) => {
            const n = new Set(s);
            if (on) n.add(id);
            else n.delete(id);
            return n;
        });

    const addToGroup = () =>
        startSaving(async () => {
            if (!group || ids.length === 0) return;
            const name = groups.find((g) => g.id === group)?.name ?? 'group';
            const r = await groupMembersAction(group, 'add', ids);
            if (notify(r)) {
                const n = ids.length;
                const added = r.data;
                toastAdded(name, n, added);
                setSelected(new Set());
            }
        });

    if (contacts.length === 0) {
        return (
            <Empty className="py-12">
                <EmptyHeader>
                    <EmptyMedia variant="icon">
                        <Users />
                    </EmptyMedia>
                    <EmptyTitle>
                        {filtered ? 'No matching contacts' : 'No contacts yet'}
                    </EmptyTitle>
                    <EmptyDescription>
                        {filtered
                            ? 'Try another search or clear the filters.'
                            : canCreate
                              ? 'Add a contact or import a CSV or Excel file. Customers who write to you are added automatically.'
                              : 'Customers who write to you appear here automatically.'}
                    </EmptyDescription>
                </EmptyHeader>
            </Empty>
        );
    }

    return (
        <div>
            {selectable && selected.size > 0 && (
                <div
                    role="region"
                    aria-label="Bulk actions"
                    className="flex flex-wrap items-center gap-2 border-b bg-muted/40 p-3 text-sm"
                >
                    <span className="font-medium">
                        {selected.size} selected
                    </span>
                    <Select value={group} onValueChange={setGroup}>
                        <SelectTrigger
                            className="w-48"
                            aria-label="Group to add the selected contacts to"
                        >
                            <SelectValue placeholder="Choose a group" />
                        </SelectTrigger>
                        <SelectContent>
                            {groups.map((g) => (
                                <SelectItem key={g.id} value={g.id}>
                                    {g.name}
                                </SelectItem>
                            ))}
                        </SelectContent>
                    </Select>
                    <Button
                        size="sm"
                        disabled={!group || saving}
                        onClick={addToGroup}
                    >
                        {saving && <Spinner />} Add to group
                    </Button>
                    <Button
                        size="sm"
                        variant="ghost"
                        onClick={() => setSelected(new Set())}
                    >
                        Clear selection
                    </Button>
                </div>
            )}
            <Table>
                <TableHeader>
                    <TableRow>
                        {selectable && (
                            <TableHead className="w-10">
                                <Checkbox
                                    aria-label="Select all shown contacts"
                                    checked={
                                        allSelected
                                            ? true
                                            : someSelected
                                              ? 'indeterminate'
                                              : false
                                    }
                                    onCheckedChange={(v) =>
                                        setSelected(
                                            v === true
                                                ? new Set(
                                                      contacts.map((c) => c.id),
                                                  )
                                                : new Set(),
                                        )
                                    }
                                />
                            </TableHead>
                        )}
                        <TableHead>Name</TableHead>
                        <TableHead>Phone</TableHead>
                        <TableHead>Tags</TableHead>
                        <TableHead>Consent</TableHead>
                        <TableHead>Added</TableHead>
                    </TableRow>
                </TableHeader>
                <TableBody>
                    {contacts.map((c) => (
                        <TableRow
                            key={c.id}
                            data-state={
                                selected.has(c.id) ? 'selected' : undefined
                            }
                            data-testid="contact-row"
                        >
                            {selectable && (
                                <TableCell>
                                    <Checkbox
                                        aria-label={`Select ${contactName(c)}`}
                                        checked={selected.has(c.id)}
                                        onCheckedChange={(v) =>
                                            toggle(c.id, v === true)
                                        }
                                    />
                                </TableCell>
                            )}
                            <TableCell>
                                <Link
                                    href={`/contacts/${c.id}`}
                                    className="font-medium hover:underline"
                                >
                                    {contactName(c)}
                                </Link>
                                {c.email && (
                                    <div className="text-xs text-muted-foreground">
                                        {c.email}
                                    </div>
                                )}
                            </TableCell>
                            <TableCell className="font-mono text-sm">
                                {c.phone}
                            </TableCell>
                            <TableCell>
                                <div className="flex max-w-xs flex-wrap gap-1">
                                    {c.tags.map((t) => (
                                        <TagChip
                                            key={t.id}
                                            name={t.name}
                                            color={t.color}
                                        />
                                    ))}
                                </div>
                            </TableCell>
                            <TableCell>
                                {c.opted_out ? (
                                    <Badge variant="destructive">
                                        Opted out
                                    </Badge>
                                ) : (
                                    <Badge variant="secondary">
                                        Subscribed
                                    </Badge>
                                )}
                            </TableCell>
                            <TableCell className="text-sm whitespace-nowrap text-muted-foreground">
                                {formatDateTime(c.created_at)}
                            </TableCell>
                        </TableRow>
                    ))}
                </TableBody>
            </Table>
            {cursor && (
                <div className="border-t p-3 text-center">
                    <Button variant="outline" onClick={more} disabled={loading}>
                        {loading && <Spinner />} Load more
                    </Button>
                </div>
            )}
        </div>
    );
}

function toastAdded(group: string, selected: number, added: number) {
    const already = selected - added;
    notify({
        ok: true,
        data: undefined,
        message:
            `Added ${added} contact${added === 1 ? '' : 's'} to ${group}.` +
            (already > 0 ? ` ${already} were already members.` : ''),
    });
}
