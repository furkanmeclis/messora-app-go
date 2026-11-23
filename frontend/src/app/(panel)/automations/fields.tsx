'use client';

import { Plus, Trash2 } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Field, FieldLabel } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { useT } from '@/i18n/client';
import type { AutomationTriggerType } from '@/lib/types';
import type { EditorOptions } from './data';
import {
    DurationInput,
    KeywordsInput,
    PlaceholderText,
    SelectField,
    SuggestInput,
} from './inputs';
import {
    CONDITION_FIELDS,
    CONTACT_FIELDS,
    conditionFieldLabel,
    contactFieldLabel,
    type EditorStep,
    HTTP_METHODS,
    MATCH_MODES,
    MAX_DELAY_SECONDS,
    MAX_KEYWORDS,
    MAX_MUTE_SECONDS,
    MESSAGE_STATUSES,
    MIN_MUTE_SECONDS,
    mk,
    newKey,
    operatorLabel,
    operatorNeedsValue,
    operatorsFor,
    type StepError,
    type TemplateVars,
    templateVariables,
    TRIGGER_TYPES,
    type TriggerForm,
} from './model';

export type StepPatch = (
    fn: (s: EditorStep) => EditorStep,
    /** Field being edited, so typing becomes one undo step. */
    group?: string,
) => void;

type Props = {
    step: EditorStep;
    edit: StepPatch;
    errors: StepError[];
    options: EditorOptions;
    readOnly: boolean;
};

function useErr(errors: StepError[]) {
    return (...fields: string[]) =>
        errors.find((e) => fields.includes(e.field))?.message;
}

