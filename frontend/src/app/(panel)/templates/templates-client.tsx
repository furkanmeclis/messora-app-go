'use client';

import { useMemo, useState, useTransition } from 'react';
import { usePathname, useRouter } from 'next/navigation';
import { AlertCircle, FileText, Plus, Trash2, X } from 'lucide-react';
import { toast } from 'sonner';
import { newIdempotencyKey, notify } from '@/components/notify';
import { useReauth } from '@/components/reauth';
import { Alert, AlertDescription } from '@/components/ui/alert';
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
import { Card, CardContent } from '@/components/ui/card';
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogHeader,
    DialogTitle,
    DialogTrigger,
} from '@/components/ui/dialog';
import {
    Empty,
    EmptyDescription,
    EmptyHeader,
    EmptyMedia,
    EmptyTitle,
} from '@/components/ui/empty';
import {
    Field,
    FieldDescription,
    FieldGroup,
    FieldLabel,
    FieldLegend,
    FieldSet,
} from '@/components/ui/field';
import { Input } from '@/components/ui/input';
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
import { TEMPLATE_STATUSES, variablesOf } from '@/lib/templates';
import type { ActionResult, Schemas, Template } from '@/lib/types';
import { createTemplateAction, deleteTemplateAction } from './actions';
import { TemplatePreview } from './template-preview';

type Option = { id: string; name: string };
type Failure = Extract<ActionResult<unknown>, { ok: false }>;

const ALL = 'all';

export function TemplateStatusBadge({ status }: { status: string }) {
    const s = status.toUpperCase();
    const variant =
        s === 'APPROVED'
            ? 'secondary'
            : s === 'REJECTED' || s === 'DISABLED'
              ? 'destructive'
              : 'outline';
    return (
        <Badge variant={variant} data-status={s}>
            {s.charAt(0) + s.slice(1).toLowerCase()}
        </Badge>
    );
}

// ── Filters ───────────────────────────────────────────────────────────────

export function TemplateFilters({
    accounts,
    account,
    status,
}: {
    accounts: Option[];
    account: string;
    status: string;
}) {
    const router = useRouter();
    const pathname = usePathname();
    const go = (next: { account?: string; status?: string }) => {
        const q = new URLSearchParams();
        const a = next.account ?? account;
        const s = next.status ?? status;
        if (a && a !== ALL) q.set('account', a);
        if (s && s !== ALL) q.set('status', s);
        const qs = q.toString();
        router.push(qs ? `${pathname}?${qs}` : pathname);
    };
    return (
        <div className="flex flex-wrap items-center gap-2">
            {accounts.length > 1 && (
                <Select
                    value={account || ALL}
                    onValueChange={(v) => go({ account: v })}
                >
                    <SelectTrigger className="w-56" aria-label="Account">
                        <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                        <SelectItem value={ALL}>All accounts</SelectItem>
                        {accounts.map((a) => (
                            <SelectItem key={a.id} value={a.id}>
                                {a.name}
                            </SelectItem>
                        ))}
                    </SelectContent>
                </Select>
            )}
            <Select
                value={status || ALL}
                onValueChange={(v) => go({ status: v })}
            >
                <SelectTrigger className="w-44" aria-label="Status">
                    <SelectValue />
                </SelectTrigger>
                <SelectContent>
                    <SelectItem value={ALL}>All statuses</SelectItem>
                    {TEMPLATE_STATUSES.map((s) => (
                        <SelectItem key={s} value={s}>
                            {s.charAt(0) + s.slice(1).toLowerCase()}
                        </SelectItem>
                    ))}
                </SelectContent>
            </Select>
            {(account || status) && (
                <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => router.push(pathname)}
                >
                    <X /> Clear filters
                </Button>
            )}
        </div>
    );
}

// ── List ──────────────────────────────────────────────────────────────────

