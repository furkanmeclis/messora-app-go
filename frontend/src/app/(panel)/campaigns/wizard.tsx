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
import { DateTimePicker } from '@/components/ui/date-picker';
import { Input } from '@/components/ui/input';
import { NumberField } from '@/components/ui/number-field';
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
import { useT } from '@/i18n/client';
import type { Translate } from '@/i18n/translate';
import {
    emptyValues,
    fieldOf,
    issuesOf,
    isMediaHeader,
    paramsFromValues,
    shapeOf,
    templateKey,
    valuesFromParams,
    type OfferFills,
    type ParamValues,
    type SampleContact,
} from '@/lib/templates';
import {
    ParamIssues,
    TemplateParamsEditor,
} from '@/components/template-params';
import { WhatsAppPreview } from '@/components/whatsapp-preview';
import { Combobox } from '@/components/ui/combobox';
import { formatInZone, utcToZoned, zonedToUtc } from '@/lib/timezone';
import type { ActionResult, Campaign, Template } from '@/lib/types';
import {
    audienceEstimateAction,
    preflightAction,
    saveCampaignAction,
    startCampaignAction,
    type CampaignInput,
    type Preflight,
} from './actions';
import {
    audienceLabel,
    PRIORITIES,
    priorityLabel,
    type Priority,
} from './campaign-ui';
import { PreviousAudience } from './previous-audience';
import { PreflightPanel } from './preflight';
import type { WizardData } from './wizard-data';
import { NUMBER_LOCALE } from '@/lib/format';

type Failure = Extract<ActionResult<unknown>, { ok: false }>;
type AudienceKind = 'segment' | 'group' | 'tag' | 'all' | 'previous';

type State = {
    name: string;
    /** '' = the default number. */
    phoneNumberId: string;
    template: string;
    params: ParamValues;
    audienceKind: AudienceKind;
    audienceId: string;
    windowSize: string;
    when: 'now' | 'schedule';
    scheduledAt: string;
    /** Linked offer ('' = none). */
    offerId: string;
    /** Unreached classes of a previous-campaign audience. */
    prevClasses: string[];
    priority: Priority;
    /** Rate cap in messages per second ('' = none). */
    maxMps: string;
};

/** The audience the API expects for the chosen kind. */
function audienceOf(s: State): CampaignInput['audience'] {
    if (s.audienceKind === 'all') return { all: true };
    if (s.audienceKind === 'previous')
        return {
            previous_campaign: {
                campaign_id: s.audienceId,
                classes: s.prevClasses as NonNullable<
                    CampaignInput['audience']['previous_campaign']
                >['classes'],
            },
        };
    return { [`${s.audienceKind}_id`]: s.audienceId };
}

const STEPS = [
    'basics',
    'template',
    'variables',
    'audience',
    'review',
] as const;

const CATEGORIES = ['MARKETING', 'UTILITY', 'AUTHENTICATION'] as const;

/** A template category in the user's language, lower case as in lists. */
function categoryLabel(t: Translate, category: string): string {
    const c = CATEGORIES.find((x) => x === category.toUpperCase());
    return (c ? t(`templates.categories.${c}`) : category).toLowerCase();
}

/** At least a minute ahead (the API starts anything sooner right away). */
function isFuture(at: Date): boolean {
    return at.getTime() > Date.now() + 60_000;
}

