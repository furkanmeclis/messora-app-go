'use client';

import { useMemo, useRef, useState, useTransition } from 'react';
import { useRouter } from 'next/navigation';
import {
    AlertCircle,
    Braces,
    Clock,
    Copy,
    ExternalLink,
    GalleryHorizontal,
    KeyRound,
    ListChecks,
    Phone,
    PhoneCall,
    Plus,
    Reply,
    Send,
    Trash2,
} from 'lucide-react';
import Link from 'next/link';
import { toast } from 'sonner';
import { newIdempotencyKey, notify } from '@/components/notify';
import { useReauth } from '@/components/reauth';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { FileDrop } from '@/components/ui/file-drop';
import { Input } from '@/components/ui/input';
import { RadioCard, RadioCards } from '@/components/ui/radio-cards';
import { Segmented } from '@/components/ui/segmented';
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from '@/components/ui/select';
import { Spinner } from '@/components/ui/spinner';
import { Switch } from '@/components/ui/switch';
import { Textarea } from '@/components/ui/textarea';
import { WhatsAppPreview } from '@/components/whatsapp-preview';
import { useT } from '@/i18n/client';
import type { Translate } from '@/i18n/translate';
import { variablesOf } from '@/lib/templates';
import type { Schemas } from '@/lib/types';
import { cn } from '@/lib/utils';
import { createTemplateAction } from '../actions';
import {
    BUTTON_LIMITS,
    LIMITS,
    allowedButtons,
    allowedHeaders,
    buildComponents,
    emptyCard,
    emptyDraft,
    nextVariable,
    validateDraft,
    type ButtonType,
    type CardButtonType,
    type Draft,
    type HeaderKind,
    type Kind,
} from './draft';

type Option = { id: string; name: string };

const LANGUAGES = [
    'tr',
    'en_US',
    'en_GB',
    'de',
    'fr',
    'es',
    'it',
    'pt_BR',
    'nl',
    'ar',
    'ru',
];

const BUTTON_ICON: Record<ButtonType, typeof Reply> = {
    QUICK_REPLY: Reply,
    URL: ExternalLink,
    PHONE_NUMBER: Phone,
    COPY_CODE: Copy,
    VOICE_CALL: PhoneCall,
    FLOW: ListChecks,
};

/** Character counter shown under a limited field. */
function Counter({ value, max }: { value: string; max: number }) {
    const n = [...value].length;
    return (
        <span
            className={cn(
                'text-xs text-muted-foreground tabular-nums',
                n > max && 'font-medium text-destructive',
            )}
        >
            {n}/{max}
        </span>
    );
}

function Section({
    n,
    title,
    hint,
    children,
    testId,
}: {
    n: number;
    title: string;
    hint?: string;
    children: React.ReactNode;
    testId?: string;
}) {
    return (
        <section
            className="space-y-4 border-b pb-6 last:border-b-0"
            data-testid={testId}
            aria-labelledby={`tpl-section-${n}`}
        >
            <div className="flex items-start gap-3">
                <span className="flex size-6 shrink-0 items-center justify-center rounded-full bg-primary text-xs font-semibold text-primary-foreground">
                    {n}
                </span>
                <div>
                    <h2 id={`tpl-section-${n}`} className="font-medium">
                        {title}
                    </h2>
                    {hint && (
                        <p className="text-sm text-muted-foreground">{hint}</p>
                    )}
                </div>
            </div>
            <div className="space-y-4 sm:pl-9">{children}</div>
        </section>
    );
}

function FieldRow({
    id,
    label,
    error,
    counter,
    hint,
    children,
}: {
    id: string;
    label: string;
    error?: string;
    counter?: React.ReactNode;
    hint?: string;
    children: React.ReactNode;
}) {
    return (
        <div className="space-y-1.5" data-invalid={error ? true : undefined}>
            <div className="flex items-center justify-between gap-2">
                <label htmlFor={id} className="text-sm font-medium">
                    {label}
                </label>
                {counter}
            </div>
            {children}
            {error ? (
                <p
                    id={`${id}-error`}
                    role="alert"
                    className="text-sm text-destructive"
                >
                    {error}
                </p>
            ) : (
                hint && <p className="text-xs text-muted-foreground">{hint}</p>
            )}
        </div>
    );
}

async function uploadSample(
    t: Translate,
    account: string,
    file: File,
): Promise<{ handle: string } | { error: string }> {
    const q = new URLSearchParams({ account_id: account, filename: file.name });
    const res = await fetch(`/bff/template-sample?${q}`, {
        method: 'POST',
        body: file,
        headers: { 'Content-Type': file.type || 'application/octet-stream' },
    });
    const out = (await res.json().catch(() => null)) as {
        data?: { handle: string };
        error?: { message?: string };
    } | null;
    if (!res.ok || !out?.data?.handle)
        return { error: out?.error?.message ?? t('common.somethingWentWrong') };
    return { handle: out.data.handle };
}

const SAMPLE_ACCEPT = {
    IMAGE: 'image/jpeg,image/png',
    VIDEO: 'video/mp4',
    DOCUMENT: 'application/pdf',
} as const;

/**
 * The template builder: one page with numbered sections (category and
 * language, header, body, footer, buttons, special types) and a live
 * WhatsApp preview. Meta's limits are checked as you type and again by the
 * server before anything is submitted.
 */