export function TemplateList({
    templates,
    manage,
    filtered,
    hasAccounts,
}: {
    templates: Template[];
    manage: boolean;
    filtered: boolean;
    hasAccounts: boolean;
}) {
    const formatDateTime = useFormatDateTime();
    if (templates.length === 0) {
        return (
            <Card>
                <CardContent>
                    <Empty className="py-10">
                        <EmptyHeader>
                            <EmptyMedia variant="icon">
                                <FileText />
                            </EmptyMedia>
                            <EmptyTitle>
                                {filtered
                                    ? 'No templates match these filters'
                                    : 'No templates yet'}
                            </EmptyTitle>
                            <EmptyDescription>
                                {filtered
                                    ? 'Try another account or status.'
                                    : hasAccounts
                                      ? 'Create a template here or sync the account to import the ones made in Meta Business Suite.'
                                      : 'Templates appear once a WhatsApp account is connected in Channels.'}
                            </EmptyDescription>
                        </EmptyHeader>
                    </Empty>
                </CardContent>
            </Card>
        );
    }
    return (
        <ul className="grid gap-4 md:grid-cols-2" aria-label="Templates">
            {templates.map((t) => (
                <li key={t.id}>
                    <Card className="h-full" data-testid="template">
                        <CardContent className="space-y-3">
                            <div className="flex items-start justify-between gap-2">
                                <div className="min-w-0">
                                    <h2 className="truncate font-mono text-sm font-medium">
                                        {t.name}
                                    </h2>
                                    <p className="text-xs text-muted-foreground">
                                        {t.language} ·{' '}
                                        {t.category.toLowerCase()} ·{' '}
                                        {t.parameter_format.toLowerCase()}
                                        {t.account_name
                                            ? ` · ${t.account_name}`
                                            : ''}
                                    </p>
                                </div>
                                <div className="flex shrink-0 items-center gap-1">
                                    <TemplateStatusBadge status={t.status} />
                                    {manage && <DeleteTemplate template={t} />}
                                </div>
                            </div>
                            {t.status.toUpperCase() === 'REJECTED' &&
                                t.rejected_reason && (
                                    <Alert variant="destructive">
                                        <AlertCircle />
                                        <AlertDescription>
                                            Rejected: {t.rejected_reason}
                                        </AlertDescription>
                                    </Alert>
                                )}
                            <TemplatePreview components={t.components} />
                            <p className="text-xs text-muted-foreground">
                                Synced {formatDateTime(t.synced_at)}
                                {t.quality_score &&
                                t.quality_score !== 'UNKNOWN'
                                    ? ` · quality ${t.quality_score.toLowerCase()}`
                                    : ''}
                            </p>
                        </CardContent>
                    </Card>
                </li>
            ))}
        </ul>
    );
}

function DeleteTemplate({ template }: { template: Template }) {
    const { guard } = useReauth();
    const [open, setOpen] = useState(false);
    const [pending, startTransition] = useTransition();
    return (
        <AlertDialog open={open} onOpenChange={setOpen}>
            <AlertDialogTrigger asChild>
                <Button
                    size="icon-sm"
                    variant="ghost"
                    aria-label={`Delete ${template.name} (${template.language})`}
                >
                    <Trash2 />
                </Button>
            </AlertDialogTrigger>
            <AlertDialogContent>
                <AlertDialogHeader>
                    <AlertDialogTitle>
                        Delete {template.name} ({template.language})?
                    </AlertDialogTitle>
                    <AlertDialogDescription>
                        This language version is deleted on Meta as well.
                        Campaigns that use it can no longer start, and Meta does
                        not let you reuse the name for 30 days.
                    </AlertDialogDescription>
                </AlertDialogHeader>
                <AlertDialogFooter>
                    <AlertDialogCancel disabled={pending}>
                        Cancel
                    </AlertDialogCancel>
                    <AlertDialogAction
                        variant="destructive"
                        disabled={pending}
                        onClick={(e) => {
                            e.preventDefault();
                            startTransition(async () => {
                                if (
                                    notify(
                                        await guard(() =>
                                            deleteTemplateAction(template.id),
                                        ),
                                    )
                                ) {
                                    setOpen(false);
                                }
                            });
                        }}
                    >
                        {pending && <Spinner />} Delete
                    </AlertDialogAction>
                </AlertDialogFooter>
            </AlertDialogContent>
        </AlertDialog>
    );
}

// ── Create ────────────────────────────────────────────────────────────────

type ButtonDraft = {
    key: number;
    type: 'QUICK_REPLY' | 'URL' | 'PHONE_NUMBER';
    text: string;
    value: string;
    example: string;
};

type Draft = {
    account_id: string;
    name: string;
    language: string;
    category: 'MARKETING' | 'UTILITY' | 'AUTHENTICATION';
    format: 'POSITIONAL' | 'NAMED';
    header: string;
    body: string;
    footer: string;
    examples: Record<string, string>;
    buttons: ButtonDraft[];
};