/** The form of one step in the inspector. */
export function StepFields(props: Props) {
    const { step, edit, errors, options, readOnly } = props;
    const t = useT();
    const set = (patch: Partial<EditorStep>, group?: string) =>
        edit((s) => ({ ...s, ...patch }), group);
    const id = (f: string) => `${step.id}-${f}`;
    const err = useErr(errors);

    switch (step.type) {
        case 'reply':
            return (
                <PlaceholderText
                    id={id('text')}
                    label={t('automations.fields.message')}
                    multiline
                    maxLength={4096}
                    value={step.text ?? ''}
                    onChange={(text) => set({ text }, 'text')}
                    error={err('text')}
                    readOnly={readOnly}
                    hint={t('automations.fields.replyHint')}
                />
            );
        case 'send_template':
            return <TemplateFields {...props} />;
        case 'add_tag':
        case 'remove_tag':
            return (
                <SuggestInput
                    id={id('tag')}
                    label={t('automations.fields.tag')}
                    maxLength={60}
                    value={step.tag ?? ''}
                    onChange={(tag) => set({ tag }, 'tag')}
                    suggestions={options.tags}
                    error={err('tag')}
                    readOnly={readOnly}
                    hint={
                        step.type === 'add_tag'
                            ? t('automations.fields.addTagHint')
                            : undefined
                    }
                />
            );
        case 'add_to_group': {
            const opts = options.groups.map((g) => ({
                value: g.id,
                label: g.label,
            }));
            if (step.group_id && !opts.some((o) => o.value === step.group_id)) {
                opts.push({
                    value: step.group_id,
                    label: t('automations.fields.unknownGroup'),
                });
            }
            return (
                <SelectField
                    id={id('group')}
                    label={t('automations.fields.group')}
                    value={step.group_id ?? ''}
                    onChange={(group_id) => set({ group_id })}
                    options={opts}
                    disabled={readOnly}
                    placeholder={
                        opts.length
                            ? t('automations.fields.chooseGroup')
                            : t('automations.fields.noGroups')
                    }
                    error={err('group_id')}
                />
            );
        }
        case 'set_field':
            return (
                <div className="grid gap-4">
                    <SelectField
                        id={id('field')}
                        label={t('automations.fields.field')}
                        value={step.field ?? ''}
                        onChange={(field) => set({ field })}
                        disabled={readOnly}
                        options={CONTACT_FIELDS.map((f) => ({
                            value: f,
                            label: contactFieldLabel(f, t),
                        }))}
                        error={err('field')}
                    />
                    <PlaceholderText
                        id={id('value')}
                        label={t('automations.fields.value')}
                        maxLength={500}
                        value={step.value ?? ''}
                        onChange={(value) => set({ value }, 'value')}
                        error={err('value')}
                        readOnly={readOnly}
                        hint={t('automations.fields.valueHint')}
                    />
                </div>
            );
        case 'condition':
            return <ConditionFields {...props} />;
        case 'http_request':
            return <HttpFields {...props} />;
        case 'send_email':
            return (
                <div className="grid gap-4">
                    <PlaceholderText
                        id={id('to')}
                        label={t('automations.fields.to')}
                        value={step.to ?? ''}
                        onChange={(to) => set({ to }, 'to')}
                        placeholder={t('automations.fields.toPlaceholder')}
                        maxLength={254}
                        error={err('to', '')}
                        readOnly={readOnly}
                    />
                    <PlaceholderText
                        id={id('subject')}
                        label={t('automations.fields.subject')}
                        value={step.subject ?? ''}
                        onChange={(subject) => set({ subject }, 'subject')}
                        error={err('subject')}
                        readOnly={readOnly}
                    />
                    <PlaceholderText
                        id={id('body')}
                        label={t('automations.fields.body')}
                        multiline
                        value={step.body ?? ''}
                        onChange={(body) => set({ body }, 'body')}
                        error={err('body')}
                        readOnly={readOnly}
                        hint={t('automations.fields.emailHint')}
                    />
                </div>
            );
        case 'delay':
            return (
                <DurationInput
                    key={step.id}
                    id={id('seconds')}
                    label={t('automations.fields.waitFor')}
                    seconds={step.seconds ?? 0}
                    onChange={(seconds) => set({ seconds }, 'seconds')}
                    min={1}
                    max={MAX_DELAY_SECONDS}
                    disabled={readOnly}
                    error={err('seconds')}
                    hint={t('automations.fields.delayHint')}
                />
            );
        case 'mute':
            return (
                <DurationInput
                    key={step.id}
                    id={id('seconds')}
                    label={t('automations.fields.muteFor')}
                    seconds={step.seconds ?? 0}
                    onChange={(seconds) => set({ seconds }, 'seconds')}
                    min={MIN_MUTE_SECONDS}
                    max={MAX_MUTE_SECONDS}
                    disabled={readOnly}
                    error={err('seconds')}
                    hint={t('automations.fields.muteHint')}
                />
            );
        case 'assign': {
            const opts = [
                { value: 'none', label: t('automations.fields.unassign') },
            ];
            const users = options.users ?? [options.me];
            for (const u of users) {
                opts.push({
                    value: u.id,
                    label:
                        u.id === options.me.id
                            ? t('automations.fields.me', { name: u.label })
                            : u.label,
                });
            }
            if (
                step.assignee_id &&
                !opts.some((o) => o.value === step.assignee_id)
            ) {
                opts.push({
                    value: step.assignee_id,
                    label: t('automations.fields.anotherUser'),
                });
            }
            return (
                <SelectField
                    id={id('assignee')}
                    label={t('automations.fields.assignTo')}
                    value={step.assignee_id || 'none'}
                    onChange={(v) =>
                        set({ assignee_id: v === 'none' ? '' : v })
                    }
                    disabled={readOnly}
                    options={opts}
                    error={err('assignee_id')}
                    hint={
                        options.users
                            ? t('automations.fields.assignHint')
                            : t('automations.fields.assignHintLimited')
                    }
                />
            );
        }
        case 'close_conversation':
            return (
                <p className="text-sm text-muted-foreground">
                    {t('automations.fields.closeHint')}
                </p>
            );
        default:
            return (
                <div className="space-y-2 text-sm">
                    <p className="text-muted-foreground">
                        {t('automations.fields.unknownStep')}
                    </p>
                    <pre className="overflow-x-auto rounded-md bg-muted p-2 text-xs">
                        {JSON.stringify(step.raw, null, 2)}
                    </pre>
                </div>
            );
    }
}

