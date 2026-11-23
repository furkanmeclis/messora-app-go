'use client';

import { useEffect, useMemo, useRef, useState, useTransition } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { AlertCircle, Check, Info, Users } from 'lucide-react';
import { toast } from 'sonner';
import { newIdempotencyKey } from '@/components/notify';
import { useReauth } from '@/components/reauth';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import {
    AlertDialog,
    AlertDialogAction,
    AlertDialogCancel,
    AlertDialogContent,
    AlertDialogDescription,
    AlertDialogFooter,
    AlertDialogHeader,
    AlertDialogTitle,
} from '@/components/ui/alert-dialog';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
    Card,
    CardContent,
    CardDescription,
    CardHeader,
    CardTitle,
} from '@/components/ui/card';
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
import { slotsOf, templateKey } from '@/lib/templates';
import { formatInZone, utcToZoned, zonedToUtc } from '@/lib/timezone';
import type { ActionResult, Campaign, Template } from '@/lib/types';
import { TemplatePreview } from '../templates/template-preview';
import {
    audienceEstimateAction,
    saveCampaignAction,
    startCampaignAction,
    type CampaignInput,
} from './actions';
import { audienceLabel } from './campaign-ui';
import type { WizardData } from './wizard-data';

type Failure = Extract<ActionResult<unknown>, { ok: false }>;
type AudienceKind = 'segment' | 'group' | 'tag' | 'all';

type State = {
    name: string;
    /** '' = the default number. */
    phoneNumberId: string;
    template: string;
    header: Record<string, string>;
    body: Record<string, string>;
    /** Button position → value for dynamic URL buttons. */
    buttons: Record<string, string>;
    fallback: Record<string, string>;
    audienceKind: AudienceKind;
    audienceId: string;
    windowSize: string;
    when: 'now' | 'schedule';
    scheduledAt: string;
};

const STEPS = ['Basics', 'Template', 'Variables', 'Audience', 'Review'];

const PLACEHOLDERS = [
    'name',
    'first_name',
    'phone',
    'email',
    'var1',
    'var2',
    'var3',
    'var4',
    'var5',
];
const PLACEHOLDER_RE = /\{\{\s*([a-z0-9_]+(?::[A-Za-z0-9_ .-]+)?)\s*\}\}/g;

function placeholdersIn(values: string[]): string[] {
    const out: string[] = [];
    for (const v of values) {
        for (const m of v.matchAll(PLACEHOLDER_RE)) {
            if (!out.includes(m[1]!)) out.push(m[1]!);
        }
    }
    return out;
}

/** At least a minute ahead (the API starts anything sooner right away). */
function isFuture(at: Date): boolean {
    return at.getTime() > Date.now() + 60_000;
}

function toRecord(
    v: string[] | Record<string, string> | undefined,
): Record<string, string> {
    if (!v) return {};
    if (Array.isArray(v))
        return Object.fromEntries(v.map((x, i) => [String(i + 1), x]));
    return { ...v };
}

function initialState(c: Campaign | undefined, timezone: string): State {
    if (!c) {
        return {
            name: '',
            phoneNumberId: '',
            template: '',
            header: {},
            body: {},
            buttons: {},
            fallback: {},
            audienceKind: 'segment',
            audienceId: '',
            windowSize: '',
            when: 'now',
            scheduledAt: '',
        };
    }
    const a = c.audience;
    const kind: AudienceKind = a.all
        ? 'all'
        : a.group_id
          ? 'group'
          : a.tag_id
            ? 'tag'
            : 'segment';
    const p = c.template_params ?? {};
    return {
        name: c.name,
        phoneNumberId: c.phone_number_id,
        template: templateKey({
            name: c.template_name,
            language: c.template_language,
        }),
        header: toRecord(p.header),
        body: toRecord(p.body),
        buttons: Object.fromEntries(
            (p.buttons ?? []).map((v, i) => [String(i), v]),
        ),
        fallback: { ...(p.fallback ?? {}) },
        audienceKind: kind,
        audienceId: a.segment_id ?? a.group_id ?? a.tag_id ?? '',
        windowSize: c.window_size ? String(c.window_size) : '',
        when: c.scheduled_at ? 'schedule' : 'now',
        scheduledAt: c.scheduled_at ? utcToZoned(c.scheduled_at, timezone) : '',
    };
}

/** Which wizard step an API field error belongs to. */
function stepOf(field: string): number {
    if (field === 'name' || field === 'phone_number_id') return 0;
    if (field.startsWith('template_params')) return 2;
    if (field.startsWith('template')) return 1;
    if (field.startsWith('audience')) return 3;
    return 4;
}