const LANGUAGES = [
    'en_US',
    'en_GB',
    'tr',
    'de',
    'fr',
    'es',
    'it',
    'pt_BR',
    'nl',
    'ar',
    'ru',
];

function checkVariables(
    vars: string[],
    format: Draft['format'],
): string | null {
    if (format === 'POSITIONAL') {
        const bad = vars.find((v) => !/^[1-9][0-9]*$/.test(v));
        if (bad)
            return `Positional variables are numbers: use {{1}}, {{2}} … instead of {{${bad}}}.`;
        const nums = vars.map(Number).sort((a, b) => a - b);
        if (nums.some((n, i) => n !== i + 1))
            return 'Number variables in order without gaps: {{1}}, {{2}}, {{3}} …';
        return null;
    }
    const bad = vars.find((v) => !/^[a-z_][a-z0-9_]*$/.test(v));
    return bad
        ? `Named variables use lowercase letters, digits and underscores: {{${bad}}} is not allowed.`
        : null;
}

/** Meta's components JSON for the draft. */
function buildComponents(d: Draft): Record<string, unknown>[] {
    const named = d.format === 'NAMED';
    const ex = (v: string) => d.examples[v] ?? '';
    const out: Record<string, unknown>[] = [];
    const header = d.header.trim();
    if (header) {
        const vars = variablesOf(header);
        const c: Record<string, unknown> = {
            type: 'HEADER',
            format: 'TEXT',
            text: header,
        };
        if (vars.length > 0) {
            c.example = named
                ? {
                      header_text_named_params: vars.map((v) => ({
                          param_name: v,
                          example: ex(`h:${v}`),
                      })),
                  }
                : { header_text: vars.map((v) => ex(`h:${v}`)) };
        }
        out.push(c);
    }
    const body = d.body.trim();
    const bodyVars = variablesOf(body);
    const bc: Record<string, unknown> = { type: 'BODY', text: body };
    if (bodyVars.length > 0) {
        const sorted = named
            ? bodyVars
            : [...bodyVars].sort((a, b) => Number(a) - Number(b));
        bc.example = named
            ? {
                  body_text_named_params: sorted.map((v) => ({
                      param_name: v,
                      example: ex(`b:${v}`),
                  })),
              }
            : { body_text: [sorted.map((v) => ex(`b:${v}`))] };
    }
    out.push(bc);
    if (d.footer.trim()) out.push({ type: 'FOOTER', text: d.footer.trim() });
    if (d.buttons.length > 0) {
        out.push({
            type: 'BUTTONS',
            buttons: d.buttons.map((b) => {
                if (b.type === 'URL') {
                    const dyn = variablesOf(b.value).length > 0;
                    return {
                        type: 'URL',
                        text: b.text.trim(),
                        url: b.value.trim(),
                        ...(dyn ? { example: [b.example.trim()] } : {}),
                    };
                }
                if (b.type === 'PHONE_NUMBER') {
                    return {
                        type: 'PHONE_NUMBER',
                        text: b.text.trim(),
                        phone_number: b.value.trim(),
                    };
                }
                return { type: 'QUICK_REPLY', text: b.text.trim() };
            }),
        });
    }
    return out;
}

/** Problems Meta would reject anyway, keyed by form field. */
function validate(d: Draft): Record<string, string> {
    const e: Record<string, string> = {};
    if (!d.account_id) e.account_id = 'Choose an account.';
    if (!/^[a-z0-9_]{1,512}$/.test(d.name))
        e.name = 'Use lowercase letters, digits and underscores.';
    if (d.language.trim().length < 2) e.language = 'Language code, e.g. en_US.';
    if (!d.body.trim()) e.body = 'Body text is required.';
    else if (d.body.length > 1024) e.body = 'At most 1024 characters.';
    const bodyProblem = checkVariables(variablesOf(d.body), d.format);
    if (bodyProblem) e.body = bodyProblem;
    const hv = variablesOf(d.header);
    if (d.header.length > 60) e.header = 'At most 60 characters.';
    else if (hv.length > 1) e.header = 'A header can have one variable.';
    else if (hv.length === 1) {
        const p = checkVariables(hv, d.format);
        if (p) e.header = p;
    }
    if (d.footer.length > 60) e.footer = 'At most 60 characters.';
    if (variablesOf(d.footer).length > 0)
        e.footer = 'The footer cannot contain variables.';
    for (const v of hv)
        if (!d.examples[`h:${v}`]?.trim())
            e[`example-h:${v}`] = 'Meta needs an example value.';
    for (const v of variablesOf(d.body))
        if (!d.examples[`b:${v}`]?.trim())
            e[`example-b:${v}`] = 'Meta needs an example value.';
    d.buttons.forEach((b, i) => {
        if (!b.text.trim()) e[`button-${i}-text`] = 'Button text is required.';
        if (b.type === 'URL') {
            if (!/^https:\/\/\S+$/.test(b.value.trim()))
                e[`button-${i}-value`] = 'An https:// address.';
            else if (variablesOf(b.value).length > 0) {
                if (!/\{\{1\}\}$/.test(b.value.trim()))
                    e[`button-${i}-value`] =
                        'Only one {{1}}, at the end of the address.';
                else if (!b.example.trim())
                    e[`button-${i}-example`] = 'Full example address.';
            }
        }
        if (b.type === 'PHONE_NUMBER' && !/^\+?[0-9 ]{7,20}$/.test(b.value))
            e[`button-${i}-value`] = 'Phone number in international format.';
    });
    return e;
}

