'use client';

import { useRouter } from 'next/navigation';
import { useActionState, useEffect, useState } from 'react';
import { ChevronDown, Download, Search, UserPlus } from 'lucide-react';
import { FormStatus, TextField, fieldError } from '@/components/forms';
import { newIdempotencyKey } from '@/components/notify';
import { Button } from '@/components/ui/button';
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
import { useT } from '@/i18n/client';
import type { ActionResult, Contact } from '@/lib/types';
import { CONSENT_CATEGORIES, CONSENT_STATUSES } from '@/lib/consent';
import { createContactAction } from './actions';
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
    marketing_consent: string;
    service_consent: string;
    reachability: string;
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
    const t = useT();
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
                    <SelectItem value={ANY}>
                        {t('contacts.filters.any')}
                    </SelectItem>
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
    const t = useT();
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
        filters.opted_out ||
        filters.marketing_consent ||
        filters.service_consent ||
        filters.reachability;
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
                    {t('common.search')}
                </Label>
                <div className="relative">
                    <Search className="pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2 text-muted-foreground" />
                    <Input
                        id="contact-search"
                        type="search"
                        className="pl-8"
                        data-search
                        placeholder={t('contacts.filters.searchPlaceholder')}
                        maxLength={100}
                        value={q}
                        onChange={(e) => setQ(e.target.value)}
                    />
                </div>
            </form>
            <FilterSelect
                id="filter-tag"
                label={t('contacts.filters.tag')}
                value={filters.tag_id}
                options={tags}
                onChange={(tag_id) => go({ tag_id })}
            />
            <FilterSelect
                id="filter-group"
                label={t('contacts.filters.group')}
                value={filters.group_id}
                options={groups}
                onChange={(group_id) => go({ group_id })}
            />
            {segments.length > 0 && (
                <FilterSelect
                    id="filter-segment"
                    label={t('contacts.filters.segment')}
                    value={filters.segment_id}
                    options={segments}
                    onChange={(segment_id) => go({ segment_id })}
                />
            )}
            <FilterSelect
                id="filter-consent"
                label={t('contacts.consent.label')}
                value={filters.opted_out}
                options={[
                    { id: 'false', name: t('contacts.consent.subscribed') },
                    { id: 'true', name: t('contacts.consent.optedOut') },
                ]}
                onChange={(opted_out) => go({ opted_out })}
            />
            {CONSENT_CATEGORIES.map((k) => (
                <FilterSelect
                    key={k}
                    id={`filter-${k}-consent`}
                    label={t(`consent.filters.${k}`)}
                    value={filters[`${k}_consent`]}
                    options={CONSENT_STATUSES.map((s) => ({
                        id: s,
                        name: t(`consent.statuses.${s}`),
                    }))}
                    onChange={(v) => go({ [`${k}_consent`]: v })}
                />
            ))}
            <FilterSelect
                id="filter-reachability"
                label={t('consent.filters.reachability')}
                value={filters.reachability}
                options={[
                    {
                        id: 'meta_marketing_stopped',
                        name: t('consent.flags.metaStopped'),
                    },
                    {
                        id: 'undeliverable',
                        name: t('consent.flags.undeliverable'),
                    },
                ]}
                onChange={(reachability) => go({ reachability })}
            />
            {active && (
                <Button
                    variant="ghost"
                    onClick={() => {
                        setQ('');
                        router.push('/contacts');
                    }}
                >
                    {t('contacts.filters.clear')}
                </Button>
            )}
        </div>
    );
}

// ── Export ─────────────────────────────────────────────────────────────────

export function ExportMenu({ query }: { query: string }) {
    const t = useT();
    const href = (format: 'csv' | 'xlsx') => {
        const sp = new URLSearchParams(query);
        sp.set('format', format);
        return `/bff/contacts/export?${sp}`;
    };
    return (
        <DropdownMenu>
            <DropdownMenuTrigger asChild>
                <Button variant="outline" shortcut="e">
                    <Download /> {t('contacts.export')} <ChevronDown />
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
    const t = useT();
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
                <Button shortcut="n">
                    <UserPlus /> {t('contacts.create.title')}
                </Button>
            </DialogTrigger>
            <DialogContent>
                <DialogHeader>
                    <DialogTitle>{t('contacts.create.title')}</DialogTitle>
                    <DialogDescription>
                        {t('contacts.create.description')}
                    </DialogDescription>
                </DialogHeader>
                <form onSubmit={submit} className="space-y-4">
                    <input type="hidden" name="idempotency_key" value={key} />
                    <FormStatus state={state?.ok ? null : state} />
                    <FieldGroup>
                        <TextField
                            label={t('contacts.fields.phone')}
                            name="phone"
                            type="tel"
                            required
                            autoFocus
                            placeholder="+905551112233"
                            hint={t('contacts.create.phoneHint')}
                            minLength={7}
                            maxLength={24}
                            error={fieldError(state, 'phone')}
                        />
                        <TextField
                            label={t('contacts.fields.name')}
                            name="name"
                            maxLength={200}
                            error={fieldError(state, 'name')}
                        />
                        <TextField
                            label={t('contacts.fields.email')}
                            name="email"
                            type="email"
                            maxLength={254}
                            error={fieldError(state, 'email')}
                        />
                    </FieldGroup>
                    <Button type="submit" className="w-full" disabled={pending}>
                        {pending && <Spinner />}
                        {t('contacts.create.submit')}
                    </Button>
                </form>
            </DialogContent>
        </Dialog>
    );
}

// ── Table ──────────────────────────────────────────────────────────────────