export function CampaignWizard({
    data,
    campaign,
}: {
    data: WizardData;
    campaign?: Campaign;
}) {
    const router = useRouter();
    const { guard } = useReauth();
    const [s, setS] = useState<State>(() =>
        initialState(campaign, data.timezone),
    );
    const [step, setStep] = useState(0);
    const [savedId, setSavedId] = useState<string | null>(campaign?.id ?? null);
    const [errors, setErrors] = useState<Record<string, string>>({});
    const [failure, setFailure] = useState<Failure | null>(null);
    const [idem, setIdem] = useState(newIdempotencyKey);
    const [pending, startTransition] = useTransition();
    const [confirmOpen, setConfirmOpen] = useState(false);
    const [estimates, setEstimates] = useState<Record<string, number | null>>(
        {},
    );
    const headingRef = useRef<HTMLHeadingElement>(null);

    const set = <K extends keyof State>(k: K, v: State[K]) =>
        setS((p) => ({ ...p, [k]: v }));

    // ── Derived data ────────────────────────────────────────────────────
    const numbers = data.numbers;
    const defaultNumber = numbers?.find((n) => n.is_default);
    const sender = s.phoneNumberId
        ? numbers?.find((n) => n.id === s.phoneNumberId)
        : defaultNumber;
    const templates = useMemo(
        () =>
            sender
                ? data.templates.filter(
                      (t) => t.account_id === sender.account_id,
                  )
                : data.templates,
        [data.templates, sender],
    );
    const template: Template | undefined = templates.find(
        (t) => templateKey(t) === s.template,
    );
    const slots = template ? slotsOf(template) : null;
    const buttonGap =
        !!slots &&
        slots.urlButtons.length > 0 &&
        slots.urlButtons.some((pos, i) => pos !== i);

    const usedValues = slots
        ? [
              ...slots.header.map((v) => s.header[v] ?? ''),
              ...slots.body.map((v) => s.body[v] ?? ''),
              ...slots.urlButtons.map((p) => s.buttons[String(p)] ?? ''),
          ]
        : [];
    const usedPlaceholders = placeholdersIn(usedValues);

    const names: Record<string, string> = {};
    for (const x of data.segments ?? []) names[x.id] = x.name;
    for (const x of data.groups ?? []) names[x.id] = x.name;
    for (const x of data.tags ?? []) names[x.id] = x.name;

    const audienceKey =
        s.audienceKind === 'all' ? 'all' : `${s.audienceKind}:${s.audienceId}`;
    const estimate: number | null | undefined = (() => {
        if (s.audienceKind === 'group')
            return data.groups?.find((g) => g.id === s.audienceId)
                ?.member_count;
        if (s.audienceKind === 'tag')
            return data.tags?.find((t) => t.id === s.audienceId)?.contact_count;
        return estimates[audienceKey];
    })();

    // Segment sizes and "all contacts" are counted by the API on demand.
    useEffect(() => {
        if (!data.canEstimate) return;
        if (s.audienceKind === 'segment' && !s.audienceId) return;
        if (s.audienceKind !== 'segment' && s.audienceKind !== 'all') return;
        if (audienceKey in estimates) return;
        let cancelled = false;
        audienceEstimateAction(
            s.audienceKind,
            s.audienceKind === 'segment' ? s.audienceId : undefined,
        ).then((r) => {
            if (!cancelled)
                setEstimates((e) => ({
                    ...e,
                    [audienceKey]: r.ok ? r.data : null,
                }));
        });
        return () => {
            cancelled = true;
        };
    }, [
        audienceKey,
        s.audienceKind,
        s.audienceId,
        estimates,
        data.canEstimate,
    ]);

    // ── Validation ──────────────────────────────────────────────────────
    function validateStep(i: number): Record<string, string> {
        const e: Record<string, string> = {};
        if (i === 0) {
            if (!s.name.trim()) e.name = 'Give the campaign a name.';
            if (numbers && numbers.length === 0)
                e.phone_number_id =
                    'No WhatsApp phone number is connected yet.';
            else if (numbers && !sender)
                e.phone_number_id = 'Choose the number that sends.';
        }
        if (i === 1) {
            if (!template) e.template = 'Choose an approved template.';
            else if (slots?.mediaHeader)
                e.template =
                    'Templates with an image, video or document header cannot be used in campaigns yet.';
        }
        if (i === 2 && slots) {
            for (const v of slots.header)
                if (!s.header[v]?.trim()) e[`header.${v}`] = 'Required.';
            for (const v of slots.body)
                if (!s.body[v]?.trim()) e[`body.${v}`] = 'Required.';
            for (const p of slots.urlButtons)
                if (!s.buttons[String(p)]?.trim())
                    e[`button.${p}`] = 'Required.';
            if (buttonGap)
                e.buttons =
                    'This template has a dynamic website button after another button; campaigns can only fill dynamic buttons that come first.';
        }
        if (i === 3) {
            if (s.audienceKind !== 'all' && !s.audienceId)
                e.audience = `Choose a ${s.audienceKind}.`;
        }
        if (i === 4) {
            if (s.windowSize) {
                const n = Number(s.windowSize);
                if (!Number.isInteger(n) || n < 0 || n > 10000)
                    e.window_size = 'A whole number from 0 to 10000.';
            }
            if (s.when === 'schedule') {
                const at = zonedToUtc(s.scheduledAt, data.timezone);
                if (!at) e.scheduled_at = 'Choose a date and time.';
                else if (!isFuture(at))
                    e.scheduled_at = 'Choose a time in the future.';
            }
        }
        return e;
    }

    const next = () => {
        const e = validateStep(step);
        setErrors(e);
        if (Object.keys(e).length === 0) goTo(step + 1);
    };

    const goTo = (i: number) => {
        setStep(i);
        requestAnimationFrame(() => headingRef.current?.focus());
    };

    // ── Saving ──────────────────────────────────────────────────────────
    function buildInput(): CampaignInput | null {
        if (!template || !slots) return null;
        const pick = (vars: string[], src: Record<string, string>) =>
            slots.named
                ? Object.fromEntries(vars.map((v) => [v, src[v]?.trim() ?? '']))
                : vars.map((v) => src[v]?.trim() ?? '');
        const params: CampaignInput['template_params'] = {};
        if (slots.header.length) params.header = pick(slots.header, s.header);
        if (slots.body.length) params.body = pick(slots.body, s.body);
        if (slots.urlButtons.length)
            params.buttons = slots.urlButtons.map(
                (p) => s.buttons[String(p)]?.trim() ?? '',
            );
        const fallback = Object.fromEntries(
            usedPlaceholders
                .map((k) => [k, s.fallback[k]?.trim() ?? ''] as const)
                .filter(([, v]) => v),
        );
        if (Object.keys(fallback).length) params.fallback = fallback;
        const audience: CampaignInput['audience'] =
            s.audienceKind === 'all'
                ? { all: true }
                : { [`${s.audienceKind}_id`]: s.audienceId };
        return {
            name: s.name.trim(),
            phone_number_id: sender?.id ?? (s.phoneNumberId || undefined),
            template_name: template.name,
            template_language: template.language,
            template_params: params,
            audience,
            window_size: s.windowSize ? Number(s.windowSize) : 0,
        };
    }

    function showFailure(r: Failure) {
        const fe = r.fieldErrors ?? {};
        setErrors(fe);
        setFailure(r);
        const keys = Object.keys(fe);
        if (keys.length > 0) {
            goTo(Math.min(...keys.map(stepOf)));
        }
        if (r.code !== 'VALIDATION_ERROR') toast.error(r.message);
    }

    /** Saves and returns the campaign id, or null after showing errors. */
    async function save(): Promise<string | null> {
        for (let i = 0; i < STEPS.length; i++) {
            const e = validateStep(i);
            if (Object.keys(e).length > 0) {
                setErrors(e);
                goTo(i);
                return null;
            }
        }
        const input = buildInput();
        if (!input) return null;
        const r = await guard(() => saveCampaignAction(savedId, input, idem));
        if (!r.ok) {
            if (!savedId) setIdem(newIdempotencyKey());
            showFailure(r);
            return null;
        }
        if (!savedId) {
            setSavedId(r.data.id);
            // Later saves edit this draft instead of creating another one.
            window.history.replaceState(
                null,
                '',
                `/campaigns/${r.data.id}/edit`,
            );
        }
        return r.data.id;
    }

    const saveDraft = () =>
        startTransition(async () => {
            setFailure(null);
            const id = await save();
            if (id) {
                toast.success('Draft saved.');
                router.push(`/campaigns/${id}`);
            }
        });

    const start = () =>
        startTransition(async () => {
            setFailure(null);
            const id = await save();
            if (!id) return;
            const at =
                s.when === 'schedule'
                    ? zonedToUtc(s.scheduledAt, data.timezone)
                    : null;
            const r = await guard(() =>
                startCampaignAction(
                    id,
                    at ? at.toISOString() : null,
                    newIdempotencyKey(),
                ),
            );
            setConfirmOpen(false);
            if (!r.ok) {
                showFailure(r);
                return;
            }
            toast.success(r.message ?? 'Campaign started.');
            router.push(`/campaigns/${id}`);
        });

    // ── Rendering helpers ───────────────────────────────────────────────
    const err = (k: string) =>
        errors[k] ? (
            <p
                id={`cw-${k}-error`}
                role="alert"
                className="text-sm text-destructive"
            >
                {errors[k]}
            </p>
        ) : null;
    const aria = (k: string) => ({
        'aria-invalid': errors[k] ? true : undefined,
        'aria-describedby': errors[k] ? `cw-${k}-error` : undefined,
    });
    const otherErrors = Object.entries(errors).filter(
        ([k]) =>
            ![
                'name',
                'phone_number_id',
                'template',
                'template_name',
                'template_language',
                'audience',
                'window_size',
                'scheduled_at',
                'buttons',
            ].includes(k) && !/^(header|body|button)\./.test(k),
    );

    return (
        <div className="space-y-6">
            <nav aria-label="Campaign steps">
                <ol className="flex flex-wrap gap-2">
                    {STEPS.map((label, i) => (
                        <li key={label}>
                            <Button
                                type="button"
                                size="sm"
                                variant={i === step ? 'default' : 'ghost'}
                                aria-current={i === step ? 'step' : undefined}
                                disabled={i > step || pending}
                                onClick={() => goTo(i)}
                            >
                                <span
                                    aria-hidden
                                    className="flex size-5 items-center justify-center rounded-full border text-xs"
                                >
                                    {i < step ? (
                                        <Check className="size-3" />
                                    ) : (
                                        i + 1
                                    )}
                                </span>
                                {label}
                            </Button>
                        </li>
                    ))}
                </ol>
            </nav>

            {failure &&
                (failure.code !== 'VALIDATION_ERROR' ||
                    otherErrors.length > 0) && (
                    <Alert variant="destructive" data-testid="wizard-error">
                        <AlertCircle />
                        <AlertTitle>
                            {failure.code === 'NO_PHONE_NUMBER'
                                ? 'No phone number connected'
                                : 'The campaign was not saved'}
                        </AlertTitle>
                        <AlertDescription>
                            <p>{failure.message}</p>
                            {otherErrors.map(([k, v]) => (
                                <p key={k}>
                                    {k && <code>{k}</code>} {v}
                                </p>
                            ))}
                            {failure.code === 'NO_PHONE_NUMBER' && (
                                <Link href="/channels" className="underline">
                                    Open Channels
                                </Link>
                            )}
                        </AlertDescription>
                    </Alert>
                )}

            <Card>
                <CardHeader>
                    <CardTitle>
                        <h2
                            ref={headingRef}
                            tabIndex={-1}
                            className="outline-none"
                        >
                            {step + 1}. {STEPS[step]}
                        </h2>
                    </CardTitle>
                    <CardDescription>
                        {
                            [
                                'Name the campaign and choose the number it is sent from.',
                                'Only templates approved by Meta for the sender’s account can be sent.',
                                'Fill each template variable with contact data or fixed text.',
                                'Who receives the campaign. Opted-out contacts are always skipped.',
                                'Check everything, then save as draft, start now or schedule.',
                            ][step]
                        }
                    </CardDescription>
                </CardHeader>
                <CardContent>
                    {step === 0 && (
                        <FieldGroup>
                            <Field data-invalid={!!errors.name}>
                                <FieldLabel htmlFor="cw-name">Name</FieldLabel>
                                <Input
                                    id="cw-name"
                                    {...aria('name')}
                                    value={s.name}
                                    maxLength={120}
                                    placeholder="November newsletter"
                                    onChange={(e) =>
                                        set('name', e.target.value)
                                    }
                                />
                                {err('name')}
                            </Field>
                            {numbers === null ? (
                                <p className="text-sm text-muted-foreground">
                                    The default number sends this campaign.
                                </p>
                            ) : numbers.length === 0 ? (
                                <Alert
                                    variant="destructive"
                                    data-testid="no-phone-number"
                                >
                                    <AlertCircle />
                                    <AlertTitle>
                                        No phone number connected
                                    </AlertTitle>
                                    <AlertDescription>
                                        <p>
                                            Connect a WhatsApp Business account
                                            before creating campaigns.
                                        </p>
                                        <Link
                                            href="/channels"
                                            className="underline"
                                        >
                                            Open Channels
                                        </Link>
                                    </AlertDescription>
                                </Alert>
                            ) : (
                                <Field data-invalid={!!errors.phone_number_id}>
                                    <FieldLabel htmlFor="cw-sender">
                                        Sender
                                    </FieldLabel>
                                    <Select
                                        value={sender?.id ?? ''}
                                        onValueChange={(v) => {
                                            set('phoneNumberId', v);
                                            set('template', '');
                                        }}
                                    >
                                        <SelectTrigger
                                            id="cw-sender"
                                            className="w-full"
                                            {...aria('phone_number_id')}
                                        >
                                            <SelectValue placeholder="Choose a number" />
                                        </SelectTrigger>
                                        <SelectContent>
                                            {numbers.map((n) => (
                                                <SelectItem
                                                    key={n.id}
                                                    value={n.id}
                                                >
                                                    {n.display_phone_number}
                                                    {n.verified_name
                                                        ? ` · ${n.verified_name}`
                                                        : ''}
                                                    {n.is_default
                                                        ? ' (default)'
                                                        : ''}
                                                </SelectItem>
                                            ))}
                                        </SelectContent>
                                    </Select>
                                    {err('phone_number_id') ?? (
                                        <FieldDescription>
                                            {sender?.messaging_limit
                                                ? `Messaging limit: ${sender.messaging_limit}.`
                                                : 'Templates come from this number’s account.'}
                                        </FieldDescription>
                                    )}
                                </Field>
                            )}
                            {errors.phone_number_id &&
                                numbers?.length === 0 && (
                                    <p role="alert" className="sr-only">
                                        {errors.phone_number_id}
                                    </p>
                                )}
                        </FieldGroup>
                    )}

                    {step === 1 && (
                        <div className="grid gap-6 md:grid-cols-[1fr_18rem]">
                            <Field data-invalid={!!errors.template}>
                                <FieldLabel htmlFor="cw-template">
                                    Template
                                </FieldLabel>
                                {templates.length === 0 ? (
                                    <Alert data-testid="no-templates">
                                        <Info />
                                        <AlertDescription>
                                            No approved templates for this
                                            sender.{' '}
                                            <Link
                                                href="/templates"
                                                className="underline"
                                            >
                                                Manage templates
                                            </Link>
                                        </AlertDescription>
                                    </Alert>
                                ) : (
                                    <Select
                                        value={template ? s.template : ''}
                                        onValueChange={(v) =>
                                            setS((p) => ({
                                                ...p,
                                                template: v,
                                                header: {},
                                                body: {},
                                                buttons: {},
                                            }))
                                        }
                                    >
                                        <SelectTrigger
                                            id="cw-template"
                                            className="w-full"
                                            {...aria('template')}
                                        >
                                            <SelectValue placeholder="Choose a template" />
                                        </SelectTrigger>
                                        <SelectContent>
                                            {templates.map((t) => (
                                                <SelectItem
                                                    key={t.id}
                                                    value={templateKey(t)}
                                                >
                                                    {t.name} ({t.language}) ·{' '}
                                                    {t.category.toLowerCase()}
                                                </SelectItem>
                                            ))}
                                        </SelectContent>
                                    </Select>
                                )}
                                {err('template') ??
                                    err('template_name') ??
                                    err('template_language')}
                            </Field>
                            {template && (
                                <div className="space-y-2">
                                    <p className="text-sm font-medium">
                                        Preview
                                    </p>
                                    <TemplatePreview
                                        components={template.components}
                                    />
                                </div>
                            )}
                        </div>
                    )}

                    {step === 2 &&
                        (!slots ? (
                            <p className="text-sm text-muted-foreground">
                                Choose a template first.
                            </p>
                        ) : (
                            <div className="grid gap-6 md:grid-cols-[1fr_18rem]">
                                <div className="space-y-6">
                                    {slots.header.length +
                                        slots.body.length +
                                        slots.urlButtons.length ===
                                        0 && (
                                        <p className="text-sm text-muted-foreground">
                                            This template has no variables.
                                        </p>
                                    )}
                                    {slots.header.map((v) => (
                                        <VariableInput
                                            key={`h-${v}`}
                                            id={`header.${v}`}
                                            label={`Header {{${v}}}`}
                                            value={s.header[v] ?? ''}
                                            error={
                                                errors[`header.${v}`] ??
                                                errors[`template_params.header`]
                                            }
                                            onChange={(x) =>
                                                set('header', {
                                                    ...s.header,
                                                    [v]: x,
                                                })
                                            }
                                        />
                                    ))}
                                    {slots.body.map((v) => (
                                        <VariableInput
                                            key={`b-${v}`}
                                            id={`body.${v}`}
                                            label={`Body {{${v}}}`}
                                            value={s.body[v] ?? ''}
                                            error={errors[`body.${v}`]}
                                            onChange={(x) =>
                                                set('body', {
                                                    ...s.body,
                                                    [v]: x,
                                                })
                                            }
                                        />
                                    ))}
                                    {slots.urlButtons.map((p) => (
                                        <VariableInput
                                            key={`u-${p}`}
                                            id={`button.${p}`}
                                            label={`Website button ${p + 1}: end of address`}
                                            value={s.buttons[String(p)] ?? ''}
                                            error={errors[`button.${p}`]}
                                            onChange={(x) =>
                                                set('buttons', {
                                                    ...s.buttons,
                                                    [String(p)]: x,
                                                })
                                            }
                                        />
                                    ))}
                                    {errors.buttons && (
                                        <Alert variant="destructive">
                                            <AlertCircle />
                                            <AlertDescription role="alert">
                                                {errors.buttons}
                                            </AlertDescription>
                                        </Alert>
                                    )}
                                    {usedPlaceholders.length > 0 && (
                                        <FieldSet>
                                            <FieldLegend variant="label">
                                                Fallbacks
                                            </FieldLegend>
                                            <FieldDescription>
                                                Used when a contact has no
                                                value. Without a fallback such
                                                contacts are skipped, because
                                                Meta rejects empty variables.
                                            </FieldDescription>
                                            <div className="grid gap-3 sm:grid-cols-2">
                                                {usedPlaceholders.map((k) => (
                                                    <Field key={k}>
                                                        <FieldLabel
                                                            htmlFor={`cw-fallback-${k}`}
                                                        >
                                                            {`If {{${k}}} is empty`}
                                                        </FieldLabel>
                                                        <Input
                                                            id={`cw-fallback-${k}`}
                                                            value={
                                                                s.fallback[k] ??
                                                                ''
                                                            }
                                                            maxLength={1024}
                                                            placeholder="Skip the contact"
                                                            onChange={(e) =>
                                                                set(
                                                                    'fallback',
                                                                    {
                                                                        ...s.fallback,
                                                                        [k]: e
                                                                            .target
                                                                            .value,
                                                                    },
                                                                )
                                                            }
                                                        />
                                                    </Field>
                                                ))}
                                            </div>
                                        </FieldSet>
                                    )}
                                </div>
                                {template && (
                                    <div className="space-y-2">
                                        <p className="text-sm font-medium">
                                            Preview
                                        </p>
                                        <TemplatePreview
                                            components={template.components}
                                            values={{
                                                header: s.header,
                                                body: s.body,
                                            }}
                                        />
                                    </div>
                                )}
                            </div>
                        ))}

                    {step === 3 && (
                        <FieldGroup>
                            <FieldSet>
                                <FieldLegend variant="label">
                                    Send to
                                </FieldLegend>
                                <div
                                    role="radiogroup"
                                    aria-label="Audience type"
                                    className="flex flex-wrap gap-4"
                                >
                                    {(
                                        [
                                            [
                                                'segment',
                                                'A segment',
                                                data.segments,
                                            ],
                                            ['group', 'A group', data.groups],
                                            ['tag', 'A tag', data.tags],
                                            ['all', 'All contacts', []],
                                        ] as const
                                    ).map(([kind, label, list]) => (
                                        <label
                                            key={kind}
                                            className="flex items-center gap-2 text-sm"
                                        >
                                            <input
                                                type="radio"
                                                name="audience-kind"
                                                value={kind}
                                                checked={
                                                    s.audienceKind === kind
                                                }
                                                disabled={list === null}
                                                className="size-4 accent-primary"
                                                onChange={() =>
                                                    setS((p) => ({
                                                        ...p,
                                                        audienceKind: kind,
                                                        audienceId: '',
                                                    }))
                                                }
                                            />
                                            {label}
                                        </label>
                                    ))}
                                </div>
                            </FieldSet>
                            {s.audienceKind !== 'all' && (
                                <AudiencePicker
                                    kind={s.audienceKind}
                                    value={s.audienceId}
                                    error={
                                        errors.audience ??
                                        errors[`audience.${s.audienceKind}_id`]
                                    }
                                    options={
                                        (s.audienceKind === 'segment'
                                            ? data.segments
                                            : s.audienceKind === 'group'
                                              ? data.groups?.map((g) => ({
                                                    id: g.id,
                                                    name: `${g.name} (${g.member_count})`,
                                                }))
                                              : data.tags?.map((t) => ({
                                                    id: t.id,
                                                    name: `${t.name} (${t.contact_count})`,
                                                }))) ?? []
                                    }
                                    onChange={(v) => set('audienceId', v)}
                                />
                            )}
                            {s.audienceKind === 'all' &&
                                errors.audience &&
                                err('audience')}
                            <EstimateLine
                                estimate={estimate}
                                kind={s.audienceKind}
                                chosen={
                                    s.audienceKind === 'all' || !!s.audienceId
                                }
                            />
                        </FieldGroup>
                    )}

                    {step === 4 && (
                        <div className="space-y-6">
                            <dl className="grid gap-x-6 gap-y-2 text-sm sm:grid-cols-[10rem_1fr]">
                                <dt className="text-muted-foreground">Name</dt>
                                <dd>{s.name}</dd>
                                <dt className="text-muted-foreground">
                                    Sender
                                </dt>
                                <dd>
                                    {sender
                                        ? `${sender.display_phone_number}${sender.verified_name ? ` · ${sender.verified_name}` : ''}`
                                        : 'Default number'}
                                </dd>
                                <dt className="text-muted-foreground">
                                    Template
                                </dt>
                                <dd className="font-mono">
                                    {template
                                        ? `${template.name} (${template.language})`
                                        : '—'}
                                </dd>
                                {slots &&
                                    [
                                        ...slots.header.map((v) => [
                                            `Header {{${v}}}`,
                                            s.header[v],
                                        ]),
                                        ...slots.body.map((v) => [
                                            `Body {{${v}}}`,
                                            s.body[v],
                                        ]),
                                        ...slots.urlButtons.map((p) => [
                                            `Button ${p + 1}`,
                                            s.buttons[String(p)],
                                        ]),
                                    ].map(([k, v]) => (
                                        <div key={k} className="contents">
                                            <dt className="text-muted-foreground">
                                                {k}
                                            </dt>
                                            <dd className="font-mono break-all">
                                                {v}
                                            </dd>
                                        </div>
                                    ))}
                                <dt className="text-muted-foreground">
                                    Audience
                                </dt>
                                <dd>
                                    {audienceLabel(
                                        s.audienceKind === 'all'
                                            ? { all: true }
                                            : {
                                                  [`${s.audienceKind}_id`]:
                                                      s.audienceId,
                                              },
                                        names,
                                    )}
                                    {typeof estimate === 'number' &&
                                        ` · about ${estimate.toLocaleString()} contacts`}
                                </dd>
                            </dl>

                            <details className="rounded-lg border p-3">
                                <summary className="cursor-pointer text-sm font-medium">
                                    Advanced
                                </summary>
                                <Field
                                    className="mt-3 max-w-xs"
                                    data-invalid={!!errors.window_size}
                                >
                                    <FieldLabel htmlFor="cw-window">
                                        Messages in flight
                                    </FieldLabel>
                                    <Input
                                        id="cw-window"
                                        type="number"
                                        min={0}
                                        max={10000}
                                        step={1}
                                        placeholder="Automatic"
                                        {...aria('window_size')}
                                        value={s.windowSize}
                                        onChange={(e) =>
                                            set('windowSize', e.target.value)
                                        }
                                    />
                                    {err('window_size') ?? (
                                        <FieldDescription>
                                            How many messages may wait for Meta
                                            at once. Empty or 0 = automatic
                                            (800).
                                        </FieldDescription>
                                    )}
                                </Field>
                            </details>

                            {data.canStart ? (
                                <FieldSet>
                                    <FieldLegend variant="label">
                                        When
                                    </FieldLegend>
                                    <div
                                        role="radiogroup"
                                        aria-label="When"
                                        className="flex flex-wrap gap-4"
                                    >
                                        {(
                                            [
                                                ['now', 'Start now'],
                                                ['schedule', 'Schedule'],
                                            ] as const
                                        ).map(([v, label]) => (
                                            <label
                                                key={v}
                                                className="flex items-center gap-2 text-sm"
                                            >
                                                <input
                                                    type="radio"
                                                    name="when"
                                                    value={v}
                                                    checked={s.when === v}
                                                    className="size-4 accent-primary"
                                                    onChange={() =>
                                                        set('when', v)
                                                    }
                                                />
                                                {label}
                                            </label>
                                        ))}
                                    </div>
                                    {s.when === 'schedule' && (
                                        <Field
                                            className="max-w-xs"
                                            data-invalid={!!errors.scheduled_at}
                                        >
                                            <FieldLabel htmlFor="cw-at">
                                                Date and time ({data.timezone})
                                            </FieldLabel>
                                            <Input
                                                id="cw-at"
                                                type="datetime-local"
                                                {...aria('scheduled_at')}
                                                value={s.scheduledAt}
                                                onChange={(e) =>
                                                    set(
                                                        'scheduledAt',
                                                        e.target.value,
                                                    )
                                                }
                                            />
                                            {err('scheduled_at') ?? (
                                                <FieldDescription>
                                                    Your profile’s time zone.
                                                    {(() => {
                                                        const at = zonedToUtc(
                                                            s.scheduledAt,
                                                            data.timezone,
                                                        );
                                                        return at
                                                            ? ` Starts ${formatInZone(at, Intl.DateTimeFormat().resolvedOptions().timeZone)} for you here.`
                                                            : '';
                                                    })()}
                                                </FieldDescription>
                                            )}
                                        </Field>
                                    )}
                                </FieldSet>
                            ) : (
                                <p className="text-sm text-muted-foreground">
                                    You can save the draft; someone allowed to
                                    start campaigns sends it.
                                </p>
                            )}
                        </div>
                    )}
                </CardContent>
            </Card>

            <div className="flex flex-wrap justify-between gap-2">
                <Button
                    type="button"
                    variant="outline"
                    disabled={step === 0 || pending}
                    onClick={() => goTo(step - 1)}
                >
                    Back
                </Button>
                <div className="flex flex-wrap gap-2">
                    {step < STEPS.length - 1 ? (
                        <Button type="button" onClick={next} disabled={pending}>
                            Next
                        </Button>
                    ) : (
                        <>
                            <Button
                                type="button"
                                variant="outline"
                                onClick={saveDraft}
                                disabled={pending}
                            >
                                {pending && <Spinner />} Save draft
                            </Button>
                            {data.canStart && (
                                <Button
                                    type="button"
                                    disabled={pending}
                                    onClick={() => {
                                        const e = validateStep(4);
                                        setErrors(e);
                                        if (Object.keys(e).length > 0) return;
                                        if (s.when === 'schedule') start();
                                        else setConfirmOpen(true);
                                    }}
                                >
                                    {pending && <Spinner />}
                                    {s.when === 'schedule'
                                        ? 'Schedule campaign'
                                        : 'Start campaign'}
                                </Button>
                            )}
                        </>
                    )}
                </div>
            </div>

            <AlertDialog open={confirmOpen} onOpenChange={setConfirmOpen}>
                <AlertDialogContent>
                    <AlertDialogHeader>
                        <AlertDialogTitle>Start sending now?</AlertDialogTitle>
                        <AlertDialogDescription>
                            {typeof estimate === 'number'
                                ? `About ${estimate.toLocaleString()} contacts receive “${template?.name}” right away. `
                                : `Contacts receive “${template?.name}” right away. `}
                            You can pause or cancel while it runs, but messages
                            already sent cannot be recalled.
                        </AlertDialogDescription>
                    </AlertDialogHeader>
                    <AlertDialogFooter>
                        <AlertDialogCancel disabled={pending}>
                            Cancel
                        </AlertDialogCancel>
                        <AlertDialogAction
                            disabled={pending}
                            onClick={(e) => {
                                e.preventDefault();
                                start();
                            }}
                        >
                            {pending && <Spinner />} Start now
                        </AlertDialogAction>
                    </AlertDialogFooter>
                </AlertDialogContent>
            </AlertDialog>
        </div>
    );
}