function ConditionFields({ step, edit, errors, options, readOnly }: Props) {
    const t = useT();
    const err = useErr(errors);
    const cond = step.if ?? { field: 'message.body', operator: 'contains' };
    const setIf = (patch: Partial<typeof cond>, group?: string) =>
        edit((s) => ({ ...s, if: { ...(s.if ?? cond), ...patch } }), group);
    const id = (f: string) => `${step.id}-${f}`;
    const ops = operatorsFor(cond.field);
    return (
        <div className="grid gap-4">
            <SelectField
                id={id('if-field')}
                label={t('automations.fields.if')}
                value={cond.field}
                disabled={readOnly}
                onChange={(field) => {
                    const next = operatorsFor(field);
                    setIf({
                        field,
                        operator: next.includes(cond.operator)
                            ? cond.operator
                            : next[0]!,
                    });
                }}
                options={CONDITION_FIELDS.map((f) => ({
                    value: f.value,
                    label: conditionFieldLabel(f.value, t),
                }))}
                error={err('if.field', 'if')}
            />
            <SelectField
                id={id('if-operator')}
                label={t('automations.fields.operator')}
                value={cond.operator}
                disabled={readOnly}
                onChange={(operator) => setIf({ operator })}
                options={ops.map((o) => ({
                    value: o,
                    label: operatorLabel(cond.field, o, t),
                }))}
                error={err('if.operator')}
            />
            {operatorNeedsValue(cond.operator) &&
                (cond.field === 'contact.tag' ? (
                    <SuggestInput
                        id={id('if-value')}
                        label={t('automations.fields.tag')}
                        maxLength={500}
                        value={cond.value ?? ''}
                        onChange={(value) => setIf({ value }, 'if.value')}
                        suggestions={options.tags}
                        readOnly={readOnly}
                        error={err('if.value')}
                    />
                ) : (
                    <Field data-invalid={err('if.value') ? true : undefined}>
                        <FieldLabel htmlFor={id('if-value')}>
                            {t('automations.fields.value')}
                        </FieldLabel>
                        <Input
                            id={id('if-value')}
                            value={cond.value ?? ''}
                            maxLength={500}
                            readOnly={readOnly}
                            aria-invalid={err('if.value') ? true : undefined}
                            onChange={(ev) =>
                                setIf({ value: ev.target.value }, 'if.value')
                            }
                        />
                        {err('if.value') && (
                            <p
                                role="alert"
                                className="text-sm text-destructive"
                            >
                                {err('if.value')}
                            </p>
                        )}
                    </Field>
                ))}
            <p className="text-xs text-muted-foreground">
                {t('automations.fields.caseHint')}
            </p>
            {(err('then') || err('else')) && (
                <p role="alert" className="text-sm text-destructive">
                    {err('then') ?? err('else')}
                </p>
            )}
            <ul className="grid gap-1.5 rounded-lg border bg-muted/40 p-3 text-xs text-muted-foreground">
                <li className="flex gap-2">
                    <span className="mt-1 size-2 shrink-0 rounded-full bg-emerald-500" />
                    {t('automations.fields.yesHint')}
                </li>
                <li className="flex gap-2">
                    <span className="mt-1 size-2 shrink-0 rounded-full bg-rose-500" />
                    {t('automations.fields.noHint')}
                </li>
                <li className="flex gap-2">
                    <span className="mt-1 size-2 shrink-0 rounded-full bg-muted-foreground" />
                    {t('automations.fields.nextHint')}
                </li>
            </ul>
        </div>
    );
}