export function TemplateBuilder({ accounts }: { accounts: Option[] }) {
    const t = useT();
    const router = useRouter();
    const { guard } = useReauth();
    const [d, setD] = useState<Draft>(() =>
        emptyDraft(accounts.length === 1 ? accounts[0]!.id : ''),
    );
    const [errors, setErrors] = useState<Record<string, string>>({});
    const [server, setServer] = useState<{
        message: string;
        fields: [string, string][];
    } | null>(null);
    const [uploading, setUploading] = useState<string | null>(null);
    const [idem, setIdem] = useState(newIdempotencyKey);
    const [pending, startTransition] = useTransition();
    // The preview counts down to a sample end two days ahead.
    const [sampleEnd] = useState(
        () => Date.now() + 2 * 86_400_000 + 3 * 3_600_000,
    );
    const bodyRef = useRef<HTMLTextAreaElement>(null);
    const keyRef = useRef(100);
    const nextKey = () => ++keyRef.current;

    const set = <K extends keyof Draft>(k: K, v: Draft[K]) =>
        setD((p) => ({ ...p, [k]: v }));
    const components = useMemo(() => buildComponents(d), [d]);
    const headerVars = d.headerKind === 'TEXT' ? variablesOf(d.header) : [];
    const bodyVars = variablesOf(d.body);
    const named = d.format === 'NAMED';
    const marketingOnly = d.kind === 'carousel' || d.kind === 'lto';
    const bodyMax = d.kind === 'lto' ? LIMITS.ltoBody : LIMITS.body;
    const err = (k: string) => errors[k];
    const aria = (k: string) => ({
        id: `tpl-${k}`,
        'aria-invalid': errors[k] ? true : undefined,
        'aria-describedby': errors[k] ? `tpl-${k}-error` : undefined,
    });

    const setKind = (kind: Kind) =>
        setD((p) => ({
            ...p,
            kind,
            category:
                kind === 'authentication'
                    ? 'AUTHENTICATION'
                    : kind === 'standard'
                      ? p.category === 'AUTHENTICATION'
                          ? 'MARKETING'
                          : p.category
                      : 'MARKETING',
            headerKind: allowedHeaders(kind).includes(p.headerKind)
                ? p.headerKind
                : 'NONE',
            buttons: kind === 'standard' ? p.buttons : [],
        }));

    const sample = async (
        file: File | null,
        target: 'header' | number,
    ): Promise<void> => {
        if (!file) return;
        const field = target === 'header' ? 'header' : `card-${target}-media`;
        if (!d.account_id) {
            setErrors((e) => ({
                ...e,
                [field]: t('templates.validation.chooseAccount'),
            }));
            return;
        }
        setUploading(field);
        try {
            const r = await uploadSample(t, d.account_id, file);
            if ('error' in r) {
                setErrors((e) => ({ ...e, [field]: r.error }));
                return;
            }
            setErrors((e) => {
                const next = { ...e };
                delete next[field];
                return next;
            });
            const preview = file.type.startsWith('image/')
                ? URL.createObjectURL(file)
                : undefined;
            if (target === 'header') {
                setD((p) => ({
                    ...p,
                    headerHandle: r.handle,
                    headerFile: file.name,
                    headerPreview: preview,
                }));
            } else {
                setD((p) => ({
                    ...p,
                    cards: p.cards.map((c, k) =>
                        k === target
                            ? {
                                  ...c,
                                  handle: r.handle,
                                  file: file.name,
                                  preview,
                              }
                            : c,
                    ),
                }));
            }
        } finally {
            setUploading(null);
        }
    };

    const insertVariable = () => {
        const v = nextVariable(d.body, named);
        const token = `{{${v}}}`;
        const el = bodyRef.current;
        const at = el?.selectionStart ?? d.body.length;
        const end = el?.selectionEnd ?? at;
        set('body', d.body.slice(0, at) + token + d.body.slice(end));
        requestAnimationFrame(() => {
            el?.focus();
            el?.setSelectionRange(at + token.length, at + token.length);
        });
    };

    const submit = (e: React.FormEvent) => {
        e.preventDefault();
        const found = validateDraft(t, d);
        setErrors(found);
        setServer(null);
        if (Object.keys(found).length > 0) {
            const first = Object.keys(found)[0]!;
            document.getElementById(`tpl-${first}`)?.focus();
            toast.error(t('templates.builder.fix'));
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
            setIdem(newIdempotencyKey());
            if (r.ok) {
                notify(r);
                router.push('/templates');
                return;
            }
            setErrors(
                Object.fromEntries(
                    Object.entries(r.fieldErrors ?? {}).filter(
                        ([k]) => !k.startsWith('components'),
                    ),
                ),
            );
            setServer({
                message: r.message,
                fields: Object.entries(r.fieldErrors ?? {}).filter(([k]) =>
                    k.startsWith('components'),
                ),
            });
        });
    };

    const counts = d.buttons.reduce<Partial<Record<ButtonType, number>>>(
        (a, b) => ({ ...a, [b.type]: (a[b.type] ?? 0) + 1 }),
        {},
    );
    const addButton = (type: ButtonType) =>
        set('buttons', [
            ...d.buttons,
            {
                key: nextKey(),
                type,
                text: '',
                value: '',
                example: '',
                flowAction: 'navigate',
            },
        ]);
    const editButton = (key: number, patch: Partial<Draft['buttons'][0]>) =>
        set(
            'buttons',
            d.buttons.map((b) => (b.key === key ? { ...b, ...patch } : b)),
        );

    const previewExamples = {
        header: Object.fromEntries(
            headerVars.map((v) => [v, d.examples[`h:${v}`] ?? '']),
        ),
        body: Object.fromEntries(
            bodyVars.map((v) => [v, d.examples[`b:${v}`] ?? '']),
        ),
        cards: d.cards.map((c) =>
            Object.fromEntries(
                variablesOf(c.body).map((v) => [v, c.examples[v] ?? '']),
            ),
        ),
    };

    let n = 0;
    return (
        <form
            onSubmit={submit}
            noValidate
            className="grid gap-8 lg:grid-cols-[minmax(0,1fr)_22rem]"
            data-testid="template-builder"
        >
            <div className="min-w-0 space-y-6">
                {server && (
                    <Alert variant="destructive">
                        <AlertCircle />
                        <AlertTitle>{server.message}</AlertTitle>
                        {server.fields.length > 0 && (
                            <AlertDescription>
                                <ul className="list-disc pl-4">
                                    {server.fields.map(([k, v]) => (
                                        <li key={k}>
                                            <code className="text-xs">{k}</code>{' '}
                                            {v}
                                        </li>
                                    ))}
                                </ul>
                            </AlertDescription>
                        )}
                    </Alert>
                )}

                <Section
                    n={++n}
                    title={t('templates.builder.basics')}
                    hint={t('templates.builder.basicsHint')}
                >
                    <RadioCards
                        value={d.kind}
                        onValueChange={(v) => setKind(v as Kind)}
                        className="grid-cols-1 sm:grid-cols-2"
                        aria-label={t('templates.builder.kind')}
                    >
                        {(
                            [
                                ['standard', Send],
                                ['carousel', GalleryHorizontal],
                                ['lto', Clock],
                                ['authentication', KeyRound],
                            ] as const
                        ).map(([k, Icon]) => (
                            <RadioCard key={k} value={k}>
                                <span className="flex items-center gap-2 font-medium">
                                    <Icon className="size-4 text-muted-foreground" />
                                    {t(`templates.builder.kinds.${k}`)}
                                </span>
                                <span className="block text-xs text-muted-foreground">
                                    {t(`templates.builder.kindHints.${k}`)}
                                </span>
                            </RadioCard>
                        ))}
                    </RadioCards>
                    <div className="grid gap-4 sm:grid-cols-2">
                        <FieldRow
                            id="tpl-account_id"
                            label={t('templates.create.account')}
                            error={err('account_id')}
                        >
                            <Select
                                value={d.account_id}
                                onValueChange={(v) => set('account_id', v)}
                            >
                                <SelectTrigger
                                    {...aria('account_id')}
                                    className="w-full"
                                >
                                    <SelectValue
                                        placeholder={t(
                                            'templates.create.chooseAccount',
                                        )}
                                    />
                                </SelectTrigger>
                                <SelectContent>
                                    {accounts.map((a) => (
                                        <SelectItem key={a.id} value={a.id}>
                                            {a.name}
                                        </SelectItem>
                                    ))}
                                </SelectContent>
                            </Select>
                        </FieldRow>
                        <FieldRow
                            id="tpl-category"
                            label={t('templates.create.category')}
                            hint={
                                marketingOnly
                                    ? t('templates.builder.marketingOnly')
                                    : d.kind === 'authentication'
                                      ? t('templates.builder.authOnly')
                                      : undefined
                            }
                        >
                            <Select
                                value={d.category}
                                disabled={d.kind !== 'standard'}
                                onValueChange={(v) =>
                                    set('category', v as Draft['category'])
                                }
                            >
                                <SelectTrigger
                                    id="tpl-category"
                                    className="w-full"
                                >
                                    <SelectValue />
                                </SelectTrigger>
                                <SelectContent>
                                    {(d.kind === 'standard'
                                        ? (['MARKETING', 'UTILITY'] as const)
                                        : ([d.category] as const)
                                    ).map((c) => (
                                        <SelectItem key={c} value={c}>
                                            {t(`templates.categories.${c}`)}
                                        </SelectItem>
                                    ))}
                                </SelectContent>
                            </Select>
                        </FieldRow>
                        <FieldRow
                            id="tpl-name"
                            label={t('templates.create.name')}
                            error={err('name')}
                            hint={t('templates.create.nameHint')}
                        >
                            <Input
                                {...aria('name')}
                                value={d.name}
                                maxLength={512}
                                autoComplete="off"
                                placeholder={t(
                                    'templates.create.namePlaceholder',
                                )}
                                onChange={(e) =>
                                    set(
                                        'name',
                                        e.target.value
                                            .toLowerCase()
                                            .replace(/[\s-]+/g, '_'),
                                    )
                                }
                            />
                        </FieldRow>
                        <FieldRow
                            id="tpl-language"
                            label={t('templates.create.language')}
                            error={err('language')}
                        >
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
                        </FieldRow>
                        {d.kind !== 'authentication' && (
                            <FieldRow
                                id="tpl-format"
                                label={t('templates.create.variables')}
                            >
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
                                            {t('templates.create.numbered', {
                                                example: '{{1}}, {{2}}',
                                            })}
                                        </SelectItem>
                                        <SelectItem value="NAMED">
                                            {t('templates.create.namedFormat', {
                                                example: '{{first_name}}',
                                            })}
                                        </SelectItem>
                                    </SelectContent>
                                </Select>
                            </FieldRow>
                        )}
                    </div>
                </Section>

                {allowedHeaders(d.kind).length > 1 && (
                    <Section
                        n={++n}
                        title={t('templates.builder.header')}
                        hint={
                            d.kind === 'lto'
                                ? t('templates.builder.ltoHeaderHint')
                                : t('templates.builder.headerHint')
                        }
                        testId="section-header"
                    >
                        <Segmented
                            aria-label={t('templates.builder.header')}
                            value={d.headerKind}
                            onValueChange={(k: HeaderKind) =>
                                setD((p) => ({
                                    ...p,
                                    headerKind: k,
                                    headerHandle: '',
                                    headerFile: '',
                                    headerPreview: undefined,
                                }))
                            }
                            options={allowedHeaders(d.kind).map((k) => ({
                                value: k,
                                label: t(`templates.builder.headerKinds.${k}`),
                            }))}
                        />
                        {d.headerKind === 'TEXT' && (
                            <FieldRow
                                id="tpl-header"
                                label={t('templates.builder.headerText')}
                                error={err('header')}
                                counter={
                                    <Counter
                                        value={d.header}
                                        max={LIMITS.header}
                                    />
                                }
                                hint={t('templates.builder.headerTextHint')}
                            >
                                <Input
                                    {...aria('header')}
                                    value={d.header}
                                    onChange={(e) =>
                                        set('header', e.target.value)
                                    }
                                />
                            </FieldRow>
                        )}
                        {(d.headerKind === 'IMAGE' ||
                            d.headerKind === 'VIDEO' ||
                            d.headerKind === 'DOCUMENT') && (
                            <FieldRow
                                id="tpl-header"
                                label={t('templates.builder.sample')}
                                error={err('header')}
                                hint={t('templates.create.sampleHint')}
                            >
                                <FileDrop
                                    compact
                                    accept={SAMPLE_ACCEPT[d.headerKind]}
                                    disabled={uploading === 'header'}
                                    onFile={(f) => void sample(f, 'header')}
                                    hint={
                                        uploading === 'header' ? (
                                            <>
                                                <Spinner />{' '}
                                                {t('params.media.uploading')}
                                            </>
                                        ) : d.headerFile ? (
                                            t(
                                                'templates.create.sampleUploaded',
                                                {
                                                    name: d.headerFile,
                                                },
                                            )
                                        ) : (
                                            t('templates.create.sampleHint')
                                        )
                                    }
                                />
                            </FieldRow>
                        )}
                        {d.headerKind === 'LOCATION' && (
                            <p className="text-sm text-muted-foreground">
                                {t('templates.builder.locationHint')}
                            </p>
                        )}
                        {headerVars.map((v) => (
                            <ExampleInput
                                key={`h:${v}`}
                                id={`example-h:${v}`}
                                label={t('templates.create.exampleHeader', {
                                    variable: `{{${v}}}`,
                                })}
                                value={d.examples[`h:${v}`] ?? ''}
                                error={err(`example-h:${v}`)}
                                onChange={(x) =>
                                    set('examples', {
                                        ...d.examples,
                                        [`h:${v}`]: x,
                                    })
                                }
                            />
                        ))}
                    </Section>
                )}

                {d.kind !== 'authentication' && (
                    <Section
                        n={++n}
                        title={t('templates.builder.body')}
                        hint={
                            named
                                ? t('templates.create.bodyHintNamed')
                                : t('templates.create.bodyHintPositional')
                        }
                        testId="section-body"
                    >
                        {d.kind === 'lto' && (
                            <div className="grid gap-4 rounded-lg border p-3 sm:grid-cols-[1fr_auto] sm:items-end">
                                <FieldRow
                                    id="tpl-lto-text"
                                    label={t('templates.builder.ltoText')}
                                    error={err('lto-text')}
                                    counter={
                                        <Counter
                                            value={d.ltoText}
                                            max={LIMITS.ltoText}
                                        />
                                    }
                                    hint={t('templates.builder.ltoTextHint')}
                                >
                                    <Input
                                        {...aria('lto-text')}
                                        value={d.ltoText}
                                        placeholder={t(
                                            'templates.builder.ltoTextPlaceholder',
                                        )}
                                        onChange={(e) =>
                                            set('ltoText', e.target.value)
                                        }
                                    />
                                </FieldRow>
                                <label className="flex items-center gap-2 pb-2 text-sm">
                                    <Switch
                                        checked={d.ltoExpiration}
                                        onCheckedChange={(v) =>
                                            set('ltoExpiration', v)
                                        }
                                        aria-label={t(
                                            'templates.builder.ltoExpiration',
                                        )}
                                    />
                                    {t('templates.builder.ltoExpiration')}
                                </label>
                            </div>
                        )}
                        <FieldRow
                            id="tpl-body"
                            label={t('templates.create.body')}
                            error={err('body')}
                            counter={<Counter value={d.body} max={bodyMax} />}
                        >
                            <Textarea
                                {...aria('body')}
                                ref={bodyRef}
                                rows={5}
                                value={d.body}
                                placeholder={
                                    named
                                        ? t(
                                              'templates.create.bodyPlaceholderNamed',
                                          )
                                        : t(
                                              'templates.create.bodyPlaceholderPositional',
                                          )
                                }
                                onChange={(e) => set('body', e.target.value)}
                            />
                        </FieldRow>
                        <div className="flex flex-wrap items-center gap-2">
                            <Button
                                type="button"
                                size="sm"
                                variant="outline"
                                onClick={insertVariable}
                            >
                                <Braces /> {t('templates.builder.addVariable')}
                            </Button>
                            {bodyVars.map((v) => (
                                <Badge
                                    key={v}
                                    variant="secondary"
                                    className="font-mono"
                                >
                                    {`{{${v}}}`}
                                </Badge>
                            ))}
                        </div>
                        {bodyVars.length > 0 && (
                            <div className="space-y-3 rounded-lg border bg-muted/20 p-3">
                                <p className="text-sm font-medium">
                                    {t('templates.create.examples')}
                                    <span className="block text-xs font-normal text-muted-foreground">
                                        {t('templates.create.examplesHint')}
                                    </span>
                                </p>
                                {bodyVars.map((v) => (
                                    <ExampleInput
                                        key={`b:${v}`}
                                        id={`example-b:${v}`}
                                        label={t(
                                            'templates.create.exampleBody',
                                            { variable: `{{${v}}}` },
                                        )}
                                        value={d.examples[`b:${v}`] ?? ''}
                                        error={err(`example-b:${v}`)}
                                        onChange={(x) =>
                                            set('examples', {
                                                ...d.examples,
                                                [`b:${v}`]: x,
                                            })
                                        }
                                    />
                                ))}
                            </div>
                        )}
                    </Section>
                )}

                {d.kind === 'standard' && (
                    <Section
                        n={++n}
                        title={t('templates.builder.footer')}
                        hint={t('templates.builder.footerHint')}
                    >
                        <FieldRow
                            id="tpl-footer"
                            label={t('templates.create.footer')}
                            error={err('footer')}
                            counter={
                                <Counter value={d.footer} max={LIMITS.footer} />
                            }
                        >
                            <Input
                                {...aria('footer')}
                                value={d.footer}
                                placeholder={t(
                                    'templates.create.footerPlaceholder',
                                )}
                                onChange={(e) => set('footer', e.target.value)}
                            />
                        </FieldRow>
                    </Section>
                )}

                {d.kind === 'standard' && (
                    <Section
                        n={++n}
                        title={t('templates.builder.buttons')}
                        hint={t('templates.builder.buttonsHint', {
                            count: d.buttons.length,
                            max: LIMITS.buttons,
                        })}
                        testId="section-buttons"
                    >
                        {err('buttons') && (
                            <p
                                role="alert"
                                className="text-sm text-destructive"
                            >
                                {err('buttons')}
                            </p>
                        )}
                        <ul className="space-y-3">
                            {d.buttons.map((b, i) => (
                                <ButtonEditor
                                    key={b.key}
                                    index={i}
                                    button={b}
                                    errors={errors}
                                    onChange={(patch) =>
                                        editButton(b.key, patch)
                                    }
                                    onRemove={() =>
                                        set(
                                            'buttons',
                                            d.buttons.filter(
                                                (x) => x.key !== b.key,
                                            ),
                                        )
                                    }
                                />
                            ))}
                        </ul>
                        <div
                            className="flex flex-wrap gap-2"
                            role="group"
                            aria-label={t('templates.create.addButton')}
                        >
                            {allowedButtons(d).map((type) => {
                                const Icon = BUTTON_ICON[type];
                                const used = counts[type] ?? 0;
                                const full =
                                    used >= BUTTON_LIMITS[type] ||
                                    d.buttons.length >= LIMITS.buttons;
                                return (
                                    <Button
                                        key={type}
                                        type="button"
                                        size="sm"
                                        variant="outline"
                                        disabled={full}
                                        onClick={() => addButton(type)}
                                        data-testid={`add-button-${type}`}
                                    >
                                        <Plus />
                                        <Icon />
                                        {t(
                                            `templates.builder.buttonTypes.${type}`,
                                        )}
                                        <span className="text-xs text-muted-foreground tabular-nums">
                                            {used}/{BUTTON_LIMITS[type]}
                                        </span>
                                    </Button>
                                );
                            })}
                        </div>
                        <p className="text-xs text-muted-foreground">
                            {t('templates.builder.quickReplyGroup')}
                        </p>
                    </Section>
                )}

                {d.kind === 'lto' && (
                    <Section
                        n={++n}
                        title={t('templates.builder.ltoButtons')}
                        hint={t('templates.builder.ltoButtonsHint')}
                        testId="section-lto"
                    >
                        <div className="grid gap-4 sm:grid-cols-2">
                            <FieldRow
                                id="tpl-lto-coupon"
                                label={t('templates.builder.couponExample')}
                                error={err('lto-coupon')}
                                counter={
                                    <Counter
                                        value={d.ltoCoupon}
                                        max={LIMITS.ltoCoupon}
                                    />
                                }
                                hint={t('templates.builder.couponHint')}
                            >
                                <Input
                                    {...aria('lto-coupon')}
                                    value={d.ltoCoupon}
                                    className="font-mono"
                                    onChange={(e) =>
                                        set('ltoCoupon', e.target.value)
                                    }
                                />
                            </FieldRow>
                            <FieldRow
                                id="tpl-lto-url-text"
                                label={t('templates.builder.urlText')}
                                error={err('lto-url-text')}
                                counter={
                                    <Counter
                                        value={d.ltoUrl.text}
                                        max={LIMITS.buttonText}
                                    />
                                }
                            >
                                <Input
                                    {...aria('lto-url-text')}
                                    value={d.ltoUrl.text}
                                    onChange={(e) =>
                                        set('ltoUrl', {
                                            ...d.ltoUrl,
                                            text: e.target.value,
                                        })
                                    }
                                />
                            </FieldRow>
                            <FieldRow
                                id="tpl-lto-url"
                                label={t('templates.builder.url')}
                                error={err('lto-url')}
                                hint={t('templates.builder.urlHint')}
                            >
                                <Input
                                    {...aria('lto-url')}
                                    value={d.ltoUrl.value}
                                    inputMode="url"
                                    placeholder="https://shop.example/offer?code={{1}}"
                                    onChange={(e) =>
                                        set('ltoUrl', {
                                            ...d.ltoUrl,
                                            value: e.target.value,
                                        })
                                    }
                                />
                            </FieldRow>
                            {variablesOf(d.ltoUrl.value).length > 0 && (
                                <FieldRow
                                    id="tpl-lto-url-example"
                                    label={t('templates.builder.urlExample')}
                                    error={err('lto-url-example')}
                                >
                                    <Input
                                        {...aria('lto-url-example')}
                                        value={d.ltoUrl.example}
                                        inputMode="url"
                                        onChange={(e) =>
                                            set('ltoUrl', {
                                                ...d.ltoUrl,
                                                example: e.target.value,
                                            })
                                        }
                                    />
                                </FieldRow>
                            )}
                        </div>
                        <Alert>
                            <Clock />
                            <AlertDescription>
                                {t('templates.builder.ltoNote')}
                            </AlertDescription>
                        </Alert>
                    </Section>
                )}

                {d.kind === 'carousel' && (
                    <CarouselSection
                        n={++n}
                        d={d}
                        setD={setD}
                        errors={errors}
                        uploading={uploading}
                        onSample={(f, k) => void sample(f, k)}
                        nextKey={nextKey}
                        named={named}
                    />
                )}

                {d.kind === 'authentication' && (
                    <AuthSection n={++n} d={d} setD={setD} errors={errors} />
                )}

                <div className="flex flex-wrap items-center gap-3">
                    <Button
                        type="submit"
                        disabled={pending}
                        data-testid="template-submit"
                    >
                        {pending ? <Spinner /> : <Send />}{' '}
                        {t('templates.create.submit')}
                    </Button>
                    <Button type="button" variant="ghost" asChild>
                        <Link href="/templates">{t('common.cancel')}</Link>
                    </Button>
                    <p className="text-sm text-muted-foreground">
                        {t('templates.create.description')}
                    </p>
                </div>
            </div>

            <aside className="lg:sticky lg:top-4 lg:self-start">
                <h2 className="mb-2 text-sm font-medium">
                    {t('templates.preview.title')}
                </h2>
                <WhatsAppPreview
                    components={components}
                    literal={previewExamples}
                    mediaSrc={d.headerPreview}
                    mediaName={d.headerFile}
                    category={d.category}
                    expiresAt={d.ltoExpiration ? sampleEnd : undefined}
                    coupon={d.kind === 'lto' ? d.ltoCoupon : undefined}
                    cardMedia={d.cards.map((c) => c.preview)}
                />
                <p className="mt-2 text-xs text-muted-foreground">
                    {d.kind === 'lto'
                        ? t('templates.builder.ltoPreviewNote')
                        : t('templates.builder.previewNote')}
                </p>
            </aside>
        </form>
    );
}