function initialState(
    c: Campaign | undefined,
    timezone: string,
    templates: Template[],
    numbers: WizardData['numbers'],
    initialTemplate?: string,
): State {
    if (!c) {
        // Preselected from the templates page: also pick a sender of the
        // template's account (templates are per account).
        const pre = initialTemplate
            ? templates.find((x) => templateKey(x) === initialTemplate)
            : undefined;
        const def = numbers?.find((n) => n.is_default);
        const sender =
            pre && def?.account_id !== pre.account_id
                ? numbers?.find((n) => n.account_id === pre.account_id)
                : undefined;
        return {
            name: '',
            phoneNumberId: sender?.id ?? '',
            template: pre ? templateKey(pre) : '',
            params: emptyValues(),
            audienceKind: 'segment',
            audienceId: '',
            windowSize: '',
            when: 'now',
            scheduledAt: '',
            offerId: '',
            prevClasses: ['temporary', 'ecosystem_limit', 'never_sent'],
            priority: 'normal',
            maxMps: '',
        };
    }
    const a = c.audience;
    const kind: AudienceKind = a.all
        ? 'all'
        : a.previous_campaign
          ? 'previous'
          : a.group_id
            ? 'group'
            : a.tag_id
              ? 'tag'
              : 'segment';
    const tpl = templates.find(
        (x) => x.name === c.template_name && x.language === c.template_language,
    );
    return {
        name: c.name,
        phoneNumberId: c.phone_number_id,
        template: templateKey({
            name: c.template_name,
            language: c.template_language,
        }),
        // Everything stored is kept on edit, header media included.
        params: tpl
            ? valuesFromParams(c.template_params, shapeOf(tpl))
            : emptyValues(),
        audienceKind: kind,
        audienceId:
            a.segment_id ??
            a.group_id ??
            a.tag_id ??
            a.previous_campaign?.campaign_id ??
            '',
        windowSize: c.window_size ? String(c.window_size) : '',
        when: c.scheduled_at ? 'schedule' : 'now',
        scheduledAt: c.scheduled_at ? utcToZoned(c.scheduled_at, timezone) : '',
        offerId: c.offer_id ?? '',
        prevClasses: a.previous_campaign?.classes ?? [],
        priority: c.priority,
        maxMps: c.max_mps ? String(c.max_mps) : '',
    };
}

/** Which wizard step an API field error belongs to. */
function stepOf(field: string): number {
    if (field === 'name' || field === 'phone_number_id') return 0;
    if (field === 'offer_id') return 1;
    if (field.startsWith('template_params')) return 2;
    if (field.startsWith('template')) return 1;
    if (field.startsWith('audience')) return 3;
    return 4;
}