function HttpFields({ step, edit, errors, readOnly }: Props) {
    const t = useT();
    const err = useErr(errors);
    const set = (patch: Partial<EditorStep>, group?: string) =>
        edit((s) => ({ ...s, ...patch }), group);
    const id = (f: string) => `${step.id}-${f}`;
    const headers = step.headers ?? [];
    const setHeader = (key: string, patch: { name?: string; value?: string }) =>
        edit(
            (s) => ({
                ...s,
                headers: (s.headers ?? []).map((h) =>
                    h.key === key ? { ...h, ...patch } : h,
                ),
            }),
            `header-${key}`,
        );
    return (
        <div className="grid gap-4">
            <SelectField
                id={id('method')}
                label={t('automations.fields.method')}
                value={step.method ?? 'POST'}
                disabled={readOnly}
                onChange={(method) => set({ method })}
                options={HTTP_METHODS.map((m) => ({ value: m, label: m }))}
                error={err('method')}
            />
            <PlaceholderText
                id={id('url')}
                label={t('automations.fields.url')}
                value={step.url ?? ''}
                onChange={(url) => set({ url }, 'url')}
                placeholder="https://example.com/hooks/messora"
                error={err('url')}
                readOnly={readOnly}
                hint={t('automations.fields.urlHint')}
            />
            <fieldset className="space-y-2">
                <legend className="mb-1 text-sm font-medium">
                    {t('automations.fields.headers')}
                </legend>
                {headers.length === 0 && (
                    <p className="text-sm text-muted-foreground">
                        {t('automations.fields.noHeaders')}
                    </p>
                )}
                {headers.map((h, i) => (
                    <div key={h.key} className="flex gap-2">
                        <Input
                            aria-label={t('automations.fields.headerName', {
                                n: i + 1,
                            })}
                            placeholder="X-Api-Key"
                            value={h.name}
                            readOnly={readOnly}
                            aria-invalid={err('headers') ? true : undefined}
                            onChange={(ev) =>
                                setHeader(h.key, { name: ev.target.value })
                            }
                        />
                        <Input
                            aria-label={t('automations.fields.headerValue', {
                                n: i + 1,
                            })}
                            placeholder={t(
                                'automations.fields.headerValueHint',
                            )}
                            value={h.value}
                            readOnly={readOnly}
                            onChange={(ev) =>
                                setHeader(h.key, { value: ev.target.value })
                            }
                        />
                        {!readOnly && (
                            <Button
                                type="button"
                                size="icon"
                                variant="ghost"
                                aria-label={t(
                                    'automations.fields.removeHeader',
                                    {
                                        n: i + 1,
                                    },
                                )}
                                onClick={() =>
                                    set({
                                        headers: headers.filter(
                                            (x) => x.key !== h.key,
                                        ),
                                    })
                                }
                            >
                                <Trash2 />
                            </Button>
                        )}
                    </div>
                ))}
                {err('headers') && (
                    <p role="alert" className="text-sm text-destructive">
                        {err('headers')}
                    </p>
                )}
                {!readOnly && (
                    <Button
                        type="button"
                        size="sm"
                        variant="outline"
                        disabled={headers.length >= 20}
                        onClick={() =>
                            set({
                                headers: [
                                    ...headers,
                                    { key: newKey(), name: '', value: '' },
                                ],
                            })
                        }
                    >
                        <Plus /> {t('automations.fields.addHeader')}
                    </Button>
                )}
            </fieldset>
            <PlaceholderText
                id={id('body')}
                label={t('automations.fields.body')}
                multiline
                value={step.body ?? ''}
                onChange={(body) => set({ body }, 'body')}
                placeholder='{"phone": "{{phone}}"}'
                error={err('body')}
                readOnly={readOnly}
                hint={t('automations.fields.bodyHint')}
            />
        </div>
    );
}

function TemplateFields({ step, edit, errors, options, readOnly }: Props) {
    const t = useT();
    const tpl = step.template ?? { name: '', language: '' };
    const setTpl = (
        patch: Partial<NonNullable<EditorStep['template']>>,
        group?: string,
    ) =>
        edit(
            (s) => ({ ...s, template: { ...(s.template ?? tpl), ...patch } }),
            group,
        );
    const e = errors.find((x) => x.field.startsWith('template'));
    const err = e?.message;
    const id = (f: string) => `${step.id}-${f}`;
    const value = tpl.name ? `${tpl.name}|${tpl.language}` : '';
    const known = options.templates.find(
        (x) => x.name === tpl.name && x.language === tpl.language,
    );
    const spec = known
        ? templateVariables(known.components)
        : { body: varsSpec(tpl.body), header: varsSpec(tpl.header) };

    const fresh = (s: { named: boolean; names: string[] }) =>
        s.names.length === 0
            ? undefined
            : s.named
              ? Object.fromEntries(s.names.map((n) => [n, '']))
              : s.names.map(() => '');

    const choose = (v: string) => {
        const [name = '', language = ''] = v.split('|');
        const x = options.templates.find(
            (o) => o.name === name && o.language === language,
        );
        const sp = x ? templateVariables(x.components) : null;
        setTpl({
            name,
            language,
            body: sp ? fresh(sp.body) : undefined,
            header: sp ? fresh(sp.header) : undefined,
        });
    };

    const opts = options.templates.map((x) => ({
        value: `${x.name}|${x.language}`,
        label: `${x.name} · ${x.language}`,
    }));
    if (value && !known) {
        opts.push({
            value,
            label: t('automations.fields.notApproved', {
                name: `${tpl.name} · ${tpl.language}`,
            }),
        });
    }

    return (
        <div className="grid gap-4">
            {options.templates.length > 0 || value ? (
                <SelectField
                    id={id('template')}
                    label={t('automations.fields.template')}
                    value={value}
                    onChange={choose}
                    disabled={readOnly}
                    options={opts}
                    placeholder={t('automations.fields.chooseTemplate')}
                    error={err}
                />
            ) : (
                <div className="grid gap-4">
                    <Field data-invalid={err ? true : undefined}>
                        <FieldLabel htmlFor={id('tpl-name')}>
                            {t('automations.fields.templateName')}
                        </FieldLabel>
                        <Input
                            id={id('tpl-name')}
                            value={tpl.name}
                            readOnly={readOnly}
                            aria-invalid={err ? true : undefined}
                            onChange={(ev) =>
                                setTpl({ name: ev.target.value }, 'tpl-name')
                            }
                        />
                    </Field>
                    <Field>
                        <FieldLabel htmlFor={id('tpl-lang')}>
                            {t('automations.fields.language')}
                        </FieldLabel>
                        <Input
                            id={id('tpl-lang')}
                            value={tpl.language}
                            placeholder="en_US"
                            readOnly={readOnly}
                            onChange={(ev) =>
                                setTpl(
                                    { language: ev.target.value },
                                    'tpl-lang',
                                )
                            }
                        />
                    </Field>
                    {err && (
                        <p role="alert" className="text-sm text-destructive">
                            {err}
                        </p>
                    )}
                    <p className="text-xs text-muted-foreground">
                        {options.canReadTemplates
                            ? t('automations.fields.noTemplates')
                            : t('automations.fields.cannotListTemplates')}
                    </p>
                </div>
            )}
            {(['header', 'body'] as const).map((part) =>
                spec[part].names.length ? (
                    <fieldset key={part} className="grid gap-3">
                        <legend className="mb-1 text-sm font-medium">
                            {t(mk(`automations.fields.${part}Variables`))}
                        </legend>
                        {spec[part].names.map((n, i) => (
                            <PlaceholderText
                                key={n}
                                id={id(`${part}-${n}`)}
                                label={`{{${n}}}`}
                                value={readVar(
                                    tpl[part],
                                    spec[part].named,
                                    n,
                                    i,
                                )}
                                onChange={(v) =>
                                    setTpl(
                                        {
                                            [part]: writeVar(
                                                tpl[part],
                                                spec[part],
                                                n,
                                                i,
                                                v,
                                            ),
                                        },
                                        `${part}-${n}`,
                                    )
                                }
                                readOnly={readOnly}
                                maxLength={1024}
                            />
                        ))}
                    </fieldset>
                ) : null,
            )}
            <p className="text-xs text-muted-foreground">
                {t('automations.fields.templateHint')}
            </p>
        </div>
    );
}