function ExampleInput({
    id,
    label,
    value,
    error,
    onChange,
}: {
    id: string;
    label: string;
    value: string;
    error?: string;
    onChange: (v: string) => void;
}) {
    return (
        <div className="grid items-start gap-1.5 sm:grid-cols-[12rem_1fr]">
            <label htmlFor={`tpl-${id}`} className="pt-2 text-sm">
                {label}
            </label>
            <div>
                <Input
                    id={`tpl-${id}`}
                    value={value}
                    aria-invalid={error ? true : undefined}
                    onChange={(e) => onChange(e.target.value)}
                />
                {error && (
                    <p role="alert" className="mt-1 text-sm text-destructive">
                        {error}
                    </p>
                )}
            </div>
        </div>
    );
}

function ButtonEditor({
    index,
    button: b,
    errors,
    onChange,
    onRemove,
}: {
    index: number;
    button: Draft['buttons'][0];
    errors: Record<string, string>;
    onChange: (patch: Partial<Draft['buttons'][0]>) => void;
    onRemove: () => void;
}) {
    const t = useT();
    const Icon = BUTTON_ICON[b.type];
    const f = (k: string) => `button-${index}-${k}`;
    const labelMax =
        b.type === 'VOICE_CALL' ? LIMITS.voiceText : LIMITS.buttonText;
    return (
        <li
            className="space-y-3 rounded-lg border p-3"
            data-testid={`button-${index}`}
        >
            <div className="flex items-center justify-between gap-2">
                <p className="flex items-center gap-2 text-sm font-medium">
                    <Icon className="size-4 text-muted-foreground" />
                    {t('templates.builder.buttonN', {
                        n: index + 1,
                        type: t(`templates.builder.buttonTypes.${b.type}`),
                    })}
                </p>
                <Button
                    type="button"
                    size="icon-sm"
                    variant="ghost"
                    aria-label={t('templates.create.removeButton', {
                        n: index + 1,
                    })}
                    onClick={onRemove}
                >
                    <Trash2 />
                </Button>
            </div>
            <div className="grid gap-3 sm:grid-cols-2">
                {b.type !== 'COPY_CODE' && (
                    <FieldRow
                        id={`tpl-${f('text')}`}
                        label={t('templates.create.buttonText', {
                            n: index + 1,
                        })}
                        error={errors[f('text')]}
                        counter={<Counter value={b.text} max={labelMax} />}
                    >
                        <Input
                            id={`tpl-${f('text')}`}
                            value={b.text}
                            aria-invalid={errors[f('text')] ? true : undefined}
                            placeholder={
                                b.type === 'VOICE_CALL'
                                    ? t('templates.preview.callNow')
                                    : t(
                                          'templates.create.buttonTextPlaceholder',
                                      )
                            }
                            onChange={(e) => onChange({ text: e.target.value })}
                        />
                    </FieldRow>
                )}
                {(b.type === 'URL' ||
                    b.type === 'PHONE_NUMBER' ||
                    b.type === 'COPY_CODE' ||
                    b.type === 'FLOW') && (
                    <FieldRow
                        id={`tpl-${f('value')}`}
                        label={t(`templates.builder.valueLabels.${b.type}`)}
                        error={errors[f('value')]}
                        hint={
                            b.type === 'URL'
                                ? t('templates.builder.urlHint')
                                : b.type === 'FLOW'
                                  ? t('templates.builder.flowHint')
                                  : undefined
                        }
                    >
                        <Input
                            id={`tpl-${f('value')}`}
                            value={b.value}
                            aria-invalid={errors[f('value')] ? true : undefined}
                            inputMode={b.type === 'URL' ? 'url' : undefined}
                            className={cn(
                                b.type === 'COPY_CODE' && 'font-mono',
                            )}
                            onChange={(e) =>
                                onChange({ value: e.target.value })
                            }
                        />
                    </FieldRow>
                )}
                {b.type === 'URL' && variablesOf(b.value).length > 0 && (
                    <FieldRow
                        id={`tpl-${f('example')}`}
                        label={t('templates.create.buttonExample', {
                            n: index + 1,
                        })}
                        error={errors[f('example')]}
                    >
                        <Input
                            id={`tpl-${f('example')}`}
                            value={b.example}
                            onChange={(e) =>
                                onChange({ example: e.target.value })
                            }
                        />
                    </FieldRow>
                )}
                {b.type === 'FLOW' && (
                    <FieldRow
                        id={`tpl-${f('action')}`}
                        label={t('templates.builder.flowAction')}
                    >
                        <Segmented
                            aria-label={t('templates.builder.flowAction')}
                            value={b.flowAction}
                            onValueChange={(v) => onChange({ flowAction: v })}
                            options={[
                                {
                                    value: 'navigate',
                                    label: t(
                                        'templates.builder.flowActions.navigate',
                                    ),
                                },
                                {
                                    value: 'data_exchange',
                                    label: t(
                                        'templates.builder.flowActions.data_exchange',
                                    ),
                                },
                            ]}
                        />
                    </FieldRow>
                )}
            </div>
        </li>
    );
}