export function CampaignWizard({
    data,
    campaign,
    initialTemplate,
}: {
    data: WizardData;
    campaign?: Campaign;
    initialTemplate?: string;
}) {
    const t = useT();
    const router = useRouter();
    const { guard } = useReauth();
    const [s, setS] = useState<State>(() =>
        initialState(
            campaign,
            data.timezone,
            data.templates,
            data.numbers,
            initialTemplate,
        ),
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
    const [preflights, setPreflights] = useState<
        Record<string, ActionResult<Preflight>>
    >({});
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
    const shape = useMemo(
        () => (template ? shapeOf(template) : null),
        [template],
    );
    // A linked offer fills the countdown end, the coupon and an image.
    const offer = data.offers?.find((o) => o.id === s.offerId);
    const fills: OfferFills = offer
        ? {
              expiration: true,
              coupon: !!offer.coupon_code,
              media: !!(offer.image_url || offer.image_media_id),
          }
        : {};
    const sample: SampleContact | undefined = data.sample
        ? { ...data.sample, name: data.sample.name || data.sample.profile_name }
        : undefined;

    const names: Record<string, string> = {};
    for (const x of data.segments ?? []) names[x.id] = x.name;
    for (const x of data.groups ?? []) names[x.id] = x.name;
    for (const x of data.tags ?? []) names[x.id] = x.name;
    for (const x of data.finished ?? []) names[x.id] = x.name;

    const audienceKey =
        s.audienceKind === 'all'
            ? 'all'
            : s.audienceKind === 'previous'
              ? `previous:${s.audienceId}:${[...s.prevClasses].sort().join(',')}`
              : `${s.audienceKind}:${s.audienceId}`;
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

    // The consent check runs on the review step for the chosen sender,
    // template and audience (cached per combination).
    const audienceChosen =
        s.audienceKind === 'all' ||
        (!!s.audienceId &&
            (s.audienceKind !== 'previous' || s.prevClasses.length > 0));
    const preflightKey =
        template && audienceChosen
            ? `${audienceKey}|${sender?.id ?? s.phoneNumberId}|${templateKey(template)}`
            : null;
    const preflight = preflightKey ? preflights[preflightKey] : undefined;
    useEffect(() => {
        // The previous-campaign audience shows its summary live on the
        // audience step.
        const live = step === 3 && s.audienceKind === 'previous';
        if ((step !== 4 && !live) || !preflightKey || !template) return;
        if (preflightKey in preflights) return;
        let cancelled = false;
        preflightAction({
            audience: audienceOf(s),
            ...((sender?.id ?? s.phoneNumberId)
                ? { phone_number_id: sender?.id ?? s.phoneNumberId }
                : {}),
            template_name: template.name,
            template_language: template.language,
        }).then((r) => {
            if (!cancelled) setPreflights((p) => ({ ...p, [preflightKey]: r }));
        });
        return () => {
            cancelled = true;
        };
    }, [step, preflightKey, preflights, template, sender, s]);
    const eligible = preflight?.ok ? preflight.data.eligible : undefined;

    // ── Validation ──────────────────────────────────────────────────────
    function validateStep(i: number): Record<string, string> {
        const e: Record<string, string> = {};
        if (i === 0) {
            if (!s.name.trim()) e.name = t('campaigns.wizard.errors.name');
            if (numbers && numbers.length === 0)
                e.phone_number_id = t('campaigns.wizard.errors.noNumber');
            else if (numbers && !sender)
                e.phone_number_id = t('campaigns.wizard.errors.chooseNumber');
        }
        if (i === 1) {
            if (!template)
                e.template = t('campaigns.wizard.errors.chooseTemplate');
            if (offer && offer.status === 'expired')
                e.offer_id = t('offers.campaign.ended');
        }
        if (i === 2 && shape && issuesOf(s.params, shape, fills).length > 0) {
            e.params = t('params.issues.title');
        }
        if (i === 3) {
            if (s.audienceKind === 'previous') {
                if (!s.audienceId)
                    e.audience = t('campaigns.engine.previous.chooseCampaign');
                else if (s.prevClasses.length === 0)
                    e.audience = t('campaigns.engine.previous.chooseClasses');
            } else if (s.audienceKind !== 'all' && !s.audienceId)
                e.audience = t(
                    s.audienceKind === 'segment'
                        ? 'campaigns.wizard.errors.chooseSegment'
                        : s.audienceKind === 'group'
                          ? 'campaigns.wizard.errors.chooseGroup'
                          : 'campaigns.wizard.errors.chooseTag',
                );
        }
        if (i === 4) {
            if (s.windowSize) {
                const n = Number(s.windowSize);
                if (!Number.isInteger(n) || n < 0 || n > 10000)
                    e.window_size = t('campaigns.wizard.errors.windowSize');
            }
            if (s.maxMps) {
                const n = Number(s.maxMps);
                if (!(n > 0 && n <= 1000))
                    e.max_mps = t('campaigns.engine.maxMpsHint');
            }
            if (s.when === 'schedule') {
                const at = zonedToUtc(s.scheduledAt, data.timezone);
                if (!at)
                    e.scheduled_at = t(
                        'campaigns.wizard.errors.chooseDateTime',
                    );
                else if (!isFuture(at))
                    e.scheduled_at = t('campaigns.wizard.errors.futureTime');
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
        if (!template || !shape) return null;
        const params = paramsFromValues(s.params, shape);
        const audience = audienceOf(s);
        return {
            name: s.name.trim(),
            phone_number_id: sender?.id ?? (s.phoneNumberId || undefined),
            template_name: template.name,
            template_language: template.language,
            template_params: params,
            audience,
            window_size: s.windowSize ? Number(s.windowSize) : 0,
            priority: s.priority,
            ...(s.maxMps
                ? { max_mps: Number(s.maxMps) }
                : savedId
                  ? ({ clear_max_mps: true } as Partial<CampaignInput>)
                  : {}),
            ...(s.offerId
                ? { offer_id: s.offerId }
                : savedId
                  ? ({ clear_offer: true } as Partial<CampaignInput>)
                  : {}),
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
                toast.success(t('campaigns.wizard.draftSaved'));
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
            toast.success(r.message ?? t('campaigns.actions.started'));
            router.push(`/campaigns/${id}`);
        });

    // ── Rendering helpers ───────────────────────────────────────────────
    /** A value as the review shows it: the field's name or the text. */
    const describe = (v: string | undefined) => {
        const f = fieldOf(v);
        if (!f) return v?.trim() || '—';
        return f.startsWith('attr:')
            ? t('params.attribute', { key: f.slice(5) })
            : t(`params.fields.${f as 'name'}`);
    };
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
                'max_mps',
                'priority',
                'scheduled_at',
                'buttons',
                'params',
            ].includes(k) && !/^(header|body|button)\./.test(k),
    );

    return (
        <div className="space-y-6">
            <nav aria-label={t('campaigns.wizard.campaignSteps')}>
                <ol className="flex flex-wrap gap-2">
                    {STEPS.map((key, i) => (
                        <li key={key}>
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
                                {t(`campaigns.wizard.steps.${key}`)}
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
                                ? t('campaigns.wizard.noPhoneNumber')
                                : t('campaigns.wizard.notSaved')}
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
                                    {t('campaigns.wizard.openChannels')}
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
                            {step + 1}.{' '}
                            {t(`campaigns.wizard.steps.${STEPS[step]!}`)}
                        </h2>
                    </CardTitle>
                    <CardDescription>
                        {t(`campaigns.wizard.stepHints.${STEPS[step]!}`)}
                    </CardDescription>
                </CardHeader>
                <CardContent>
                    {step === 0 && (
                        <FieldGroup>
                            <Field data-invalid={!!errors.name}>
                                <FieldLabel htmlFor="cw-name">
                                    {t('campaigns.wizard.name')}
                                </FieldLabel>
                                <Input
                                    id="cw-name"
                                    {...aria('name')}
                                    value={s.name}
                                    maxLength={120}
                                    placeholder={t(
                                        'campaigns.wizard.namePlaceholder',
                                    )}
                                    onChange={(e) =>
                                        set('name', e.target.value)
                                    }
                                />
                                {err('name')}
                            </Field>
                            {numbers === null ? (
                                <p className="text-sm text-muted-foreground">
                                    {t('campaigns.wizard.defaultSends')}
                                </p>
                            ) : numbers.length === 0 ? (
                                <Alert
                                    variant="destructive"
                                    data-testid="no-phone-number"
                                >
                                    <AlertCircle />
                                    <AlertTitle>
                                        {t('campaigns.wizard.noPhoneNumber')}
                                    </AlertTitle>
                                    <AlertDescription>
                                        <p>
                                            {t('campaigns.wizard.connectFirst')}
                                        </p>
                                        <Link
                                            href="/channels"
                                            className="underline"
                                        >
                                            {t('campaigns.wizard.openChannels')}
                                        </Link>
                                    </AlertDescription>
                                </Alert>
                            ) : (
                                <Field data-invalid={!!errors.phone_number_id}>
                                    <FieldLabel htmlFor="cw-sender">
                                        {t('campaigns.wizard.sender')}
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
                                            <SelectValue
                                                placeholder={t(
                                                    'campaigns.wizard.chooseNumber',
                                                )}
                                            />
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
                                                        ? t(
                                                              'campaigns.wizard.defaultSuffix',
                                                          )
                                                        : ''}
                                                </SelectItem>
                                            ))}
                                        </SelectContent>
                                    </Select>
                                    {err('phone_number_id') ?? (
                                        <FieldDescription>
                                            {sender?.messaging_limit
                                                ? t(
                                                      'campaigns.wizard.messagingLimit',
                                                      {
                                                          limit: sender.messaging_limit,
                                                      },
                                                  )
                                                : t(
                                                      'campaigns.wizard.templatesFromAccount',
                                                  )}
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
                                    {t('campaigns.wizard.template')}
                                </FieldLabel>
                                {templates.length === 0 ? (
                                    <Alert data-testid="no-templates">
                                        <Info />
                                        <AlertDescription>
                                            {t('campaigns.wizard.noTemplates')}{' '}
                                            <Link
                                                href="/templates"
                                                className="underline"
                                            >
                                                {t(
                                                    'campaigns.wizard.manageTemplates',
                                                )}
                                            </Link>
                                        </AlertDescription>
                                    </Alert>
                                ) : (
                                    <Combobox
                                        id="cw-template"
                                        value={template ? s.template : ''}
                                        onValueChange={(v) =>
                                            setS((p) => ({
                                                ...p,
                                                template: v,
                                                params: emptyValues(),
                                            }))
                                        }
                                        placeholder={t(
                                            'campaigns.wizard.chooseTemplate',
                                        )}
                                        searchPlaceholder={t(
                                            'campaigns.wizard.searchTemplates',
                                        )}
                                        emptyText={t(
                                            'campaigns.wizard.noTemplateMatch',
                                        )}
                                        options={templates.map((tpl) => {
                                            const sh = shapeOf(tpl);
                                            const vars =
                                                sh.headerVars.length +
                                                sh.bodyVars.length +
                                                sh.urlButtons.length;
                                            return {
                                                value: templateKey(tpl),
                                                label: `${tpl.name} (${tpl.language})`,
                                                keywords: `${tpl.name} ${tpl.language} ${tpl.category}`,
                                                hint: [
                                                    categoryLabel(
                                                        t,
                                                        tpl.category,
                                                    ),
                                                    isMediaHeader(sh.header)
                                                        ? t(
                                                              `params.media.${sh.header}`,
                                                          )
                                                        : null,
                                                    vars > 0
                                                        ? t(
                                                              'campaigns.wizard.variableCount',
                                                              { count: vars },
                                                          )
                                                        : null,
                                                ]
                                                    .filter(Boolean)
                                                    .join(' · '),
                                            };
                                        })}
                                    />
                                )}
                                {err('template') ??
                                    err('template_name') ??
                                    err('template_language')}
                            </Field>
                            {data.offers && data.offers.length > 0 && (
                                <Field data-invalid={!!errors.offer_id}>
                                    <FieldLabel htmlFor="cw-offer">
                                        {t('offers.campaign.label')}
                                    </FieldLabel>
                                    <Select
                                        value={s.offerId || 'none'}
                                        onValueChange={(v) =>
                                            set(
                                                'offerId',
                                                v === 'none' ? '' : v,
                                            )
                                        }
                                    >
                                        <SelectTrigger
                                            id="cw-offer"
                                            className="w-full"
                                        >
                                            <SelectValue />
                                        </SelectTrigger>
                                        <SelectContent>
                                            <SelectItem value="none">
                                                {t('offers.campaign.none')}
                                            </SelectItem>
                                            {data.offers.map((o) => (
                                                <SelectItem
                                                    key={o.id}
                                                    value={o.id}
                                                    disabled={
                                                        o.status === 'expired'
                                                    }
                                                >
                                                    {o.title} ·{' '}
                                                    {t(
                                                        `offers.statuses.${o.status}`,
                                                    )}
                                                </SelectItem>
                                            ))}
                                        </SelectContent>
                                    </Select>
                                    <FieldDescription>
                                        {t('offers.campaign.hint')}
                                    </FieldDescription>
                                    {err('offer_id')}
                                </Field>
                            )}
                            {template && (
                                <div className="space-y-2">
                                    <p className="text-sm font-medium">
                                        {t('campaigns.wizard.preview')}
                                    </p>
                                    <WhatsAppPreview
                                        components={template.components}
                                        mediaSrc={shape?.exampleMedia}
                                    />
                                </div>
                            )}
                        </div>
                    )}

                    {step === 2 &&
                        (!shape || !template ? (
                            <p className="text-sm text-muted-foreground">
                                {t('campaigns.wizard.chooseTemplateFirst')}
                            </p>
                        ) : (
                            <div className="grid gap-6 lg:grid-cols-[1fr_22rem]">
                                <div className="space-y-4">
                                    <TemplateParamsEditor
                                        shape={shape}
                                        value={s.params}
                                        onChange={(v) => set('params', v)}
                                        attributeKeys={data.attributeKeys}
                                        phoneNumberId={sender?.id}
                                        fills={fills}
                                    />
                                    {errors.params && (
                                        <ParamIssues
                                            shape={shape}
                                            value={s.params}
                                            fills={fills}
                                        />
                                    )}
                                </div>
                                <div className="space-y-2 lg:sticky lg:top-20 lg:self-start">
                                    <p className="text-sm font-medium">
                                        {sample
                                            ? t('params.preview.as', {
                                                  name:
                                                      sample.name ||
                                                      data.sample?.phone ||
                                                      '',
                                              })
                                            : t('campaigns.wizard.preview')}
                                    </p>
                                    <WhatsAppPreview
                                        components={template.components}
                                        params={s.params}
                                        contact={sample}
                                        category={template.category}
                                        mediaSrc={
                                            s.params.mediaPreview ??
                                            s.params.media?.link ??
                                            offer?.image_url ??
                                            shape.exampleMedia
                                        }
                                        mediaName={s.params.media?.filename}
                                        expiresAt={
                                            offer
                                                ? new Date(
                                                      offer.ends_at,
                                                  ).getTime()
                                                : undefined
                                        }
                                        coupon={
                                            s.params.coupon ||
                                            offer?.coupon_code ||
                                            undefined
                                        }
                                    />
                                </div>
                            </div>
                        ))}

                    {step === 3 && (
                        <FieldGroup>
                            <FieldSet>
                                <FieldLegend variant="label">
                                    {t('campaigns.wizard.sendTo')}
                                </FieldLegend>
                                <RadioCards
                                    aria-label={t(
                                        'campaigns.wizard.audienceType',
                                    )}
                                    className="grid-cols-2 sm:grid-cols-5"
                                    value={s.audienceKind}
                                    onValueChange={(v) =>
                                        setS((p) => ({
                                            ...p,
                                            audienceKind: v as AudienceKind,
                                            audienceId: '',
                                        }))
                                    }
                                >
                                    {(
                                        [
                                            ['segment', data.segments],
                                            ['group', data.groups],
                                            ['tag', data.tags],
                                            ['all', []],
                                            ['previous', data.finished],
                                        ] as const
                                    ).map(([kind, list]) => (
                                        <RadioCard
                                            key={kind}
                                            value={kind}
                                            disabled={list === null}
                                        >
                                            {t(
                                                `campaigns.wizard.kinds.${kind}`,
                                            )}
                                        </RadioCard>
                                    ))}
                                </RadioCards>
                            </FieldSet>
                            {s.audienceKind === 'previous' && (
                                <PreviousAudience
                                    campaigns={data.finished ?? []}
                                    campaignId={s.audienceId}
                                    classes={s.prevClasses}
                                    onCampaign={(v) => set('audienceId', v)}
                                    onClasses={(v) => set('prevClasses', v)}
                                    summary={
                                        preflightKey && template
                                            ? preflight
                                            : null
                                    }
                                    error={
                                        errors.audience ??
                                        errors['audience.previous_campaign']
                                    }
                                />
                            )}
                            {s.audienceKind !== 'all' &&
                                s.audienceKind !== 'previous' && (
                                    <AudiencePicker
                                        kind={s.audienceKind}
                                        value={s.audienceId}
                                        error={
                                            errors.audience ??
                                            errors[
                                                `audience.${s.audienceKind}_id`
                                            ]
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
                            {s.audienceKind !== 'previous' && (
                                <EstimateLine
                                    estimate={estimate}
                                    kind={s.audienceKind}
                                    chosen={
                                        s.audienceKind === 'all' ||
                                        !!s.audienceId
                                    }
                                />
                            )}
                        </FieldGroup>
                    )}

                    {step === 4 && (
                        <div className="space-y-6">
                            <dl className="grid gap-x-6 gap-y-2 text-sm sm:grid-cols-[10rem_1fr]">
                                <dt className="text-muted-foreground">
                                    {t('campaigns.wizard.name')}
                                </dt>
                                <dd>{s.name}</dd>
                                <dt className="text-muted-foreground">
                                    {t('campaigns.wizard.sender')}
                                </dt>
                                <dd>
                                    {sender
                                        ? `${sender.display_phone_number}${sender.verified_name ? ` · ${sender.verified_name}` : ''}`
                                        : t('campaigns.wizard.defaultNumber')}
                                </dd>
                                <dt className="text-muted-foreground">
                                    {t('campaigns.wizard.template')}
                                </dt>
                                <dd className="font-mono">
                                    {template
                                        ? `${template.name} (${template.language})`
                                        : '—'}
                                </dd>
                                {shape &&
                                    [
                                        ...(isMediaHeader(shape.header)
                                            ? [
                                                  [
                                                      t('params.media.title', {
                                                          kind: t(
                                                              `params.media.${shape.header}`,
                                                          ).toLowerCase(),
                                                      }),
                                                      s.params.media?.link ??
                                                          s.params.media
                                                              ?.filename ??
                                                          (s.params.media?.id
                                                              ? t(
                                                                    'params.media.uploaded',
                                                                )
                                                              : '—'),
                                                  ],
                                              ]
                                            : []),
                                        ...shape.headerVars.map((v) => [
                                            t('params.header', { name: v }),
                                            describe(s.params.header[v]),
                                        ]),
                                        ...shape.bodyVars.map((v) => [
                                            t('params.body', { name: v }),
                                            describe(s.params.body[v]),
                                        ]),
                                        ...shape.urlButtons.map((p) => [
                                            t('params.button', { n: p + 1 }),
                                            describe(s.params.buttons[p]),
                                        ]),
                                    ].map(([k, v]) => (
                                        <div key={k} className="contents">
                                            <dt className="text-muted-foreground">
                                                {k}
                                            </dt>
                                            <dd className="break-all">{v}</dd>
                                        </div>
                                    ))}
                                <dt className="text-muted-foreground">
                                    {t('campaigns.wizard.audience')}
                                </dt>
                                <dd>
                                    {audienceLabel(t, audienceOf(s), names)}
                                    {typeof estimate === 'number' &&
                                        t('campaigns.wizard.aboutContacts', {
                                            count: estimate.toLocaleString(
                                                NUMBER_LOCALE,
                                            ),
                                        })}
                                </dd>
                            </dl>

                            {preflightKey && (
                                <PreflightPanel
                                    result={preflight}
                                    canManageSettings={data.canConsentSettings}
                                />
                            )}

                            <Field className="max-w-xs">
                                <FieldLabel htmlFor="cw-priority">
                                    {t('campaigns.engine.priority')}
                                </FieldLabel>
                                <Select
                                    value={s.priority}
                                    onValueChange={(v) =>
                                        set('priority', v as Priority)
                                    }
                                >
                                    <SelectTrigger
                                        id="cw-priority"
                                        className="w-full"
                                        data-testid="wizard-priority"
                                    >
                                        <SelectValue />
                                    </SelectTrigger>
                                    <SelectContent>
                                        {PRIORITIES.map((p) => (
                                            <SelectItem key={p} value={p}>
                                                {priorityLabel(t, p)}
                                            </SelectItem>
                                        ))}
                                    </SelectContent>
                                </Select>
                                <FieldDescription>
                                    {t('campaigns.engine.priorityHint')}
                                </FieldDescription>
                            </Field>

                            <details className="rounded-lg border p-3">
                                <summary className="cursor-pointer text-sm font-medium">
                                    {t('campaigns.wizard.advanced')}
                                </summary>
                                <Field
                                    className="mt-3 max-w-xs"
                                    data-invalid={!!errors.window_size}
                                >
                                    <FieldLabel htmlFor="cw-window">
                                        {t('campaigns.wizard.inFlight')}
                                    </FieldLabel>
                                    <NumberField
                                        id="cw-window"
                                        min={0}
                                        max={10000}
                                        step={1}
                                        allowEmpty
                                        placeholder={t(
                                            'campaigns.wizard.automatic',
                                        )}
                                        aria-invalid={
                                            errors.window_size
                                                ? true
                                                : undefined
                                        }
                                        value={
                                            s.windowSize === ''
                                                ? null
                                                : Number(s.windowSize)
                                        }
                                        onValueChange={(v) =>
                                            set(
                                                'windowSize',
                                                v === null ? '' : String(v),
                                            )
                                        }
                                    />
                                    {err('window_size') ?? (
                                        <FieldDescription>
                                            {t('campaigns.wizard.inFlightHint')}
                                        </FieldDescription>
                                    )}
                                </Field>
                                <Field
                                    className="mt-3 max-w-xs"
                                    data-invalid={!!errors.max_mps}
                                >
                                    <FieldLabel htmlFor="cw-maxmps">
                                        {t('campaigns.engine.maxMps')}
                                    </FieldLabel>
                                    <NumberField
                                        id="cw-maxmps"
                                        min={0}
                                        max={1000}
                                        step={0.1}
                                        allowEmpty
                                        placeholder={t(
                                            'campaigns.engine.noCap',
                                        )}
                                        value={
                                            s.maxMps === ''
                                                ? null
                                                : Number(s.maxMps)
                                        }
                                        onValueChange={(v) =>
                                            set(
                                                'maxMps',
                                                v === null || v === 0
                                                    ? ''
                                                    : String(v),
                                            )
                                        }
                                    />
                                    {err('max_mps') ?? (
                                        <FieldDescription>
                                            {t('campaigns.engine.maxMpsHint')}
                                        </FieldDescription>
                                    )}
                                </Field>
                            </details>

                            {data.canStart ? (
                                <FieldSet>
                                    <FieldLegend variant="label">
                                        {t('campaigns.wizard.when')}
                                    </FieldLegend>
                                    <Segmented
                                        aria-label={t('campaigns.wizard.when')}
                                        className="self-start"
                                        value={s.when}
                                        onValueChange={(v) => set('when', v)}
                                        options={[
                                            {
                                                value: 'now',
                                                label: t(
                                                    'campaigns.wizard.startNow',
                                                ),
                                            },
                                            {
                                                value: 'schedule',
                                                label: t(
                                                    'campaigns.wizard.schedule',
                                                ),
                                            },
                                        ]}
                                    />
                                    {s.when === 'schedule' && (
                                        <Field
                                            className="max-w-xs"
                                            data-invalid={!!errors.scheduled_at}
                                        >
                                            <FieldLabel htmlFor="cw-at">
                                                {t(
                                                    'campaigns.wizard.dateTime',
                                                    {
                                                        zone: data.timezone,
                                                    },
                                                )}
                                            </FieldLabel>
                                            <DateTimePicker
                                                id="cw-at"
                                                value={s.scheduledAt}
                                                onValueChange={(v) =>
                                                    set('scheduledAt', v)
                                                }
                                            />
                                            {err('scheduled_at') ?? (
                                                <FieldDescription>
                                                    {t(
                                                        'campaigns.wizard.profileZone',
                                                    )}
                                                    {(() => {
                                                        const at = zonedToUtc(
                                                            s.scheduledAt,
                                                            data.timezone,
                                                        );
                                                        return at
                                                            ? t(
                                                                  'campaigns.wizard.startsHere',
                                                                  {
                                                                      date: formatInZone(
                                                                          at,
                                                                          Intl.DateTimeFormat().resolvedOptions()
                                                                              .timeZone,
                                                                      ),
                                                                  },
                                                              )
                                                            : '';
                                                    })()}
                                                </FieldDescription>
                                            )}
                                        </Field>
                                    )}
                                </FieldSet>
                            ) : (
                                <p className="text-sm text-muted-foreground">
                                    {t('campaigns.wizard.cannotStart')}
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
                    {t('campaigns.wizard.back')}
                </Button>
                <div className="flex flex-wrap gap-2">
                    {step < STEPS.length - 1 ? (
                        <Button type="button" onClick={next} disabled={pending}>
                            {t('campaigns.wizard.next')}
                        </Button>
                    ) : (
                        <>
                            <Button
                                type="button"
                                variant="outline"
                                onClick={saveDraft}
                                disabled={pending}
                            >
                                {pending && <Spinner />}{' '}
                                {t('campaigns.wizard.saveDraft')}
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
                                        ? t('campaigns.wizard.scheduleCampaign')
                                        : t('campaigns.wizard.startCampaign')}
                                </Button>
                            )}
                        </>
                    )}
                </div>
            </div>

            <AlertDialog open={confirmOpen} onOpenChange={setConfirmOpen}>
                <AlertDialogContent>
                    <AlertDialogHeader>
                        <AlertDialogTitle>
                            {t('campaigns.wizard.confirmTitle')}
                        </AlertDialogTitle>
                        <AlertDialogDescription>
                            {typeof eligible === 'number'
                                ? t('consent.preflight.confirmEligible', {
                                      count: eligible,
                                      n: eligible.toLocaleString(NUMBER_LOCALE),
                                      template: template?.name ?? '',
                                  })
                                : typeof estimate === 'number'
                                  ? t('campaigns.wizard.confirmAbout', {
                                        count: estimate.toLocaleString(
                                            NUMBER_LOCALE,
                                        ),
                                        template: template?.name ?? '',
                                    })
                                  : t('campaigns.wizard.confirmContacts', {
                                        template: template?.name ?? '',
                                    })}
                            {t('campaigns.wizard.confirmTail')}
                        </AlertDialogDescription>
                    </AlertDialogHeader>
                    <AlertDialogFooter>
                        <AlertDialogCancel disabled={pending}>
                            {t('campaigns.wizard.cancel')}
                        </AlertDialogCancel>
                        <AlertDialogAction
                            disabled={pending}
                            onClick={(e) => {
                                e.preventDefault();
                                start();
                            }}
                        >
                            {pending && <Spinner />}{' '}
                            {t('campaigns.wizard.startNow')}
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
    const t = useT();
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
                    ? t('campaigns.wizard.counting')
                    : t('campaigns.wizard.countUnavailable')
                : estimate === null
                  ? t('campaigns.wizard.countUnavailable')
                  : t(
                        kind === 'all'
                            ? 'campaigns.wizard.estimateAll'
                            : 'campaigns.wizard.estimate',
                        { count: estimate.toLocaleString(NUMBER_LOCALE) },
                    )}
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
    kind: Exclude<AudienceKind, 'all' | 'previous'>;
    value: string;
    options: { id: string; name: string }[];
    error?: string;
    onChange: (v: string) => void;
}) {
    const t = useT();
    const label = t(`campaigns.wizard.pickers.${kind}`);
    return (
        <Field data-invalid={!!error}>
            <FieldLabel htmlFor="cw-audience">{label}</FieldLabel>
            {options.length === 0 ? (
                <p className="text-sm text-muted-foreground">
                    {t(`campaigns.wizard.none.${kind}`)}{' '}
                    <Link href="/contacts" className="underline">
                        {t('campaigns.wizard.manageContacts')}
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
                            placeholder={t(`campaigns.wizard.choose.${kind}`)}
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
