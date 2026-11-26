'use client';

import { useMemo, useState, useTransition } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { ArrowLeft, Lock, Plus, Trash2, TriangleAlert } from 'lucide-react';
import { useConfirm } from '@/components/confirm';
import { notify } from '@/components/notify';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Input } from '@/components/ui/input';
import { NumberField } from '@/components/ui/number-field';
import {
    Select,
    SelectContent,
    SelectGroup,
    SelectItem,
    SelectLabel,
    SelectTrigger,
    SelectValue,
} from '@/components/ui/select';
import { Spinner } from '@/components/ui/spinner';
import { Switch } from '@/components/ui/switch';
import { Textarea } from '@/components/ui/textarea';
import { useLocale, useT } from '@/i18n/client';
import { intlLocale } from '@/i18n/config';
import { estimateTokens, parseIntents } from '@/lib/ai';
import type { Schemas } from '@/lib/types';
import { Row, Section } from '../settings/section';
import { ProviderModelPicker } from '../settings/ai/model-picker';
import { deleteProfileAction, saveProfileAction } from './actions';
import type { EditorData } from './editor-data';
import { TestConsole } from './test-console';

type Profile = Schemas['AIProfile'];
type ProfileInput = Schemas['AIProfileInput'];
type Handoff = Schemas['AIHandoff'];
type Data = Schemas['AIDataCollection'];

const NONE = '__none__';
const ACTION_KINDS = [
    'create_contact',
    'update_contact_fields',
    'add_tag',
    'assign_conversation',
    'add_internal_note',
] as const;
const CONTACT_FIELDS = [
    'name',
    'email',
    'var1',
    'var2',
    'var3',
    'var4',
    'var5',
];

function fromProfile(p: Profile | null): Required<ProfileInput> {
    return {
        name: p?.name ?? '',
        phone_number_id: p?.phone_number_id ?? null,
        system_prompt: p?.system_prompt ?? '',
        provider_id: p?.provider_id ?? null,
        model: p?.model ?? null,
        auto_reply: p?.auto_reply ?? false,
        handoff: p?.handoff ?? {
            on_request: true,
            on_low_confidence: null,
            on_no_answer: true,
            on_critical_intents: [],
            on_unauthorized_action: true,
        },
        data_collection: p?.data_collection ?? {
            enabled: false,
            fields: [],
            may_create_contact: false,
            may_update_fields: false,
        },
        allowed_actions: p?.allowed_actions ?? [],
        disclosure_mode: p?.disclosure_mode ?? 'always_first_message',
        status: p?.status ?? 'active',
    };
}