function CarouselSection({
    n,
    d,
    setD,
    errors,
    uploading,
    onSample,
    nextKey,
    named,
}: {
    n: number;
    d: Draft;
    setD: React.Dispatch<React.SetStateAction<Draft>>;
    errors: Record<string, string>;
    uploading: string | null;
    onSample: (f: File | null, card: number) => void;
    nextKey: () => number;
    named: boolean;
}) {
    const t = useT();
    const editCard = (k: number, patch: Partial<Draft['cards'][0]>) =>
        setD((p) => ({
            ...p,
            cards: p.cards.map((c, i) => (i === k ? { ...c, ...patch } : c)),
        }));
    const setStructure = (types: CardButtonType[]) =>
        setD((p) => ({
            ...p,
            cardButtons: types,
            cards: p.cards.map((c) => ({
                ...c,
                buttons: types.map(
                    (_, j) =>
                        c.buttons[j] ?? { text: '', value: '', example: '' },
                ),
            })),
        }));
    return (
        <Section
            n={n}
            title={t('templates.builder.cards')}
            hint={t('templates.builder.cardsHint', {
                count: d.cards.length,
                max: LIMITS.cards,
            })}
            testId="section-cards"
        >
            <div className="grid gap-4 rounded-lg border bg-muted/20 p-3 sm:grid-cols-2">
                <FieldRow
                    id="tpl-card-header"
                    label={t('templates.builder.cardHeader')}
                >
                    <Segmented
                        aria-label={t('templates.builder.cardHeader')}
                        value={d.cardHeader}
                        onValueChange={(v) =>
                            setD((p) => ({
                                ...p,
                                cardHeader: v,
                                cards: p.cards.map((c) => ({
                                    ...c,
                                    handle: '',
                                    file: '',
                                    preview: undefined,
                                })),
                            }))
                        }
                        options={[
                            {
                                value: 'IMAGE',
                                label: t('templates.builder.headerKinds.IMAGE'),
                            },
                            {
                                value: 'VIDEO',
                                label: t('templates.builder.headerKinds.VIDEO'),
                            },
                        ]}
                    />
                </FieldRow>
                <label className="flex items-center gap-2 self-end pb-1 text-sm">
                    <Checkbox
                        checked={d.cardBody}
                        onCheckedChange={(v) =>
                            setD((p) => ({ ...p, cardBody: v === true }))
                        }
                    />
                    {t('templates.builder.cardBody')}
                </label>
                <div className="space-y-2 sm:col-span-2">
                    <p className="text-sm font-medium">
                        {t('templates.builder.cardButtons')}
                    </p>
                    <div className="flex flex-wrap items-center gap-2">
                        {d.cardButtons.map((type, j) => (
                            <Badge
                                key={j}
                                variant="secondary"
                                className="gap-1"
                            >
                                {t(`templates.builder.buttonTypes.${type}`)}
                                <button
                                    type="button"
                                    className="ml-1 rounded hover:text-destructive"
                                    aria-label={t(
                                        'templates.create.removeButton',
                                        { n: j + 1 },
                                    )}
                                    onClick={() =>
                                        setStructure(
                                            d.cardButtons.filter(
                                                (_, i) => i !== j,
                                            ),
                                        )
                                    }
                                >
                                    ×
                                </button>
                            </Badge>
                        ))}
                        {(['QUICK_REPLY', 'URL', 'PHONE_NUMBER'] as const).map(
                            (type) => (
                                <Button
                                    key={type}
                                    type="button"
                                    size="sm"
                                    variant="outline"
                                    disabled={
                                        d.cardButtons.length >=
                                            LIMITS.cardButtons ||
                                        (type === 'PHONE_NUMBER' &&
                                            d.cardButtons.includes(type))
                                    }
                                    onClick={() =>
                                        setStructure([...d.cardButtons, type])
                                    }
                                >
                                    <Plus />
                                    {t(`templates.builder.buttonTypes.${type}`)}
                                </Button>
                            ),
                        )}
                        <span className="text-xs text-muted-foreground tabular-nums">
                            {d.cardButtons.length}/{LIMITS.cardButtons}
                        </span>
                    </div>
                    <p className="text-xs text-muted-foreground">
                        {t('templates.builder.identical')}
                    </p>
                </div>
            </div>
            {errors.cards && (
                <p role="alert" className="text-sm text-destructive">
                    {errors.cards}
                </p>
            )}
            <ol className="space-y-3">
                {d.cards.map((c, k) => {
                    const vars = variablesOf(c.body);
                    const f = (x: string) => `card-${k}-${x}`;
                    return (
                        <li
                            key={c.key}
                            className="space-y-3 rounded-lg border p-3"
                            data-testid={`card-${k}`}
                        >
                            <div className="flex items-center justify-between">
                                <p className="text-sm font-medium">
                                    {t('params.card', { n: k + 1 })}
                                </p>
                                <Button
                                    type="button"
                                    size="icon-sm"
                                    variant="ghost"
                                    disabled={d.cards.length <= LIMITS.minCards}
                                    aria-label={t(
                                        'templates.builder.removeCard',
                                        { n: k + 1 },
                                    )}
                                    onClick={() =>
                                        setD((p) => ({
                                            ...p,
                                            cards: p.cards.filter(
                                                (_, i) => i !== k,
                                            ),
                                        }))
                                    }
                                >
                                    <Trash2 />
                                </Button>
                            </div>
                            <FieldRow
                                id={`tpl-${f('media')}`}
                                label={t('templates.builder.sample')}
                                error={errors[f('media')]}
                            >
                                <FileDrop
                                    compact
                                    accept={SAMPLE_ACCEPT[d.cardHeader]}
                                    disabled={uploading === f('media')}
                                    onFile={(file) => onSample(file, k)}
                                    hint={
                                        uploading === f('media') ? (
                                            <>
                                                <Spinner />{' '}
                                                {t('params.media.uploading')}
                                            </>
                                        ) : c.file ? (
                                            t(
                                                'templates.create.sampleUploaded',
                                                {
                                                    name: c.file,
                                                },
                                            )
                                        ) : (
                                            t('templates.create.sampleHint')
                                        )
                                    }
                                />
                            </FieldRow>
                            {d.cardBody && (
                                <FieldRow
                                    id={`tpl-${f('body')}`}
                                    label={t('templates.builder.cardBodyText')}
                                    error={errors[f('body')]}
                                    counter={
                                        <Counter
                                            value={c.body}
                                            max={LIMITS.cardBody}
                                        />
                                    }
                                    hint={
                                        named
                                            ? t(
                                                  'templates.create.bodyHintNamed',
                                              )
                                            : t(
                                                  'templates.create.bodyHintPositional',
                                              )
                                    }
                                >
                                    <Textarea
                                        id={`tpl-${f('body')}`}
                                        rows={2}
                                        value={c.body}
                                        onChange={(e) =>
                                            editCard(k, {
                                                body: e.target.value,
                                            })
                                        }
                                    />
                                </FieldRow>
                            )}
                            {d.cardBody &&
                                vars.map((v) => (
                                    <ExampleInput
                                        key={v}
                                        id={`${f('example')}-${v}`}
                                        label={t(
                                            'templates.create.exampleBody',
                                            {
                                                variable: `{{${v}}}`,
                                            },
                                        )}
                                        value={c.examples[v] ?? ''}
                                        error={errors[`${f('example')}-${v}`]}
                                        onChange={(x) =>
                                            editCard(k, {
                                                examples: {
                                                    ...c.examples,
                                                    [v]: x,
                                                },
                                            })
                                        }
                                    />
                                ))}
                            {d.cardButtons.map((type, j) => {
                                const b = c.buttons[j] ?? {
                                    text: '',
                                    value: '',
                                    example: '',
                                };
                                const setB = (patch: Partial<typeof b>) =>
                                    editCard(k, {
                                        buttons: d.cardButtons.map((_, i) =>
                                            i === j
                                                ? { ...b, ...patch }
                                                : (c.buttons[i] ?? {
                                                      text: '',
                                                      value: '',
                                                      example: '',
                                                  }),
                                        ),
                                    });
                                const bf = (x: string) =>
                                    `${f('button')}-${j}-${x}`;
                                return (
                                    <div
                                        key={j}
                                        className="grid gap-3 sm:grid-cols-2"
                                    >
                                        <FieldRow
                                            id={`tpl-${bf('text')}`}
                                            label={t(
                                                'templates.create.buttonText',
                                                { n: j + 1 },
                                            )}
                                            error={errors[bf('text')]}
                                            counter={
                                                <Counter
                                                    value={b.text}
                                                    max={LIMITS.buttonText}
                                                />
                                            }
                                        >
                                            <Input
                                                id={`tpl-${bf('text')}`}
                                                value={b.text}
                                                onChange={(e) =>
                                                    setB({
                                                        text: e.target.value,
                                                    })
                                                }
                                            />
                                        </FieldRow>
                                        {type !== 'QUICK_REPLY' && (
                                            <FieldRow
                                                id={`tpl-${bf('value')}`}
                                                label={t(
                                                    `templates.builder.valueLabels.${type}`,
                                                )}
                                                error={errors[bf('value')]}
                                            >
                                                <Input
                                                    id={`tpl-${bf('value')}`}
                                                    value={b.value}
                                                    onChange={(e) =>
                                                        setB({
                                                            value: e.target
                                                                .value,
                                                        })
                                                    }
                                                />
                                            </FieldRow>
                                        )}
                                        {type === 'URL' &&
                                            variablesOf(b.value).length > 0 && (
                                                <FieldRow
                                                    id={`tpl-${bf('example')}`}
                                                    label={t(
                                                        'templates.builder.urlSuffixExample',
                                                    )}
                                                    error={
                                                        errors[bf('example')]
                                                    }
                                                >
                                                    <Input
                                                        id={`tpl-${bf('example')}`}
                                                        value={b.example}
                                                        onChange={(e) =>
                                                            setB({
                                                                example:
                                                                    e.target
                                                                        .value,
                                                            })
                                                        }
                                                    />
                                                </FieldRow>
                                            )}
                                    </div>
                                );
                            })}
                        </li>
                    );
                })}
            </ol>
            <Button
                type="button"
                variant="outline"
                disabled={d.cards.length >= LIMITS.cards}
                onClick={() =>
                    setD((p) => ({
                        ...p,
                        cards: [
                            ...p.cards,
                            emptyCard(nextKey(), p.cardButtons),
                        ],
                    }))
                }
                data-testid="add-card"
            >
                <Plus /> {t('templates.builder.addCard')}
            </Button>
        </Section>
    );
}