function varsSpec(v: TemplateVars | undefined) {
    if (!v) return { named: false, names: [] as string[] };
    if (Array.isArray(v)) {
        return { named: false, names: v.map((_, i) => String(i + 1)) };
    }
    return { named: true, names: Object.keys(v) };
}

function readVar(
    v: TemplateVars | undefined,
    named: boolean,
    name: string,
    i: number,
): string {
    if (!v) return '';
    if (Array.isArray(v)) return named ? '' : (v[i] ?? '');
    return v[name] ?? '';
}

function writeVar(
    v: TemplateVars | undefined,
    spec: { named: boolean; names: string[] },
    name: string,
    i: number,
    value: string,
): TemplateVars {
    if (spec.named) {
        const obj = v && !Array.isArray(v) ? { ...v } : {};
        obj[name] = value;
        return obj;
    }
    const arr = Array.isArray(v) ? [...v] : spec.names.map(() => '');
    while (arr.length < spec.names.length) arr.push('');
    arr[i] = value;
    return arr;
}

// ── Trigger ────────────────────────────────────────────────────────────────

const TRIGGER_FIELDS = [
    'trigger.match',
    'trigger.keywords',
    'trigger.phone_number_id',
    'trigger.tag',
    'trigger.status',
    'trigger.campaign_id',
    'trigger.cooldown_seconds',
];