function EstimateLine({
    estimate,
    kind,
    chosen,
}: {
    estimate: number | null | undefined;
    kind: AudienceKind;
    chosen: boolean;
}) {
    if (!chosen) return null;
    return (
        <p
            className="flex items-center gap-2 text-sm text-muted-foreground"
            role="status"
            data-testid="audience-estimate"
        >
            <Users className="size-4" />
            {estimate === undefined
                ? kind === 'segment' || kind === 'all'
                    ? 'Counting recipients…'
                    : 'Recipient count unavailable.'
                : estimate === null
                  ? 'Recipient count unavailable.'
                  : `About ${estimate.toLocaleString()} contacts${kind === 'all' ? ' (not opted out)' : ''}. Duplicates and opted-out contacts are skipped when the campaign starts.`}
        </p>
    );
}

function AudiencePicker({
    kind,
    value,
    options,
    error,
    onChange,
}: {
    kind: Exclude<AudienceKind, 'all'>;
    value: string;
    options: { id: string; name: string }[];
    error?: string;
    onChange: (v: string) => void;
}) {
    const label = { segment: 'Segment', group: 'Group', tag: 'Tag' }[kind];
    return (
        <Field data-invalid={!!error}>
            <FieldLabel htmlFor="cw-audience">{label}</FieldLabel>
            {options.length === 0 ? (
                <p className="text-sm text-muted-foreground">
                    No {label.toLowerCase()}s yet.{' '}
                    <Link href="/contacts" className="underline">
                        Manage contacts
                    </Link>
                </p>
            ) : (
                <Select value={value} onValueChange={onChange}>
                    <SelectTrigger
                        id="cw-audience"
                        className="w-full"
                        aria-invalid={error ? true : undefined}
                        aria-describedby={
                            error ? 'cw-audience-error' : undefined
                        }
                    >
                        <SelectValue
                            placeholder={`Choose a ${label.toLowerCase()}`}
                        />
                    </SelectTrigger>
                    <SelectContent>
                        {options.map((o) => (
                            <SelectItem key={o.id} value={o.id}>
                                {o.name}
                            </SelectItem>
                        ))}
                    </SelectContent>
                </Select>
            )}
            {error && (
                <p
                    id="cw-audience-error"
                    role="alert"
                    className="text-sm text-destructive"
                >
                    {error}
                </p>
            )}
        </Field>
    );
}