const EMPTY: Omit<Draft, 'account_id'> = {
    name: '',
    language: 'en_US',
    category: 'MARKETING',
    format: 'POSITIONAL',
    header: '',
    body: '',
    footer: '',
    examples: {},
    buttons: [],
};

export function CreateTemplateDialog({ accounts }: { accounts: Option[] }) {
    const { guard } = useReauth();
    const [open, setOpen] = useState(false);
    const [idem, setIdem] = useState(newIdempotencyKey);
    const [pending, startTransition] = useTransition();
    const [d, setD] = useState<Draft>({
        ...EMPTY,
        account_id: accounts.length === 1 ? accounts[0]!.id : '',
    });
    const [errors, setErrors] = useState<Record<string, string>>({});
    const [failure, setFailure] = useState<Failure | null>(null);
    const [nextKey, setNextKey] = useState(1);
    const set = <K extends keyof Draft>(k: K, v: Draft[K]) =>
        setD((p) => ({ ...p, [k]: v }));

    const headerVars = useMemo(() => variablesOf(d.header), [d.header]);
    const bodyVars = useMemo(() => variablesOf(d.body), [d.body]);
    const components = useMemo(() => buildComponents(d), [d]);
    const previewValues = useMemo(() => {
        const h: Record<string, string> = {};
        const b: Record<string, string> = {};
        for (const v of headerVars) h[v] = d.examples[`h:${v}`] ?? '';
        for (const v of bodyVars) b[v] = d.examples[`b:${v}`] ?? '';
        return { header: h, body: b };
    }, [headerVars, bodyVars, d.examples]);

    const submit = (e: React.FormEvent) => {
        e.preventDefault();
        const found = validate(d);
        setErrors(found);
        setFailure(null);
        if (Object.keys(found).length > 0) {
            const first = Object.keys(found)[0]!;
            document.getElementById(`tpl-${first}`)?.focus();
            return;
        }
        const input: Schemas['CreateTemplateRequest'] = {
            account_id: d.account_id,
            name: d.name,
            language: d.language.trim(),
            category: d.category,
            parameter_format: d.format,
            components,
        };
        startTransition(async () => {
            const r = await guard(() => createTemplateAction(input, idem));
            if (r.ok) {
                notify(r);
                setOpen(false);
                setD({ ...EMPTY, account_id: d.account_id });
                setIdem(newIdempotencyKey());
            } else {
                setErrors(r.fieldErrors ?? {});
                setFailure(r);
                setIdem(newIdempotencyKey());
                if (r.code !== 'VALIDATION_ERROR') toast.error(r.message);
            }
        });
    };

    const err = (k: string) =>
        errors[k] ? (
            <p
                id={`tpl-${k}-error`}
                role="alert"
                className="text-sm text-destructive"
            >
                {errors[k]}
            </p>
        ) : null;
    const aria = (k: string) => ({
        id: `tpl-${k}`,
        'aria-invalid': errors[k] ? true : undefined,
        'aria-describedby': errors[k] ? `tpl-${k}-error` : undefined,
    });
    const knownFields = new Set([
        'account_id',
        'name',
        'language',
        'category',
        'header',
        'body',
        'footer',
    ]);
    const otherErrors = Object.entries(errors).filter(
        ([k]) =>
            !knownFields.has(k) &&
            !k.startsWith('example-') &&
            !k.startsWith('button-'),
    );

    return (
        <Dialog open={open} onOpenChange={setOpen}>
            <DialogTrigger asChild>
                <Button>
                    <Plus /> New template
                </Button>
            </DialogTrigger>
            <DialogContent className="max-h-[92svh] overflow-y-auto sm:max-w-3xl">
                <DialogHeader>
                    <DialogTitle>New template</DialogTitle>
                    <DialogDescription>
                        Submitted to Meta for review; it can be sent once it is
                        approved.
                    </DialogDescription>
                </DialogHeader>
                <form
                    onSubmit={submit}
                    noValidate
                    className="grid gap-6 md:grid-cols-[1fr_16rem]"
                >
                    <div className="space-y-5">
                        {failure &&
                            (failure.code !== 'VALIDATION_ERROR' ||
                                otherErrors.length > 0) && (
                                <Alert variant="destructive">
                                    <AlertCircle />
                                    <AlertDescription>
                                        <p>{failure.message}</p>
                                        {otherErrors.map(([k, v]) => (
                                            <p key={k}>
                                                <code>{k}</code> {v}
                                            </p>
                                        ))}
                                    </AlertDescription>
                                </Alert>
                            )}
                        <FieldGroup>
                            <Field data-invalid={!!errors.account_id}>
                                <FieldLabel htmlFor="tpl-account_id">
                                    Account
                                </FieldLabel>
                                <Select
                                    value={d.account_id}
                                    onValueChange={(v) => set('account_id', v)}
                                >
                                    <SelectTrigger
                                        {...aria('account_id')}
                                        className="w-full"
                                    >
                                        <SelectValue placeholder="Choose an account" />
                                    </SelectTrigger>
                                    <SelectContent>
                                        {accounts.map((a) => (
                                            <SelectItem key={a.id} value={a.id}>
                                                {a.name}
                                            </SelectItem>
                                        ))}
                                    </SelectContent>
                                </Select>
                                {err('account_id')}
                            </Field>
                            <div className="grid gap-4 sm:grid-cols-2">
                                <Field data-invalid={!!errors.name}>
                                    <FieldLabel htmlFor="tpl-name">
                                        Name
                                    </FieldLabel>
                                    <Input
                                        {...aria('name')}
                                        value={d.name}
                                        maxLength={512}
                                        placeholder="order_update"
                                        autoComplete="off"
                                        onChange={(e) =>
                                            set(
                                                'name',
                                                e.target.value
                                                    .toLowerCase()
                                                    .replace(/[\s-]+/g, '_'),
                                            )
                                        }
                                    />
                                    {err('name') ?? (
                                        <FieldDescription>
                                            Lowercase, digits and underscores.
                                        </FieldDescription>
                                    )}
                                </Field>
                                <Field data-invalid={!!errors.language}>
                                    <FieldLabel htmlFor="tpl-language">
                                        Language
                                    </FieldLabel>
                                    <Input
                                        {...aria('language')}
                                        value={d.language}
                                        list="tpl-languages"
                                        maxLength={15}
                                        onChange={(e) =>
                                            set('language', e.target.value)
                                        }
                                    />
                                    <datalist id="tpl-languages">
                                        {LANGUAGES.map((l) => (
                                            <option key={l} value={l} />
                                        ))}
                                    </datalist>
                                    {err('language')}
                                </Field>
                                <Field>
                                    <FieldLabel htmlFor="tpl-category">
                                        Category
                                    </FieldLabel>
                                    <Select
                                        value={d.category}
                                        onValueChange={(v) =>
                                            set(
                                                'category',
                                                v as Draft['category'],
                                            )
                                        }
                                    >
                                        <SelectTrigger
                                            id="tpl-category"
                                            className="w-full"
                                        >
                                            <SelectValue />
                                        </SelectTrigger>
                                        <SelectContent>
                                            <SelectItem value="MARKETING">
                                                Marketing
                                            </SelectItem>
                                            <SelectItem value="UTILITY">
                                                Utility
                                            </SelectItem>
                                            <SelectItem value="AUTHENTICATION">
                                                Authentication
                                            </SelectItem>
                                        </SelectContent>
                                    </Select>
                                </Field>
                                <Field>
                                    <FieldLabel htmlFor="tpl-format">
                                        Variables
                                    </FieldLabel>
                                    <Select
                                        value={d.format}
                                        onValueChange={(v) =>
                                            set('format', v as Draft['format'])
                                        }
                                    >
                                        <SelectTrigger
                                            id="tpl-format"
                                            className="w-full"
                                        >
                                            <SelectValue />
                                        </SelectTrigger>
                                        <SelectContent>
                                            <SelectItem value="POSITIONAL">
                                                Numbered: {'{{1}}'}, {'{{2}}'}
                                            </SelectItem>
                                            <SelectItem value="NAMED">
                                                Named: {'{{first_name}}'}
                                            </SelectItem>
                                        </SelectContent>
                                    </Select>
                                </Field>
                            </div>
                            <Field data-invalid={!!errors.header}>
                                <FieldLabel htmlFor="tpl-header">
                                    Header text (optional)
                                </FieldLabel>
                                <Input
                                    {...aria('header')}
                                    value={d.header}
                                    maxLength={60}
                                    onChange={(e) =>
                                        set('header', e.target.value)
                                    }
                                />
                                {err('header')}
                            </Field>
                            <Field data-invalid={!!errors.body}>
                                <FieldLabel htmlFor="tpl-body">Body</FieldLabel>
                                <Textarea
                                    {...aria('body')}
                                    value={d.body}
                                    rows={5}
                                    maxLength={1024}
                                    placeholder={
                                        d.format === 'NAMED'
                                            ? 'Hello {{first_name}}, your order {{order_id}} has shipped.'
                                            : 'Hello {{1}}, your order {{2}} has shipped.'
                                    }
                                    onChange={(e) =>
                                        set('body', e.target.value)
                                    }
                                />
                                {err('body') ?? (
                                    <FieldDescription>
                                        {d.format === 'NAMED'
                                            ? 'Insert variables as {{name}}.'
                                            : 'Insert variables as {{1}}, {{2}} … in order.'}
                                    </FieldDescription>
                                )}
                            </Field>
                            {(headerVars.length > 0 || bodyVars.length > 0) && (
                                <FieldSet>
                                    <FieldLegend variant="label">
                                        Example values
                                    </FieldLegend>
                                    <FieldDescription>
                                        Meta reviews the template with these.
                                    </FieldDescription>
                                    <div className="grid gap-3 sm:grid-cols-2">
                                        {[
                                            ...headerVars.map((v) => ({
                                                k: `h:${v}`,
                                                label: `Header {{${v}}}`,
                                            })),
                                            ...bodyVars.map((v) => ({
                                                k: `b:${v}`,
                                                label: `Body {{${v}}}`,
                                            })),
                                        ].map(({ k, label }) => (
                                            <Field
                                                key={k}
                                                data-invalid={
                                                    !!errors[`example-${k}`]
                                                }
                                            >
                                                <FieldLabel
                                                    htmlFor={`tpl-example-${k}`}
                                                >
                                                    {label}
                                                </FieldLabel>
                                                <Input
                                                    {...aria(`example-${k}`)}
                                                    value={d.examples[k] ?? ''}
                                                    onChange={(e) =>
                                                        set('examples', {
                                                            ...d.examples,
                                                            [k]: e.target.value,
                                                        })
                                                    }
                                                />
                                                {err(`example-${k}`)}
                                            </Field>
                                        ))}
                                    </div>
                                </FieldSet>
                            )}
                            <Field data-invalid={!!errors.footer}>
                                <FieldLabel htmlFor="tpl-footer">
                                    Footer (optional)
                                </FieldLabel>
                                <Input
                                    {...aria('footer')}
                                    value={d.footer}
                                    maxLength={60}
                                    placeholder="Reply STOP to unsubscribe"
                                    onChange={(e) =>
                                        set('footer', e.target.value)
                                    }
                                />
                                {err('footer')}
                            </Field>
                            <FieldSet>
                                <FieldLegend variant="label">
                                    Buttons (optional)
                                </FieldLegend>
                                {d.buttons.map((b, i) => (
                                    <div
                                        key={b.key}
                                        className="space-y-2 rounded-lg border p-3"
                                    >
                                        <div className="flex items-center gap-2">
                                            <Select
                                                value={b.type}
                                                onValueChange={(v) =>
                                                    set(
                                                        'buttons',
                                                        d.buttons.map((x) =>
                                                            x.key === b.key
                                                                ? {
                                                                      ...x,
                                                                      type: v as ButtonDraft['type'],
                                                                  }
                                                                : x,
                                                        ),
                                                    )
                                                }
                                            >
                                                <SelectTrigger
                                                    className="w-40"
                                                    aria-label={`Button ${i + 1} type`}
                                                >
                                                    <SelectValue />
                                                </SelectTrigger>
                                                <SelectContent>
                                                    <SelectItem value="QUICK_REPLY">
                                                        Quick reply
                                                    </SelectItem>
                                                    <SelectItem value="URL">
                                                        Website
                                                    </SelectItem>
                                                    <SelectItem value="PHONE_NUMBER">
                                                        Call
                                                    </SelectItem>
                                                </SelectContent>
                                            </Select>
                                            <Input
                                                {...aria(`button-${i}-text`)}
                                                aria-label={`Button ${i + 1} text`}
                                                placeholder="Button text"
                                                maxLength={25}
                                                value={b.text}
                                                onChange={(e) =>
                                                    set(
                                                        'buttons',
                                                        d.buttons.map((x) =>
                                                            x.key === b.key
                                                                ? {
                                                                      ...x,
                                                                      text: e
                                                                          .target
                                                                          .value,
                                                                  }
                                                                : x,
                                                        ),
                                                    )
                                                }
                                            />
                                            <Button
                                                type="button"
                                                size="icon"
                                                variant="ghost"
                                                aria-label={`Remove button ${i + 1}`}
                                                onClick={() =>
                                                    set(
                                                        'buttons',
                                                        d.buttons.filter(
                                                            (x) =>
                                                                x.key !== b.key,
                                                        ),
                                                    )
                                                }
                                            >
                                                <X />
                                            </Button>
                                        </div>
                                        {err(`button-${i}-text`)}
                                        {b.type !== 'QUICK_REPLY' && (
                                            <>
                                                <Input
                                                    {...aria(
                                                        `button-${i}-value`,
                                                    )}
                                                    aria-label={
                                                        b.type === 'URL'
                                                            ? `Button ${i + 1} address`
                                                            : `Button ${i + 1} phone number`
                                                    }
                                                    placeholder={
                                                        b.type === 'URL'
                                                            ? 'https://example.com/track/{{1}}'
                                                            : '+15551234567'
                                                    }
                                                    value={b.value}
                                                    onChange={(e) =>
                                                        set(
                                                            'buttons',
                                                            d.buttons.map(
                                                                (x) =>
                                                                    x.key ===
                                                                    b.key
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
                                                {err(`button-${i}-value`)}
                                            </>
                                        )}
                                        {b.type === 'URL' &&
                                            variablesOf(b.value).length > 0 && (
                                                <>
                                                    <Input
                                                        {...aria(
                                                            `button-${i}-example`,
                                                        )}
                                                        aria-label={`Button ${i + 1} example address`}
                                                        placeholder="https://example.com/track/ABC123"
                                                        value={b.example}
                                                        onChange={(e) =>
                                                            set(
                                                                'buttons',
                                                                d.buttons.map(
                                                                    (x) =>
                                                                        x.key ===
                                                                        b.key
                                                                            ? {
                                                                                  ...x,
                                                                                  example:
                                                                                      e
                                                                                          .target
                                                                                          .value,
                                                                              }
                                                                            : x,
                                                                ),
                                                            )
                                                        }
                                                    />
                                                    {err(`button-${i}-example`)}
                                                </>
                                            )}
                                    </div>
                                ))}
                                {d.buttons.length < 10 && (
                                    <Button
                                        type="button"
                                        variant="outline"
                                        size="sm"
                                        className="w-fit"
                                        onClick={() => {
                                            set('buttons', [
                                                ...d.buttons,
                                                {
                                                    key: nextKey,
                                                    type: 'QUICK_REPLY',
                                                    text: '',
                                                    value: '',
                                                    example: '',
                                                },
                                            ]);
                                            setNextKey((k) => k + 1);
                                        }}
                                    >
                                        <Plus /> Add button
                                    </Button>
                                )}
                            </FieldSet>
                        </FieldGroup>
                        <Button
                            type="submit"
                            className="w-full"
                            disabled={pending}
                        >
                            {pending && <Spinner />} Submit for review
                        </Button>
                    </div>
                    <aside aria-label="Preview" className="space-y-2">
                        <p className="text-sm font-medium">Preview</p>
                        <TemplatePreview
                            components={components}
                            values={previewValues}
                        />
                    </aside>
                </form>
            </DialogContent>
        </Dialog>
    );
}
