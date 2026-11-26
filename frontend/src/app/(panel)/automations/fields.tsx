'use client';

import { Plus, Trash2 } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Field, FieldDescription, FieldLabel } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { NumberField } from '@/components/ui/number-field';
import { Switch } from '@/components/ui/switch';
import { useT } from '@/i18n/client';
import type { AutomationTriggerType } from '@/lib/types';
import type { EditorOptions } from './data';
import {
    DurationInput,
    FieldMessage,
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
    DEFAULT_VARIABLES,
    DEFAULT_WINDOW_MINUTES,
    type EditorStep,
    ESCALATION_REASONS,
    HTTP_METHODS,
    isRetryable,
    isVarField,
    MATCH_MODES,
    MAX_BACKOFF_SECONDS,
    MAX_NO_REPLY_MINUTES,
    MAX_RETRIES,
    MAX_WINDOW_MINUTES,
    MIN_NO_REPLY_MINUTES,
    MIN_WINDOW_MINUTES,
    NO_REPLY_FROM,
    REACTIONS,
    triggerMinutes,
    triggerProblems,
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
    SUBSCRIPTION_KINDS,
    type TriggerForm,
} from './model';
import { TemplateParamsEditor } from '@/components/template-params';
import { paramsFromValues, shapeOf, valuesFromParams } from '@/lib/templates';
import type { CampaignParams, Template } from '@/lib/types';

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

/** The form of one step in the inspector (with its retry policy). */
export function StepFields(props: Props) {
    return (
        <div className="grid gap-4">
            <StepForm {...props} />
            {isRetryable(props.step.type) && <RetryFields {...props} />}
        </div>
    );
}