/** A variable's value with placeholder chips that insert at the cursor. */
function VariableInput({
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
    const ref = useRef<HTMLInputElement>(null);
    const inputId = `cw-${id}`;
    const insert = (token: string, caretBack = 0) => {
        const el = ref.current;
        const start = el?.selectionStart ?? value.length;
        const end = el?.selectionEnd ?? value.length;
        const next = value.slice(0, start) + token + value.slice(end);
        onChange(next);
        const caret = start + token.length - caretBack;
        requestAnimationFrame(() => {
            el?.focus();
            el?.setSelectionRange(caret, caret);
        });
    };
    return (
        <Field data-invalid={!!error}>
            <FieldLabel htmlFor={inputId}>{label}</FieldLabel>
            <Input
                ref={ref}
                id={inputId}
                value={value}
                maxLength={1024}
                placeholder="{{first_name}} or fixed text"
                aria-invalid={error ? true : undefined}
                aria-describedby={`${inputId}-chips${error ? ` ${inputId}-error` : ''}`}
                onChange={(e) => onChange(e.target.value)}
            />
            <div
                id={`${inputId}-chips`}
                className="flex flex-wrap gap-1"
                role="group"
                aria-label={`Insert contact data into ${label}`}
            >
                {PLACEHOLDERS.map((p) => (
                    <Badge key={p} asChild variant="outline">
                        <button
                            type="button"
                            className="cursor-pointer font-mono hover:bg-muted"
                            onClick={() => insert(`{{${p}}}`)}
                        >
                            {`{{${p}}}`}
                        </button>
                    </Badge>
                ))}
                <Badge asChild variant="outline">
                    <button
                        type="button"
                        className="cursor-pointer font-mono hover:bg-muted"
                        aria-label="Insert a custom attribute, then type its key"
                        onClick={() => insert('{{attr:}}', 2)}
                    >
                        {'{{attr:key}}'}
                    </button>
                </Badge>
            </div>
            {error && (
                <p
                    id={`${inputId}-error`}
                    role="alert"
                    className="text-sm text-destructive"
                >
                    {error}
                </p>
            )}
        </Field>
    );
}