/** The trigger form in the inspector. */
export function TriggerFields({
    type,
    trigger,
    onType,
    onChange,
    options,
    errors,
    readOnly,
}: {
    type: AutomationTriggerType;
    trigger: TriggerForm;
    onType: (t: AutomationTriggerType) => void;
    onChange: (patch: Partial<TriggerForm>, group?: string) => void;
    options: EditorOptions;
    /** API errors by path ("trigger.tag", "trigger_type" …). */
    errors: Record<string, string>;
    readOnly: boolean;
}) {
    const t = useT();
    const tErr = (f: string) => errors[`trigger.${f}`];
    const other = Object.entries(errors)
        .filter(
            ([k]) =>
                (k === 'trigger' || k.startsWith('trigger.')) &&
                !TRIGGER_FIELDS.includes(k) &&
                !k.startsWith('trigger.keywords.'),
        )
        .map(([, v]) => v);

    const phoneOptions = [
        { value: 'any', label: t('automations.trigger.anyNumber') },
        ...options.phones.map((p) => ({ value: p.id, label: p.label })),
    ];
    if (
        trigger.phone_number_id &&
        !phoneOptions.some((p) => p.value === trigger.phone_number_id)
    ) {
        phoneOptions.push({
            value: trigger.phone_number_id,
            label: t('automations.summary.oneNumber'),
        });
    }
    const campaignOptions = [
        { value: 'any', label: t('automations.trigger.anyCampaign') },
        ...options.campaigns.map((c) => ({ value: c.id, label: c.label })),
    ];
    if (
        trigger.campaign_id &&
        !campaignOptions.some((c) => c.value === trigger.campaign_id)
    ) {
        campaignOptions.push({
            value: trigger.campaign_id,
            label: t('automations.trigger.oneCampaign'),
        });
    }

    return (
        <div className="grid gap-4">
            <SelectField
                id="trigger-type"
                label={t('automations.trigger.when')}
                value={type}
                disabled={readOnly}
                onChange={(v) => onType(v as AutomationTriggerType)}
                options={TRIGGER_TYPES.map((v) => ({
                    value: v,
                    label: t(mk(`automations.triggers.${v}.label`)),
                }))}
                hint={t(mk(`automations.triggers.${type}.description`))}
                error={errors.trigger_type}
            />
            {type === 'message_received' && (
                <>
                    <SelectField
                        id="trigger-match"
                        label={t('automations.trigger.message')}
                        value={trigger.match}
                        disabled={readOnly}
                        onChange={(match) => onChange({ match })}
                        options={MATCH_MODES.map((m) => ({
                            value: m,
                            label: t(mk(`automations.match.${m}`)),
                        }))}
                        error={tErr('match')}
                    />
                    {trigger.match !== 'any' && (
                        <KeywordsInput
                            id="trigger-keywords"
                            label={t('automations.trigger.keywords')}
                            value={trigger.keywords}
                            onChange={(keywords) => onChange({ keywords })}
                            max={MAX_KEYWORDS}
                            readOnly={readOnly}
                            error={tErr('keywords')}
                            hint={t('automations.trigger.keywordsHint', {
                                max: MAX_KEYWORDS,
                            })}
                        />
                    )}
                </>
            )}
            {(type === 'message_received' || type === 'new_contact') && (
                <SelectField
                    id="trigger-phone"
                    label={t('automations.trigger.onNumber')}
                    value={trigger.phone_number_id || 'any'}
                    disabled={readOnly}
                    onChange={(v) =>
                        onChange({ phone_number_id: v === 'any' ? '' : v })
                    }
                    options={phoneOptions}
                    error={tErr('phone_number_id')}
                />
            )}
            {type === 'tag_added' && (
                <SuggestInput
                    id="trigger-tag"
                    label={t('automations.fields.tag')}
                    maxLength={60}
                    value={trigger.tag}
                    onChange={(tag) => onChange({ tag }, 'trigger-tag')}
                    suggestions={options.tags}
                    readOnly={readOnly}
                    error={tErr('tag')}
                    hint={t('automations.trigger.tagHint')}
                />
            )}
            {type === 'message_status' && (
                <>
                    <SelectField
                        id="trigger-status"
                        label={t('automations.trigger.status')}
                        value={trigger.status}
                        disabled={readOnly}
                        onChange={(status) => onChange({ status })}
                        options={MESSAGE_STATUSES.map((s) => ({
                            value: s,
                            label: t(mk(`automations.messageStatus.${s}`)),
                        }))}
                        error={tErr('status')}
                    />
                    <SelectField
                        id="trigger-campaign"
                        label={t('automations.trigger.campaign')}
                        value={trigger.campaign_id || 'any'}
                        disabled={readOnly}
                        onChange={(v) =>
                            onChange({ campaign_id: v === 'any' ? '' : v })
                        }
                        options={campaignOptions}
                        error={tErr('campaign_id')}
                        hint={t('automations.trigger.campaignHint')}
                    />
                </>
            )}
            <DurationInput
                id="trigger-cooldown"
                label={t('automations.trigger.cooldown')}
                seconds={trigger.cooldown_seconds}
                onChange={(cooldown_seconds) =>
                    onChange({ cooldown_seconds }, 'cooldown')
                }
                min={0}
                max={MAX_MUTE_SECONDS}
                disabled={readOnly}
                error={tErr('cooldown_seconds')}
                hint={t('automations.trigger.cooldownHint')}
            />
            {other.map((m) => (
                <p key={m} role="alert" className="text-sm text-destructive">
                    {m}
                </p>
            ))}
        </div>
    );
}