function StepForm(props: Props) {
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
        case 'handoff_to_human':
        case 'ai_route_to_agent':
            return <HandoffFields {...props} />;
        case 'handoff_to_ai':
            return (
                <SelectField
                    id={id('mode')}
                    label={t('automations.fields.aiMode')}
                    value={step.mode || 'auto_reply'}
                    onChange={(mode) => set({ mode })}
                    disabled={readOnly}
                    options={(['auto_reply', 'awaiting_approval'] as const).map(
                        (m) => ({
                            value: m,
                            label: t(mk(`automations.fields.aiModes.${m}`)),
                        }),
                    )}
                    error={err('mode')}
                    hint={t('automations.fields.aiModeHint')}
                />
            );
        case 'add_internal_note':
            return (
                <div className="grid gap-4">
                    <PlaceholderText
                        id={id('text')}
                        label={t('automations.fields.note')}
                        multiline
                        maxLength={4000}
                        value={step.text ?? ''}
                        onChange={(text) => set({ text }, 'text')}
                        error={err('text')}
                        readOnly={readOnly}
                        hint={t('automations.fields.noteHint')}
                    />
                    <SwitchField
                        id={id('for-ai')}
                        label={t('automations.fields.forAI')}
                        hint={t('automations.fields.forAIHint')}
                        checked={!!step.for_ai}
                        onChange={(for_ai) => set({ for_ai })}
                        disabled={readOnly}
                    />
                </div>
            );
        case 'ai_generate_message':
            return (
                <div className="grid gap-4">
                    <PlaceholderText
                        id={id('prompt')}
                        label={t('automations.fields.prompt')}
                        multiline
                        maxLength={4000}
                        value={step.prompt ?? ''}
                        onChange={(prompt) => set({ prompt }, 'prompt')}
                        error={err('prompt')}
                        readOnly={readOnly}
                        placeholder={t('automations.fields.promptPlaceholder')}
                    />
                    <SelectField
                        id={id('output')}
                        label={t('automations.fields.output')}
                        value={step.output === 'send' ? 'send' : 'variable'}
                        onChange={(output) => set({ output })}
                        disabled={readOnly}
                        options={(['variable', 'send'] as const).map((o) => ({
                            value: o,
                            label: t(mk(`automations.fields.outputs.${o}`)),
                        }))}
                        hint={t('automations.fields.outputHint')}
                    />
                    <VariableInput {...props} />
                </div>
            );
        case 'ai_analyze_message':
            return (
                <div className="grid gap-4">
                    <AIWrites names={['sentiment', 'summary']} />
                    <ExtraPrompt {...props} />
                </div>
            );
        case 'ai_detect_intent':
            return (
                <div className="grid gap-4">
                    <KeywordsInput
                        id={id('intents')}
                        label={t('automations.fields.intents')}
                        value={step.intents ?? []}
                        onChange={(intents) =>
                            set({
                                intents: intents.map((x) =>
                                    x.trim().toLowerCase().replace(/\s+/g, '_'),
                                ),
                            })
                        }
                        max={20}
                        readOnly={readOnly}
                        error={err('intents')}
                        hint={t('automations.fields.intentsHint')}
                    />
                    <VariableInput {...props} />
                    <p className="rounded-lg border bg-muted/40 p-3 text-xs text-muted-foreground">
                        {t('automations.fields.intentBranchTip', {
                            variable: `var.${step.variable || 'intent'}`,
                        })}
                    </p>
                    <ExtraPrompt {...props} />
                </div>
            );
        case 'ai_extract_fields':
            return <ExtractFields {...props} />;
        case 'ai_classify_segment':
            return <SegmentFields {...props} />;
        case 'ai_auto_reply':
            return (
                <p className="text-sm text-muted-foreground">
                    {t('automations.fields.autoReplyHint')}
                </p>
            );
        case 'ai_build_profile':
            return (
                <div className="grid gap-4">
                    <Field data-invalid={err('attribute') ? true : undefined}>
                        <FieldLabel htmlFor={id('attribute')}>
                            {t('automations.fields.attribute')}
                        </FieldLabel>
                        <Input
                            id={id('attribute')}
                            value={step.attribute ?? ''}
                            placeholder="ai_profile"
                            maxLength={64}
                            readOnly={readOnly}
                            aria-invalid={err('attribute') ? true : undefined}
                            onChange={(e) =>
                                set({ attribute: e.target.value }, 'attribute')
                            }
                        />
                        <FieldMessage
                            id={id('attribute')}
                            error={err('attribute')}
                        />
                        <p className="text-xs text-muted-foreground">
                            {t('automations.fields.attributeHint')}
                        </p>
                    </Field>
                    <ExtraPrompt {...props} />
                </div>
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
                value={isVarField(cond.field) ? 'var.' : cond.field}
                disabled={readOnly}
                onChange={(v) => {
                    const field = v === 'var.' ? 'var.intent' : v;
                    const next = operatorsFor(field);
                    setIf({
                        field,
                        operator: next.includes(cond.operator)
                            ? cond.operator
                            : next[0]!,
                    });
                }}
                options={[
                    ...CONDITION_FIELDS.map((f) => ({
                        value: f.value,
                        label: conditionFieldLabel(f.value, t),
                    })),
                    {
                        value: 'var.',
                        label: t('automations.conditionFields.runVarOption'),
                    },
                ]}
                error={
                    isVarField(cond.field) ? undefined : err('if.field', 'if')
                }
            />
            {isVarField(cond.field) && (
                <Field data-invalid={err('if.field') ? true : undefined}>
                    <FieldLabel htmlFor={id('if-var')}>
                        {t('automations.fields.variable')}
                    </FieldLabel>
                    <Input
                        id={id('if-var')}
                        value={cond.field.slice(4)}
                        maxLength={40}
                        readOnly={readOnly}
                        aria-invalid={err('if.field') ? true : undefined}
                        onChange={(e) =>
                            setIf({ field: `var.${e.target.value}` }, 'if.var')
                        }
                    />
                    <FieldMessage id={id('if-var')} error={err('if.field')} />
                    <p className="text-xs text-muted-foreground">
                        {t('automations.fields.conditionVarHint')}
                    </p>
                </Field>
            )}
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

function SwitchField({
    id,
    label,
    hint,
    checked,
    onChange,
    disabled,
}: {
    id: string;
    label: string;
    hint?: string;
    checked: boolean;
    onChange: (v: boolean) => void;
    disabled?: boolean;
}) {
    return (
        <Field orientation="horizontal">
            <Switch
                id={id}
                checked={checked}
                onCheckedChange={onChange}
                disabled={disabled}
            />
            <div>
                <FieldLabel htmlFor={id}>{label}</FieldLabel>
                {hint && <FieldDescription>{hint}</FieldDescription>}
            </div>
        </Field>
    );
}

/** Teammates to pick, plus a "none" entry and an unknown current one. */
function assigneeOptions(
    options: EditorOptions,
    t: ReturnType<typeof useT>,
    current: string | undefined,
    none: string,
) {
    const opts = [{ value: 'none', label: none }];
    for (const u of options.users ?? [options.me]) {
        opts.push({
            value: u.id,
            label:
                u.id === options.me.id
                    ? t('automations.fields.me', { name: u.label })
                    : u.label,
        });
    }
    if (current && !opts.some((o) => o.value === current)) {
        opts.push({
            value: current,
            label: t('automations.fields.anotherUser'),
        });
    }
    return opts;
}

function HandoffFields({ step, edit, errors, options, readOnly }: Props) {
    const t = useT();
    const err = useErr(errors);
    const set = (patch: Partial<EditorStep>, group?: string) =>
        edit((s) => ({ ...s, ...patch }), group);
    const id = (f: string) => `${step.id}-${f}`;
    return (
        <div className="grid gap-4">
            <SelectField
                id={id('assignee')}
                label={t('automations.fields.handoffTo')}
                value={step.assignee_id || 'none'}
                onChange={(v) => set({ assignee_id: v === 'none' ? '' : v })}
                disabled={readOnly}
                options={assigneeOptions(
                    options,
                    t,
                    step.assignee_id,
                    t('automations.fields.pool'),
                )}
                error={err('assignee_id')}
                hint={t('automations.fields.handoffHint')}
            />
            <PlaceholderText
                id={id('text')}
                label={t('automations.fields.handoffNote')}
                multiline
                maxLength={500}
                value={step.text ?? ''}
                onChange={(text) => set({ text }, 'text')}
                error={err('text')}
                readOnly={readOnly}
            />
            {step.type === 'ai_route_to_agent' && (
                <p className="text-xs text-muted-foreground">
                    {t('automations.fields.routeHint')}
                </p>
            )}
        </div>
    );
}

/** The run variable an AI step writes ({{var:<name>}}). */
function VariableInput({ step, edit, errors, readOnly }: Props) {
    const t = useT();
    const err = useErr(errors);
    const id = `${step.id}-variable`;
    const fallback = DEFAULT_VARIABLES[step.type] ?? '';
    return (
        <Field data-invalid={err('variable') ? true : undefined}>
            <FieldLabel htmlFor={id}>
                {t('automations.fields.variable')}
            </FieldLabel>
            <Input
                id={id}
                value={step.variable ?? ''}
                placeholder={fallback}
                maxLength={40}
                readOnly={readOnly}
                aria-invalid={err('variable') ? true : undefined}
                onChange={(e) =>
                    edit(
                        (s) => ({ ...s, variable: e.target.value }),
                        'variable',
                    )
                }
            />
            <FieldMessage id={id} error={err('variable')} />
            <p className="text-xs text-muted-foreground">
                {t('automations.fields.variableHint', {
                    name: step.variable || fallback,
                })}
            </p>
        </Field>
    );
}

function AIWrites({ names }: { names: string[] }) {
    const t = useT();
    return (
        <p className="rounded-lg border bg-muted/40 p-3 text-xs text-muted-foreground">
            {t('automations.fields.aiWrites', {
                vars: names.map((n) => `{{var:${n}}}`).join(', '),
            })}
        </p>
    );
}

/** An optional extra instruction for an AI step. */
function ExtraPrompt({ step, edit, errors, readOnly }: Props) {
    const t = useT();
    const err = useErr(errors);
    return (
        <PlaceholderText
            id={`${step.id}-prompt`}
            label={t('automations.fields.extraPrompt')}
            multiline
            maxLength={4000}
            value={step.prompt ?? ''}
            onChange={(prompt) => edit((s) => ({ ...s, prompt }), 'prompt')}
            error={err('prompt')}
            readOnly={readOnly}
            hint={t('automations.fields.extraPromptHint')}
        />
    );
}

function ExtractFields({ step, edit, errors, options, readOnly }: Props) {
    const t = useT();
    const err = useErr(errors);
    const rows = step.fields ?? [];
    const setRows = (fields: NonNullable<EditorStep['fields']>, g?: string) =>
        edit((s) => ({ ...s, fields }), g);
    const keyOptions = [
        ...CONTACT_FIELDS.map((f) => ({
            value: f,
            label: contactFieldLabel(f, t),
        })),
        ...options.attributeKeys.map((k) => ({
            value: `attr:${k}`,
            label: `attr:${k}`,
        })),
    ];
    for (const r of rows) {
        if (r.key && !keyOptions.some((o) => o.value === r.key)) {
            keyOptions.push({ value: r.key, label: r.key });
        }
    }
    return (
        <fieldset className="space-y-2">
            <legend className="mb-1 text-sm font-medium">
                {t('automations.fields.extractFields')}
            </legend>
            {rows.map((r, i) => (
                <div key={r.rowKey} className="flex gap-2">
                    <div className="w-36 shrink-0">
                        <SelectField
                            id={`${step.id}-fkey-${r.rowKey}`}
                            label={t('automations.fields.extractKey', {
                                n: i + 1,
                            })}
                            value={r.key}
                            disabled={readOnly}
                            onChange={(key) =>
                                setRows(
                                    rows.map((x) =>
                                        x.rowKey === r.rowKey
                                            ? { ...x, key }
                                            : x,
                                    ),
                                )
                            }
                            options={keyOptions}
                        />
                    </div>
                    <Field className="min-w-0 flex-1">
                        <FieldLabel htmlFor={`${step.id}-flabel-${r.rowKey}`}>
                            {t('automations.fields.extractLabel')}
                        </FieldLabel>
                        <Input
                            id={`${step.id}-flabel-${r.rowKey}`}
                            value={r.label}
                            maxLength={120}
                            readOnly={readOnly}
                            onChange={(e) =>
                                setRows(
                                    rows.map((x) =>
                                        x.rowKey === r.rowKey
                                            ? { ...x, label: e.target.value }
                                            : x,
                                    ),
                                    `flabel-${r.rowKey}`,
                                )
                            }
                        />
                    </Field>
                    {!readOnly && (
                        <Button
                            type="button"
                            size="icon"
                            variant="ghost"
                            className="mt-6"
                            aria-label={t('automations.fields.removeField', {
                                n: i + 1,
                            })}
                            onClick={() =>
                                setRows(
                                    rows.filter((x) => x.rowKey !== r.rowKey),
                                )
                            }
                        >
                            <Trash2 />
                        </Button>
                    )}
                </div>
            ))}
            {err('fields') && (
                <p role="alert" className="text-sm text-destructive">
                    {err('fields')}
                </p>
            )}
            {!readOnly && (
                <Button
                    type="button"
                    size="sm"
                    variant="outline"
                    disabled={rows.length >= 20}
                    onClick={() =>
                        setRows([
                            ...rows,
                            { key: 'email', label: '', rowKey: newKey() },
                        ])
                    }
                >
                    <Plus /> {t('automations.fields.addField')}
                </Button>
            )}
            <p className="text-xs text-muted-foreground">
                {t('automations.fields.extractHint')}
            </p>
            <ExtraPrompt
                step={step}
                edit={edit}
                errors={errors}
                options={options}
                readOnly={readOnly}
            />
        </fieldset>
    );
}

function SegmentFields(props: Props) {
    const { step, edit, errors, options, readOnly } = props;
    const t = useT();
    const err = useErr(errors);
    const byGroup = (step.group_ids?.length ?? 0) > 0 || step.mode === 'groups';
    const id = (f: string) => `${step.id}-${f}`;
    return (
        <div className="grid gap-4">
            <SelectField
                id={id('segment-kind')}
                label={t('automations.fields.segmentKind')}
                value={byGroup ? 'groups' : 'tags'}
                disabled={readOnly}
                onChange={(v) =>
                    edit((s) => ({
                        ...s,
                        tags: [],
                        group_ids: [],
                        // Remembered only in the editor (not sent).
                        mode: v === 'groups' ? 'groups' : undefined,
                    }))
                }
                options={[
                    { value: 'tags', label: t('automations.fields.byTags') },
                    {
                        value: 'groups',
                        label: t('automations.fields.byGroups'),
                    },
                ]}
            />
            {byGroup ? (
                <fieldset className="space-y-1.5">
                    <legend className="mb-1 text-sm font-medium">
                        {t('automations.fields.segmentGroups')}
                    </legend>
                    {options.groups.length === 0 && (
                        <p className="text-sm text-muted-foreground">
                            {t('automations.fields.noGroups')}
                        </p>
                    )}
                    {options.groups.map((g) => {
                        const on = step.group_ids?.includes(g.id) ?? false;
                        return (
                            <label
                                key={g.id}
                                className="flex items-center gap-2 text-sm"
                            >
                                <Checkbox
                                    checked={on}
                                    disabled={readOnly}
                                    onCheckedChange={(v) =>
                                        edit((s) => ({
                                            ...s,
                                            group_ids: v
                                                ? [...(s.group_ids ?? []), g.id]
                                                : (s.group_ids ?? []).filter(
                                                      (x) => x !== g.id,
                                                  ),
                                        }))
                                    }
                                />
                                {g.label}
                            </label>
                        );
                    })}
                    {err('tags', 'group_ids') && (
                        <p role="alert" className="text-sm text-destructive">
                            {err('tags', 'group_ids')}
                        </p>
                    )}
                </fieldset>
            ) : (
                <KeywordsInput
                    id={id('tags')}
                    label={t('automations.fields.segmentTags')}
                    value={step.tags ?? []}
                    onChange={(tags) => edit((s) => ({ ...s, tags }))}
                    max={20}
                    readOnly={readOnly}
                    error={err('tags')}
                    hint={t('automations.fields.segmentHint')}
                />
            )}
            <VariableInput {...props} />
            <ExtraPrompt {...props} />
        </div>
    );
}

/** Per-step retry policy: up to N more attempts, doubling the wait. */
function RetryFields({ step, edit, errors, readOnly }: Props) {
    const t = useT();
    const err = useErr(errors);
    const r = step.retry ?? { max: 0, backoff: 30 };
    const setRetry = (patch: Partial<typeof r>, group?: string) =>
        edit((s) => ({ ...s, retry: { ...r, ...patch } }), group);
    const id = (f: string) => `${step.id}-${f}`;
    return (
        <fieldset className="space-y-3 rounded-lg border p-3">
            <legend className="px-1 text-sm font-medium">
                {t('automations.fields.retry')}
            </legend>
            <Field data-invalid={err('retry.max') ? true : undefined}>
                <FieldLabel htmlFor={id('retry-max')}>
                    {t('automations.fields.retryMax')}
                </FieldLabel>
                <NumberField
                    id={id('retry-max')}
                    min={0}
                    max={MAX_RETRIES}
                    value={r.max}
                    disabled={readOnly}
                    aria-invalid={err('retry.max') ? true : undefined}
                    onValueChange={(v) =>
                        setRetry({ max: v ?? 0 }, 'retry-max')
                    }
                />
                <FieldMessage id={id('retry-max')} error={err('retry.max')} />
            </Field>
            {r.max > 0 && (
                <DurationInput
                    key={`${step.id}-backoff`}
                    id={id('retry-backoff')}
                    label={t('automations.fields.retryBackoff')}
                    seconds={r.backoff}
                    onChange={(backoff) => setRetry({ backoff }, 'backoff')}
                    min={1}
                    max={MAX_BACKOFF_SECONDS}
                    disabled={readOnly}
                    error={err('retry.backoff')}
                />
            )}
            <p className="text-xs text-muted-foreground">
                {t('automations.fields.retryHint')}
            </p>
        </fieldset>
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
            {known && (
                <TemplateParamsEditor
                    shape={shapeOf(known as unknown as Template)}
                    value={valuesFromParams(
                        tpl as CampaignParams,
                        shapeOf(known as unknown as Template),
                    )}
                    onChange={(v) => {
                        const p = paramsFromValues(
                            v,
                            shapeOf(known as unknown as Template),
                        );
                        setTpl(
                            {
                                header: p.header,
                                body: p.body,
                                buttons: p.buttons,
                                fallback: p.fallback,
                                header_media: p.header_media,
                                location: p.location,
                                coupon_code: p.coupon_code,
                                expiration_time_ms: p.expiration_time_ms,
                                cards: p.cards,
                                otp_code: p.otp_code,
                                flow: p.flow,
                            },
                            'params',
                        );
                    }}
                    attributeKeys={options.attributeKeys}
                    disabled={readOnly}
                />
            )}
            {!known &&
                (['header', 'body'] as const).map((part) =>
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
    'trigger.subscription',
    'trigger.minutes',
    'trigger.no_reply_from',
    'trigger.reaction',
    'trigger.reasons',
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
    const local = triggerProblems(type, trigger, t);
    const tErr = (f: string) =>
        errors[`trigger.${f}`] ?? local.find((p) => p.field === f)?.message;
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
            {type === 'window_closing' && (
                <>
                    <DurationInput
                        key="window-minutes"
                        id="trigger-minutes"
                        label={t('automations.trigger.windowMinutes')}
                        seconds={triggerMinutes(type, trigger) * 60}
                        onChange={(s) =>
                            onChange({ minutes: Math.round(s / 60) }, 'minutes')
                        }
                        min={MIN_WINDOW_MINUTES * 60}
                        max={MAX_WINDOW_MINUTES * 60}
                        disabled={readOnly}
                        error={tErr('minutes')}
                        hint={t('automations.trigger.windowMinutesHint', {
                            minutes: DEFAULT_WINDOW_MINUTES,
                        })}
                    />
                    <SwitchField
                        id="trigger-include-answered"
                        label={t('automations.trigger.includeAnswered')}
                        hint={t('automations.trigger.includeAnsweredHint')}
                        checked={trigger.include_answered}
                        onChange={(include_answered) =>
                            onChange({ include_answered })
                        }
                        disabled={readOnly}
                    />
                    <p
                        className="rounded-lg border border-amber-500/40 bg-amber-500/10 p-3 text-xs text-amber-800 dark:text-amber-300"
                        data-testid="window-guidance"
                    >
                        {t('automations.trigger.windowGuidance')}
                    </p>
                </>
            )}
            {type === 'no_reply' && (
                <>
                    <SelectField
                        id="trigger-no-reply-from"
                        label={t('automations.trigger.noReplyFrom')}
                        value={trigger.no_reply_from || 'customer'}
                        disabled={readOnly}
                        onChange={(no_reply_from) =>
                            onChange({ no_reply_from })
                        }
                        options={NO_REPLY_FROM.map((w) => ({
                            value: w,
                            label: t(mk(`automations.trigger.noReply.${w}`)),
                        }))}
                        error={tErr('no_reply_from')}
                    />
                    <DurationInput
                        key="no-reply-minutes"
                        id="trigger-minutes"
                        label={t('automations.trigger.noReplyMinutes')}
                        seconds={triggerMinutes(type, trigger) * 60}
                        onChange={(s) =>
                            onChange({ minutes: Math.round(s / 60) }, 'minutes')
                        }
                        min={MIN_NO_REPLY_MINUTES * 60}
                        max={MAX_NO_REPLY_MINUTES * 60}
                        disabled={readOnly}
                        error={tErr('minutes')}
                    />
                    {trigger.no_reply_from !== 'agent' && (
                        <SwitchField
                            id="trigger-include-campaigns"
                            label={t('automations.trigger.includeCampaigns')}
                            hint={t('automations.trigger.includeCampaignsHint')}
                            checked={trigger.include_campaigns}
                            onChange={(include_campaigns) =>
                                onChange({ include_campaigns })
                            }
                            disabled={readOnly}
                        />
                    )}
                </>
            )}
            {type === 'reaction' && (
                <SelectField
                    id="trigger-reaction"
                    label={t('automations.trigger.reaction')}
                    value={trigger.reaction || 'any'}
                    disabled={readOnly}
                    onChange={(v) =>
                        onChange({ reaction: v === 'any' ? '' : v })
                    }
                    options={(['any', ...REACTIONS] as const).map((r) => ({
                        value: r,
                        label: t(mk(`automations.reactions.${r}`)),
                    }))}
                    error={tErr('reaction')}
                    hint={t('automations.trigger.reactionHint')}
                />
            )}
            {(type === 'campaign_reply' || type === 'reaction') && (
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
            )}
            {type === 'ai_escalation' && (
                <fieldset className="space-y-1.5">
                    <legend className="mb-1 text-sm font-medium">
                        {t('automations.trigger.reasons')}
                    </legend>
                    {ESCALATION_REASONS.map((r) => (
                        <label
                            key={r}
                            className="flex items-center gap-2 text-sm"
                        >
                            <Checkbox
                                checked={trigger.reasons.includes(r)}
                                disabled={readOnly}
                                onCheckedChange={(v) =>
                                    onChange({
                                        reasons: v
                                            ? [...trigger.reasons, r]
                                            : trigger.reasons.filter(
                                                  (x) => x !== r,
                                              ),
                                    })
                                }
                            />
                            {t(mk(`automations.escalationReasons.${r}`))}
                        </label>
                    ))}
                    <p className="text-xs text-muted-foreground">
                        {t('automations.trigger.reasonsHint')}
                    </p>
                    {tErr('reasons') && (
                        <p role="alert" className="text-sm text-destructive">
                            {tErr('reasons')}
                        </p>
                    )}
                </fieldset>
            )}
            {(type === 'message_received' ||
                type === 'new_contact' ||
                type === 'window_closing' ||
                type === 'no_reply') && (
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
            {type === 'subscription_request' && (
                <SelectField
                    id="trigger-subscription"
                    label={t('automations.trigger.subscription')}
                    value={trigger.subscription || 'any'}
                    disabled={readOnly}
                    onChange={(v) =>
                        onChange({ subscription: v === 'any' ? '' : v })
                    }
                    options={(['any', ...SUBSCRIPTION_KINDS] as const).map(
                        (k) => ({
                            value: k,
                            label: t(mk(`automations.subscription.${k}`)),
                        }),
                    )}
                    error={tErr('subscription')}
                    hint={t('automations.trigger.subscriptionHint')}
                />
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