function AuthSection({
    n,
    d,
    setD,
    errors,
}: {
    n: number;
    d: Draft;
    setD: React.Dispatch<React.SetStateAction<Draft>>;
    errors: Record<string, string>;
}) {
    const t = useT();
    const a = d.auth;
    const set = (patch: Partial<Draft['auth']>) =>
        setD((p) => ({ ...p, auth: { ...p.auth, ...patch } }));
    const app = a.otpType !== 'COPY_CODE';
    return (
        <Section
            n={n}
            title={t('templates.builder.otp')}
            hint={t('templates.builder.otpHint')}
            testId="section-otp"
        >
            <Segmented
                aria-label={t('templates.builder.otpType')}
                value={a.otpType}
                onValueChange={(v) => set({ otpType: v })}
                options={(['COPY_CODE', 'ONE_TAP', 'ZERO_TAP'] as const).map(
                    (k) => ({
                        value: k,
                        label: t(`templates.builder.otpTypes.${k}`),
                    }),
                )}
            />
            <p className="text-xs text-muted-foreground">
                {t(`templates.builder.otpTypeHints.${a.otpType}`)}
            </p>
            <label className="flex items-center gap-2 text-sm">
                <Switch
                    checked={a.security}
                    onCheckedChange={(v) => set({ security: v })}
                    aria-label={t('templates.builder.otpSecurity')}
                />
                {t('templates.builder.otpSecurity')}
            </label>
            <div className="grid gap-4 sm:grid-cols-2">
                <FieldRow
                    id="tpl-otp-expiry"
                    label={t('templates.builder.otpExpiry')}
                    error={errors['otp-expiry']}
                    hint={t('templates.builder.otpExpiryHint')}
                >
                    <Input
                        id="tpl-otp-expiry"
                        inputMode="numeric"
                        value={a.expiry}
                        placeholder="10"
                        onChange={(e) => set({ expiry: e.target.value })}
                    />
                </FieldRow>
                <FieldRow
                    id="tpl-otp-text"
                    label={t('templates.builder.otpButtonText')}
                    error={errors['otp-text']}
                    counter={<Counter value={a.text} max={LIMITS.buttonText} />}
                >
                    <Input
                        id="tpl-otp-text"
                        value={a.text}
                        placeholder={t('templates.preview.copyCode')}
                        onChange={(e) => set({ text: e.target.value })}
                    />
                </FieldRow>
                {app && (
                    <>
                        <FieldRow
                            id="tpl-otp-autofill"
                            label={t('templates.builder.otpAutofill')}
                            error={errors['otp-autofill']}
                            counter={
                                <Counter
                                    value={a.autofill}
                                    max={LIMITS.buttonText}
                                />
                            }
                        >
                            <Input
                                id="tpl-otp-autofill"
                                value={a.autofill}
                                placeholder={t('templates.preview.autofill')}
                                onChange={(e) =>
                                    set({ autofill: e.target.value })
                                }
                            />
                        </FieldRow>
                        <FieldRow
                            id="tpl-otp-package"
                            label={t('templates.builder.otpPackage')}
                            error={errors['otp-package']}
                        >
                            <Input
                                id="tpl-otp-package"
                                value={a.packageName}
                                placeholder="com.example.app"
                                className="font-mono"
                                onChange={(e) =>
                                    set({ packageName: e.target.value })
                                }
                            />
                        </FieldRow>
                        <FieldRow
                            id="tpl-otp-hash"
                            label={t('templates.builder.otpHash')}
                            error={errors['otp-hash']}
                            hint={t('templates.builder.otpHashHint')}
                        >
                            <Input
                                id="tpl-otp-hash"
                                value={a.signatureHash}
                                maxLength={11}
                                className="font-mono"
                                onChange={(e) =>
                                    set({ signatureHash: e.target.value })
                                }
                            />
                        </FieldRow>
                    </>
                )}
            </div>
            {a.otpType === 'ZERO_TAP' && (
                <label className="flex items-start gap-2 text-sm">
                    <Checkbox
                        id="tpl-otp-terms"
                        checked={a.zeroTapTerms}
                        onCheckedChange={(v) =>
                            set({ zeroTapTerms: v === true })
                        }
                        aria-invalid={errors['otp-terms'] ? true : undefined}
                    />
                    <span>
                        {t('templates.builder.otpTerms')}
                        {errors['otp-terms'] && (
                            <span className="block text-destructive">
                                {errors['otp-terms']}
                            </span>
                        )}
                    </span>
                </label>
            )}
        </Section>
    );
}