/** Full-page AI profile editor with the test console beside it. */
export function ProfileEditor({
    profile,
    data,
    canManage,
}: {
    profile: Profile | null;
    data: EditorData;
    canManage: boolean;
}) {
    const t = useT();
    const router = useRouter();
    const ask = useConfirm();
    const locale = intlLocale(useLocale());
    const [v, setV] = useState(() => fromProfile(profile));
    const [intents, setIntents] = useState(
        v.handoff.on_critical_intents.join(', '),
    );
    const [errors, setErrors] = useState<Record<string, string>>({});
    const [saving, start] = useTransition();
    const readOnly = !canManage;
    const saved = useMemo(() => fromProfile(profile), [profile]);
    const dirty =
        JSON.stringify({
            ...v,
            handoff: { ...v.handoff, on_critical_intents: [] },
        }) !==
            JSON.stringify({
                ...saved,
                handoff: { ...saved.handoff, on_critical_intents: [] },
            }) || intents !== saved.handoff.on_critical_intents.join(', ');

    const setHandoff = (h: Partial<Handoff>) =>
        setV({ ...v, handoff: { ...v.handoff, ...h } });
    const setData = (d: Partial<Data>) =>
        setV({ ...v, data_collection: { ...v.data_collection, ...d } });
    const toggleAction = (a: string, on: boolean) =>
        setV({
            ...v,
            allowed_actions: on
                ? [...v.allowed_actions, a]
                : v.allowed_actions.filter((x) => x !== a),
        });

    const save = () =>
        start(async () => {
            const body: ProfileInput = {
                ...v,
                handoff: {
                    ...v.handoff,
                    on_request: true,
                    on_critical_intents: parseIntents(intents),
                },
            };
            const r = await saveProfileAction(profile?.id ?? null, body);
            if (notify(r)) {
                setErrors({});
                if (!profile) router.push(`/ai-profiles/${r.data.id}`);
                else router.refresh();
            } else if (!r.ok) setErrors(r.fieldErrors ?? {});
        });

    const remove = async () => {
        if (!profile) return;
        const ok = await ask({
            title: t('ai.profiles.deleteTitle', { name: profile.name }),
            description: t('ai.profiles.deleteBody'),
            confirmLabel: t('ai.profiles.delete'),
            destructive: true,
        });
        if (ok)
            start(async () => {
                if (notify(await deleteProfileAction(profile.id)))
                    router.push('/ai-profiles');
            });
    };

    const err = (k: string) =>
        errors[k] ? (
            <p className="text-xs text-destructive">{errors[k]}</p>
        ) : null;
    const usedKeys = new Set(v.data_collection.fields.map((f) => f.key));
    const fieldChoices = [
        ...CONTACT_FIELDS.map((k) => ({ key: k, group: 'contact' })),
        ...data.attributeKeys.map((k) => ({ key: `attr:${k}`, group: 'attr' })),
    ];
    const nextField = fieldChoices.find((f) => !usedKeys.has(f.key));
    const tokens = estimateTokens(v.system_prompt);

    return (
        <div className="space-y-4">
            <div className="flex flex-wrap items-center justify-between gap-2">
                <Button asChild variant="ghost" size="sm">
                    <Link href="/ai-profiles">
                        <ArrowLeft /> {t('ai.profiles.back')}
                    </Link>
                </Button>
                {canManage && (
                    <div className="flex gap-2">
                        {profile && (
                            <Button
                                variant="outline"
                                onClick={() => void remove()}
                                disabled={saving}
                            >
                                <Trash2 /> {t('ai.profiles.delete')}
                            </Button>
                        )}
                        <Button
                            onClick={save}
                            disabled={saving || (!!profile && !dirty)}
                            data-testid="ai-profile-save"
                        >
                            {saving && <Spinner />}
                            {t('ai.profiles.save')}
                        </Button>
                    </div>
                )}
            </div>
            {!data.settings.enabled && (
                <Alert>
                    <AlertDescription>
                        {t('ai.profiles.aiOff')}
                    </AlertDescription>
                </Alert>
            )}
            <div className="grid items-start gap-6 xl:grid-cols-[minmax(0,1fr)_26rem]">
                <div className="min-w-0 space-y-6">
                    <Section title={t('ai.profiles.name')}>
                        <div className="grid gap-4 sm:grid-cols-2">
                            <Row
                                label={t('ai.profiles.name')}
                                htmlFor="ai-profile-name"
                            >
                                <Input
                                    id="ai-profile-name"
                                    value={v.name}
                                    maxLength={120}
                                    disabled={readOnly}
                                    onChange={(e) =>
                                        setV({ ...v, name: e.target.value })
                                    }
                                />
                                {err('name')}
                            </Row>
                            <Row
                                label={t('ai.profiles.number')}
                                hint={t('ai.profiles.numberHint')}
                                htmlFor="ai-profile-number"
                            >
                                <Select
                                    value={v.phone_number_id ?? NONE}
                                    disabled={readOnly}
                                    onValueChange={(x) =>
                                        setV({
                                            ...v,
                                            phone_number_id:
                                                x === NONE ? null : x,
                                        })
                                    }
                                >
                                    <SelectTrigger
                                        id="ai-profile-number"
                                        className="w-full"
                                    >
                                        <SelectValue />
                                    </SelectTrigger>
                                    <SelectContent>
                                        <SelectItem value={NONE}>
                                            {t('ai.profiles.noNumber')}
                                        </SelectItem>
                                        {data.numbers.map((n) => (
                                            <SelectItem key={n.id} value={n.id}>
                                                {n.display_phone_number}
                                                {n.verified_name &&
                                                    ` · ${n.verified_name}`}
                                            </SelectItem>
                                        ))}
                                    </SelectContent>
                                </Select>
                                {err('phone_number_id')}
                            </Row>
                        </div>
                        <Row
                            label={t('ai.profiles.model')}
                            hint={
                                v.provider_id
                                    ? undefined
                                    : t('ai.settings.defaultModelHint')
                            }
                        >
                            <ProviderModelPicker
                                idPrefix="ai-profile"
                                providers={data.providers}
                                models={data.models}
                                providerId={v.provider_id}
                                model={v.model ?? ''}
                                noneLabel={t('ai.profiles.companyDefault')}
                                disabled={readOnly}
                                onChange={(p, m) =>
                                    setV({
                                        ...v,
                                        provider_id: p,
                                        model: m ? m : null,
                                    })
                                }
                            />
                            {err('provider_id') ?? err('model')}
                        </Row>
                        <div className="grid gap-4 sm:grid-cols-2">
                            <label className="flex items-start justify-between gap-4">
                                <span>
                                    <span className="block text-sm font-medium">
                                        {t('ai.profiles.autoReply')}
                                    </span>
                                    <span className="block text-xs text-muted-foreground">
                                        {t('ai.profiles.autoReplyHint')}
                                    </span>
                                </span>
                                <Switch
                                    checked={v.auto_reply}
                                    disabled={readOnly}
                                    onCheckedChange={(c) =>
                                        setV({ ...v, auto_reply: c })
                                    }
                                    aria-label={t('ai.profiles.autoReply')}
                                />
                            </label>
                            <Row label={t('ai.profiles.profileStatus')}>
                                <Select
                                    value={v.status}
                                    disabled={readOnly}
                                    onValueChange={(x) =>
                                        setV({
                                            ...v,
                                            status: x as Required<ProfileInput>['status'],
                                        })
                                    }
                                >
                                    <SelectTrigger
                                        className="w-full"
                                        aria-label={t(
                                            'ai.profiles.profileStatus',
                                        )}
                                    >
                                        <SelectValue />
                                    </SelectTrigger>
                                    <SelectContent>
                                        <SelectItem value="active">
                                            {t('ai.profiles.status.active')}
                                        </SelectItem>
                                        <SelectItem value="paused">
                                            {t('ai.profiles.status.paused')}
                                        </SelectItem>
                                    </SelectContent>
                                </Select>
                            </Row>
                        </div>
                    </Section>

                    <Section
                        title={t('ai.profiles.systemPrompt')}
                        description={t('ai.profiles.systemPromptHint')}
                    >
                        <Textarea
                            id="ai-profile-prompt"
                            aria-label={t('ai.profiles.systemPrompt')}
                            value={v.system_prompt}
                            maxLength={100000}
                            disabled={readOnly}
                            onChange={(e) =>
                                setV({ ...v, system_prompt: e.target.value })
                            }
                            className="min-h-96 font-mono text-sm leading-relaxed"
                        />
                        <div className="flex justify-between text-xs text-muted-foreground">
                            <span>{err('system_prompt')}</span>
                            <span data-testid="ai-prompt-tokens">
                                {t('ai.profiles.tokens', {
                                    count: tokens.toLocaleString(locale),
                                })}{' '}
                                ·{' '}
                                {v.system_prompt.length.toLocaleString(locale)}{' '}
                                / 100.000
                            </span>
                        </div>
                    </Section>

                    <Section title={t('ai.profiles.handoff')}>
                        <div className="flex items-start justify-between gap-4 rounded-lg border bg-muted/30 p-3">
                            <span>
                                <span className="flex items-center gap-2 text-sm font-medium">
                                    <Lock className="size-3.5" />
                                    {t('ai.profiles.onRequest')}
                                </span>
                                <span className="block text-xs text-muted-foreground">
                                    {t('ai.profiles.onRequestLocked')}
                                </span>
                            </span>
                            <Switch
                                checked
                                disabled
                                aria-label={t('ai.profiles.onRequest')}
                                data-testid="ai-handoff-on-request"
                            />
                        </div>
                        <Row label={t('ai.profiles.onLowConfidence')}>
                            <div className="flex items-center gap-3">
                                <Switch
                                    checked={
                                        v.handoff.on_low_confidence != null
                                    }
                                    disabled={readOnly}
                                    aria-label={t(
                                        'ai.profiles.onLowConfidence',
                                    )}
                                    onCheckedChange={(c) =>
                                        setHandoff({
                                            on_low_confidence: c ? 0.6 : null,
                                        })
                                    }
                                />
                                {v.handoff.on_low_confidence != null ? (
                                    <NumberField
                                        min={0}
                                        max={100}
                                        className="w-32"
                                        disabled={readOnly}
                                        aria-label={t(
                                            'ai.profiles.onLowConfidence',
                                        )}
                                        value={Math.round(
                                            v.handoff.on_low_confidence * 100,
                                        )}
                                        onValueChange={(n) =>
                                            setHandoff({
                                                on_low_confidence:
                                                    Math.min(
                                                        100,
                                                        Math.max(0, n ?? 0),
                                                    ) / 100,
                                            })
                                        }
                                    />
                                ) : (
                                    <span className="text-sm text-muted-foreground">
                                        {t('ai.profiles.onLowConfidenceOff')}
                                    </span>
                                )}
                            </div>
                        </Row>
                        {(
                            [
                                ['on_no_answer', 'ai.profiles.onNoAnswer'],
                                [
                                    'on_unauthorized_action',
                                    'ai.profiles.onUnauthorized',
                                ],
                            ] as const
                        ).map(([k, label]) => (
                            <label
                                key={k}
                                className="flex items-center justify-between gap-4 text-sm"
                            >
                                {t(label)}
                                <Switch
                                    checked={v.handoff[k]}
                                    disabled={readOnly}
                                    onCheckedChange={(c) =>
                                        setHandoff({ [k]: c })
                                    }
                                    aria-label={t(label)}
                                />
                            </label>
                        ))}
                        <Row
                            label={t('ai.profiles.criticalIntents')}
                            hint={t('ai.profiles.criticalIntentsHint')}
                            htmlFor="ai-profile-intents"
                        >
                            <Input
                                id="ai-profile-intents"
                                value={intents}
                                disabled={readOnly}
                                onChange={(e) => setIntents(e.target.value)}
                                className="font-mono text-sm"
                            />
                            {err('handoff.on_critical_intents')}
                        </Row>
                        <Row
                            label={t('ai.profiles.onHumanTakeover')}
                            hint={t('ai.profiles.onHumanTakeoverHint')}
                        >
                            <Select
                                value={v.handoff.on_human_takeover ?? 'off'}
                                disabled={readOnly}
                                onValueChange={(x) =>
                                    setHandoff({
                                        on_human_takeover: x as
                                            'off' | 'suggest',
                                    })
                                }
                            >
                                <SelectTrigger
                                    className="w-64"
                                    aria-label={t(
                                        'ai.profiles.onHumanTakeover',
                                    )}
                                >
                                    <SelectValue />
                                </SelectTrigger>
                                <SelectContent>
                                    <SelectItem value="off">
                                        {t('ai.profiles.takeoverOff')}
                                    </SelectItem>
                                    <SelectItem value="suggest">
                                        {t('ai.profiles.takeoverSuggest')}
                                    </SelectItem>
                                </SelectContent>
                            </Select>
                        </Row>
                    </Section>

                    <Section
                        title={t('ai.profiles.dataCollection')}
                        description={t('ai.profiles.dataHint')}
                    >
                        <label className="flex items-center justify-between gap-4 text-sm font-medium">
                            {t('ai.profiles.dataEnabled')}
                            <Switch
                                checked={v.data_collection.enabled}
                                disabled={readOnly}
                                onCheckedChange={(c) => setData({ enabled: c })}
                                aria-label={t('ai.profiles.dataEnabled')}
                            />
                        </label>
                        {v.data_collection.enabled && (
                            <div className="space-y-3">
                                {v.data_collection.fields.map((f, i) => (
                                    <div
                                        key={i}
                                        className="grid gap-2 sm:grid-cols-[12rem_1fr_auto_auto] sm:items-center"
                                    >
                                        <Select
                                            value={f.key}
                                            disabled={readOnly}
                                            onValueChange={(key) =>
                                                setData({
                                                    fields: v.data_collection.fields.map(
                                                        (x, j) =>
                                                            j === i
                                                                ? { ...x, key }
                                                                : x,
                                                    ),
                                                })
                                            }
                                        >
                                            <SelectTrigger
                                                className="w-full font-mono text-xs"
                                                aria-label={t(
                                                    'ai.profiles.field',
                                                )}
                                            >
                                                <SelectValue />
                                            </SelectTrigger>
                                            <SelectContent>
                                                <SelectGroup>
                                                    <SelectLabel>
                                                        {t(
                                                            'ai.profiles.contactFields',
                                                        )}
                                                    </SelectLabel>
                                                    {CONTACT_FIELDS.map((k) => (
                                                        <SelectItem
                                                            key={k}
                                                            value={k}
                                                            disabled={
                                                                usedKeys.has(
                                                                    k,
                                                                ) && k !== f.key
                                                            }
                                                        >
                                                            {k}
                                                        </SelectItem>
                                                    ))}
                                                </SelectGroup>
                                                {data.attributeKeys.length >
                                                    0 && (
                                                    <SelectGroup>
                                                        <SelectLabel>
                                                            {t(
                                                                'ai.profiles.customAttributes',
                                                            )}
                                                        </SelectLabel>
                                                        {data.attributeKeys.map(
                                                            (k) => (
                                                                <SelectItem
                                                                    key={k}
                                                                    value={`attr:${k}`}
                                                                    disabled={
                                                                        usedKeys.has(
                                                                            `attr:${k}`,
                                                                        ) &&
                                                                        `attr:${k}` !==
                                                                            f.key
                                                                    }
                                                                >
                                                                    {k}
                                                                </SelectItem>
                                                            ),
                                                        )}
                                                    </SelectGroup>
                                                )}
                                            </SelectContent>
                                        </Select>
                                        <Input
                                            value={f.label}
                                            maxLength={120}
                                            disabled={readOnly}
                                            placeholder={t(
                                                'ai.profiles.fieldLabel',
                                            )}
                                            aria-label={t(
                                                'ai.profiles.fieldLabel',
                                            )}
                                            onChange={(e) =>
                                                setData({
                                                    fields: v.data_collection.fields.map(
                                                        (x, j) =>
                                                            j === i
                                                                ? {
                                                                      ...x,
                                                                      label: e
                                                                          .target
                                                                          .value,
                                                                  }
                                                                : x,
                                                    ),
                                                })
                                            }
                                        />
                                        <label className="flex items-center gap-2 text-sm">
                                            <Checkbox
                                                checked={f.required}
                                                disabled={readOnly}
                                                onCheckedChange={(c) =>
                                                    setData({
                                                        fields: v.data_collection.fields.map(
                                                            (x, j) =>
                                                                j === i
                                                                    ? {
                                                                          ...x,
                                                                          required:
                                                                              c ===
                                                                              true,
                                                                      }
                                                                    : x,
                                                        ),
                                                    })
                                                }
                                            />
                                            {t('ai.profiles.required')}
                                        </label>
                                        <Button
                                            type="button"
                                            variant="ghost"
                                            size="icon"
                                            disabled={readOnly}
                                            aria-label={t(
                                                'ai.profiles.removeField',
                                            )}
                                            onClick={() =>
                                                setData({
                                                    fields: v.data_collection.fields.filter(
                                                        (_, j) => j !== i,
                                                    ),
                                                })
                                            }
                                        >
                                            <Trash2 />
                                        </Button>
                                    </div>
                                ))}
                                {err('data_collection.fields')}
                                {!readOnly && nextField && (
                                    <Button
                                        type="button"
                                        variant="outline"
                                        size="sm"
                                        onClick={() =>
                                            setData({
                                                fields: [
                                                    ...v.data_collection.fields,
                                                    {
                                                        key: nextField.key,
                                                        label: '',
                                                        required: false,
                                                    },
                                                ],
                                            })
                                        }
                                    >
                                        <Plus /> {t('ai.profiles.addField')}
                                    </Button>
                                )}
                                {(
                                    [
                                        [
                                            'may_create_contact',
                                            'ai.profiles.mayCreate',
                                        ],
                                        [
                                            'may_update_fields',
                                            'ai.profiles.mayUpdate',
                                        ],
                                    ] as const
                                ).map(([k, label]) => (
                                    <label
                                        key={k}
                                        className="flex items-center justify-between gap-4 text-sm"
                                    >
                                        {t(label)}
                                        <Switch
                                            checked={v.data_collection[k]}
                                            disabled={readOnly}
                                            onCheckedChange={(c) =>
                                                setData({ [k]: c })
                                            }
                                            aria-label={t(label)}
                                        />
                                    </label>
                                ))}
                            </div>
                        )}
                    </Section>

                    <Section
                        title={t('ai.profiles.actions')}
                        description={t('ai.profiles.actionsHint')}
                    >
                        <div className="grid gap-2 sm:grid-cols-2">
                            {ACTION_KINDS.map((a) => (
                                <label
                                    key={a}
                                    className="flex items-center gap-2 text-sm"
                                >
                                    <Checkbox
                                        checked={v.allowed_actions.includes(a)}
                                        disabled={readOnly}
                                        onCheckedChange={(c) =>
                                            toggleAction(a, c === true)
                                        }
                                    />
                                    {t(`ai.profiles.actionKinds.${a}`)}
                                </label>
                            ))}
                        </div>
                        {data.automations.length > 0 && (
                            <Row label={t('ai.profiles.automations')}>
                                <div className="grid gap-2 sm:grid-cols-2">
                                    {data.automations.map((a) => (
                                        <label
                                            key={a.id}
                                            className="flex items-center gap-2 text-sm"
                                        >
                                            <Checkbox
                                                checked={v.allowed_actions.includes(
                                                    `automation:${a.id}`,
                                                )}
                                                disabled={readOnly}
                                                onCheckedChange={(c) =>
                                                    toggleAction(
                                                        `automation:${a.id}`,
                                                        c === true,
                                                    )
                                                }
                                            />
                                            {a.name}
                                        </label>
                                    ))}
                                </div>
                            </Row>
                        )}
                        {err('allowed_actions')}
                    </Section>

                    <Section title={t('ai.profiles.disclosure')}>
                        <Select
                            value={v.disclosure_mode}
                            disabled={readOnly}
                            onValueChange={(x) =>
                                setV({
                                    ...v,
                                    disclosure_mode:
                                        x as Required<ProfileInput>['disclosure_mode'],
                                })
                            }
                        >
                            <SelectTrigger
                                className="w-full sm:w-80"
                                aria-label={t('ai.profiles.disclosure')}
                            >
                                <SelectValue />
                            </SelectTrigger>
                            <SelectContent>
                                {(
                                    [
                                        'always_first_message',
                                        'on_question',
                                        'off',
                                    ] as const
                                ).map((m) => (
                                    <SelectItem key={m} value={m}>
                                        {t(`ai.profiles.disclosureModes.${m}`)}
                                    </SelectItem>
                                ))}
                            </SelectContent>
                        </Select>
                        {v.disclosure_mode === 'off' && (
                            <Alert variant="destructive">
                                <TriangleAlert />
                                <AlertDescription>
                                    {t('ai.profiles.warnings.disclosure_off')}
                                </AlertDescription>
                            </Alert>
                        )}
                    </Section>
                </div>
                <div className="xl:sticky xl:top-4">
                    <TestConsole
                        profileId={profile?.id ?? null}
                        canRun={canManage}
                        dirty={dirty}
                    />
                </div>
            </div>
        </div>
    );
}
