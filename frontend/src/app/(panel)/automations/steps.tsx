'use client';

import { createContext, use } from 'react';
import {
    ArrowDown,
    ArrowUp,
    BellOff,
    CircleCheck,
    Clock,
    Copy,
    FileText,
    GitBranch,
    Globe,
    Mail,
    MessageSquare,
    PenLine,
    Plus,
    Tag,
    TagsIcon,
    Trash2,
    UserCheck,
    UsersRound,
} from 'lucide-react';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import {
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuLabel,
    DropdownMenuSeparator,
    DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { Field, FieldLabel } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { cn } from '@/lib/utils';
import type { EditorOptions } from './data';
import {
    DurationInput,
    PlaceholderText,
    SelectField,
    SuggestInput,
} from './inputs';
import {
    cloneStep,
    CONDITION_FIELDS,
    CONTACT_FIELDS,
    countSteps,
    type EditorStep,
    HTTP_METHODS,
    type ListId,
    MAX_DELAY_SECONDS,
    MAX_DEPTH,
    MAX_MUTE_SECONDS,
    MAX_STEPS,
    MIN_MUTE_SECONDS,
    newKey,
    newStep,
    operatorNeedsValue,
    operatorsFor,
    type StepError,
    stepLabel,
    STEP_TYPES,
    type StepType,
    type TemplateVars,
    templateVariables,
} from './model';

export type EditorContextValue = {
    options: EditorOptions;
    readOnly: boolean;
    errors: Record<string, StepError>;
    total: number;
    editStep: (key: string, fn: (s: EditorStep) => EditorStep) => void;
    editList: (list: ListId, fn: (l: EditorStep[]) => EditorStep[]) => void;
};

export const EditorContext = createContext<EditorContextValue | null>(null);

function useEditor(): EditorContextValue {
    const v = use(EditorContext);
    if (!v) throw new Error('EditorContext missing');
    return v;
}

const ICONS: Record<string, React.ComponentType<{ className?: string }>> = {
    reply: MessageSquare,
    send_template: FileText,
    add_tag: Tag,
    remove_tag: TagsIcon,
    add_to_group: UsersRound,
    set_field: PenLine,
    condition: GitBranch,
    http_request: Globe,
    send_email: Mail,
    delay: Clock,
    assign: UserCheck,
    close_conversation: CircleCheck,
    mute: BellOff,
};

// ── Lists ──────────────────────────────────────────────────────────────────

/** A vertical list of step cards with an "Add step" menu. */
export function StepList({
    steps,
    list,
    depth,
    labelPrefix,
    branch,
}: {
    steps: EditorStep[];
    list: ListId;
    depth: number;
    labelPrefix: string;
    branch?: 'then' | 'else';
}) {
    const { readOnly } = useEditor();
    return (
        <div className="space-y-3">
            {steps.length === 0 && (
                <p className="rounded-lg border border-dashed p-3 text-sm text-muted-foreground">
                    {branch
                        ? `No steps when the condition is ${branch === 'then' ? 'true' : 'false'}.`
                        : 'No steps yet. Add the first one below.'}
                </p>
            )}
            {steps.length > 0 && (
                <ol className="space-y-3">
                    {steps.map((s, i) => (
                        <StepCard
                            key={s.key}
                            step={s}
                            index={i}
                            count={steps.length}
                            list={list}
                            depth={depth}
                            label={`${labelPrefix}${i + 1}`}
                        />
                    ))}
                </ol>
            )}
            {!readOnly && (
                <AddStepMenu list={list} depth={depth} branch={branch} />
            )}
        </div>
    );
}

function AddStepMenu({
    list,
    depth,
    branch,
}: {
    list: ListId;
    depth: number;
    branch?: 'then' | 'else';
}) {
    const { total, editList } = useEditor();
    const full = total >= MAX_STEPS;
    const deep = depth >= MAX_DEPTH;
    const add = (type: StepType) =>
        editList(list, (l) => [...l, newStep(type)]);
    const label = branch ? `Add step to ${branch}` : 'Add step';
    return (
        <div className="flex flex-wrap items-center gap-2">
            <DropdownMenu>
                <DropdownMenuTrigger asChild>
                    <Button
                        type="button"
                        variant="outline"
                        size="sm"
                        disabled={full}
                    >
                        <Plus /> {label}
                    </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="start" className="w-72">
                    <DropdownMenuLabel>Actions</DropdownMenuLabel>
                    {STEP_TYPES.filter((t) => t.type !== 'condition').map(
                        (t) => {
                            const Icon = ICONS[t.type] ?? Plus;
                            return (
                                <DropdownMenuItem
                                    key={t.type}
                                    onSelect={() => add(t.type)}
                                >
                                    <Icon />
                                    <span>{t.label}</span>
                                </DropdownMenuItem>
                            );
                        },
                    )}
                    <DropdownMenuSeparator />
                    <DropdownMenuItem
                        disabled={deep}
                        onSelect={() => add('condition')}
                    >
                        <GitBranch />
                        <span>
                            Condition
                            {deep && (
                                <span className="block text-xs text-muted-foreground">
                                    At most {MAX_DEPTH} nested levels
                                </span>
                            )}
                        </span>
                    </DropdownMenuItem>
                </DropdownMenuContent>
            </DropdownMenu>
            {full && (
                <span className="text-xs text-muted-foreground">
                    An automation can have at most {MAX_STEPS} steps, including
                    conditions and nested steps.
                </span>
            )}
        </div>
    );
}

// ── Cards ──────────────────────────────────────────────────────────────────

function StepCard({
    step,
    index,
    count,
    list,
    depth,
    label,
}: {
    step: EditorStep;
    index: number;
    count: number;
    list: ListId;
    depth: number;
    label: string;
}) {
    const { readOnly, errors, total, editList } = useEditor();
    const error = errors[step.key];
    const Icon = ICONS[step.type] ?? FileText;
    const size = countSteps([step]);
    const title = `${label} · ${stepLabel(step.type)}`;
    const move = (by: number) =>
        editList(list, (l) => {
            const next = [...l];
            const [s] = next.splice(index, 1);
            next.splice(index + by, 0, s!);
            return next;
        });
    return (
        <li
            id={`step-${step.key}`}
            tabIndex={-1}
            data-testid="step-card"
            data-step-type={step.type}
            data-invalid={error ? 'true' : undefined}
            aria-label={title}
            className={cn(
                'rounded-xl border bg-card p-4 outline-none focus-visible:ring-2 focus-visible:ring-ring',
                error && 'border-destructive',
            )}
        >
            <div className="mb-3 flex flex-wrap items-center gap-2">
                <span className="flex size-7 items-center justify-center rounded-md bg-muted">
                    <Icon className="size-4" />
                </span>
                <h3 className="text-sm font-medium">
                    <span className="text-muted-foreground">{label}</span>{' '}
                    {stepLabel(step.type)}
                </h3>
                {!readOnly && (
                    <div className="ml-auto flex gap-1">
                        <Button
                            type="button"
                            size="icon-sm"
                            variant="ghost"
                            aria-label={`Move ${label} up`}
                            disabled={index === 0}
                            onClick={() => move(-1)}
                        >
                            <ArrowUp />
                        </Button>
                        <Button
                            type="button"
                            size="icon-sm"
                            variant="ghost"
                            aria-label={`Move ${label} down`}
                            disabled={index === count - 1}
                            onClick={() => move(1)}
                        >
                            <ArrowDown />
                        </Button>
                        <Button
                            type="button"
                            size="icon-sm"
                            variant="ghost"
                            aria-label={`Duplicate ${label}`}
                            title={
                                total + size > MAX_STEPS
                                    ? `At most ${MAX_STEPS} steps`
                                    : undefined
                            }
                            disabled={total + size > MAX_STEPS}
                            onClick={() =>
                                editList(list, (l) => [
                                    ...l.slice(0, index + 1),
                                    cloneStep(step),
                                    ...l.slice(index + 1),
                                ])
                            }
                        >
                            <Copy />
                        </Button>
                        <Button
                            type="button"
                            size="icon-sm"
                            variant="ghost"
                            aria-label={`Remove ${label}`}
                            onClick={() =>
                                editList(list, (l) =>
                                    l.filter((s) => s.key !== step.key),
                                )
                            }
                        >
                            <Trash2 />
                        </Button>
                    </div>
                )}
            </div>
            {error && !shownByField(step.type, error.field) && (
                <Alert variant="destructive" className="mb-3">
                    <AlertDescription data-testid="step-error">
                        {error.message}
                    </AlertDescription>
                </Alert>
            )}
            <StepForm step={step} depth={depth} label={label} />
        </li>
    );
}

/** Error fields each form shows next to the input (others: on the card). */
const FIELD_ERRORS: Record<string, string[]> = {
    reply: ['text'],
    add_tag: ['tag'],
    remove_tag: ['tag'],
    add_to_group: ['group_id'],
    set_field: ['field', 'value'],
    condition: ['if', 'if.field', 'if.operator', 'if.value', 'then', 'else'],
    http_request: ['url', 'method', 'headers'],
    send_email: ['', 'to', 'subject', 'body'],
    delay: ['seconds'],
    mute: ['seconds'],
    assign: ['assignee_id'],
};

function shownByField(type: string, field: string): boolean {
    if (type === 'send_template') return field.startsWith('template');
    return FIELD_ERRORS[type]?.includes(field) ?? false;
}

function StepForm({
    step,
    depth,
    label,
}: {
    step: EditorStep;
    depth: number;
    label: string;
}) {
    const { editStep, errors, options, readOnly } = useEditor();
    const set = (patch: Partial<EditorStep>) =>
        editStep(step.key, (s) => ({ ...s, ...patch }));
    const id = (f: string) => `${step.key}-${f}`;
    const err = (...fields: string[]) => {
        const e = errors[step.key];
        return e && fields.includes(e.field) ? e.message : undefined;
    };

    switch (step.type) {
        case 'reply':
            return (
                <PlaceholderText
                    id={id('text')}
                    label="Message"
                    multiline
                    maxLength={4096}
                    value={step.text ?? ''}
                    onChange={(text) => set({ text })}
                    error={err('text')}
                    readOnly={readOnly}
                    hint="Sent only inside the 24-hour window; otherwise the step is skipped."
                />
            );
        case 'send_template':
            return <TemplateForm step={step} />;
        case 'add_tag':
        case 'remove_tag':
            return (
                <SuggestInput
                    id={id('tag')}
                    label="Tag"
                    maxLength={60}
                    value={step.tag ?? ''}
                    onChange={(tag) => set({ tag })}
                    suggestions={options.tags}
                    error={err('tag')}
                    hint={
                        step.type === 'add_tag'
                            ? 'A new tag is created when it does not exist yet.'
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
                opts.push({ value: step.group_id, label: 'Unknown group' });
            }
            return (
                <SelectField
                    id={id('group')}
                    label="Group"
                    value={step.group_id ?? ''}
                    onChange={(group_id) => set({ group_id })}
                    options={opts}
                    placeholder={
                        opts.length ? 'Choose a group' : 'No groups available'
                    }
                    error={err('group_id')}
                />
            );
        }
        case 'set_field':
            return (
                <div className="grid gap-4 sm:grid-cols-[12rem_1fr]">
                    <SelectField
                        id={id('field')}
                        label="Field"
                        value={step.field ?? ''}
                        onChange={(field) => set({ field })}
                        options={CONTACT_FIELDS}
                        error={err('field')}
                    />
                    <PlaceholderText
                        id={id('value')}
                        label="Value"
                        maxLength={500}
                        value={step.value ?? ''}
                        onChange={(value) => set({ value })}
                        error={err('value')}
                        readOnly={readOnly}
                        hint="Leave empty to clear the field."
                    />
                </div>
            );
        case 'condition':
            return <ConditionForm step={step} depth={depth} label={label} />;
        case 'http_request':
            return <HttpForm step={step} />;
        case 'send_email':
            return (
                <div className="grid gap-4">
                    <PlaceholderText
                        id={id('to')}
                        label="To"
                        value={step.to ?? ''}
                        onChange={(to) => set({ to })}
                        placeholder="team@example.com or {{email}}"
                        maxLength={254}
                        error={err('to', '')}
                        readOnly={readOnly}
                    />
                    <PlaceholderText
                        id={id('subject')}
                        label="Subject"
                        value={step.subject ?? ''}
                        onChange={(subject) => set({ subject })}
                        error={err('subject')}
                        readOnly={readOnly}
                    />
                    <PlaceholderText
                        id={id('body')}
                        label="Body"
                        multiline
                        value={step.body ?? ''}
                        onChange={(body) => set({ body })}
                        error={err('body')}
                        readOnly={readOnly}
                        hint="Needs SMTP; without it the step is skipped."
                    />
                </div>
            );
        case 'delay':
            return (
                <DurationInput
                    id={id('seconds')}
                    label="Wait for"
                    seconds={step.seconds ?? 0}
                    onChange={(seconds) => set({ seconds })}
                    min={1}
                    max={MAX_DELAY_SECONDS}
                    error={err('seconds')}
                    hint="1 second to 30 days. The wait survives restarts."
                />
            );
        case 'mute':
            return (
                <DurationInput
                    id={id('seconds')}
                    label="Mute automations for"
                    seconds={step.seconds ?? 0}
                    onChange={(seconds) => set({ seconds })}
                    min={MIN_MUTE_SECONDS}
                    max={MAX_MUTE_SECONDS}
                    error={err('seconds')}
                    hint="No automation starts for this contact meanwhile (1 minute to 90 days)."
                />
            );
        case 'assign': {
            const opts = [{ value: 'none', label: 'Nobody (unassign)' }];
            const users = options.users ?? [options.me];
            for (const u of users) {
                opts.push({
                    value: u.id,
                    label: u.id === options.me.id ? `${u.label} (me)` : u.label,
                });
            }
            if (
                step.assignee_id &&
                !opts.some((o) => o.value === step.assignee_id)
            ) {
                opts.push({ value: step.assignee_id, label: 'Another user' });
            }
            return (
                <SelectField
                    id={id('assignee')}
                    label="Assign the conversation to"
                    value={step.assignee_id || 'none'}
                    onChange={(v) =>
                        set({ assignee_id: v === 'none' ? '' : v })
                    }
                    options={opts}
                    error={err('assignee_id')}
                    hint={
                        options.users
                            ? 'Skipped when the person is no longer active or there is no conversation.'
                            : 'Only people who manage users can pick a teammate here.'
                    }
                />
            );
        }
        case 'close_conversation':
            return (
                <p className="text-sm text-muted-foreground">
                    Closes the contact&apos;s conversation. Skipped when there
                    is none.
                </p>
            );
        default:
            return (
                <div className="space-y-2 text-sm">
                    <p className="text-muted-foreground">
                        The editor does not know this step type; it is kept as
                        it is.
                    </p>
                    <pre className="overflow-x-auto rounded-md bg-muted p-2 text-xs">
                        {JSON.stringify(step.raw, null, 2)}
                    </pre>
                </div>
            );
    }
}

// ── Type-specific forms ────────────────────────────────────────────────────

function ConditionForm({
    step,
    depth,
    label,
}: {
    step: EditorStep;
    depth: number;
    label: string;
}) {
    const { editStep, errors, options } = useEditor();
    const cond = step.if ?? { field: 'message.body', operator: 'contains' };
    const setIf = (patch: Partial<typeof cond>) =>
        editStep(step.key, (s) => ({
            ...s,
            if: { ...(s.if ?? cond), ...patch },
        }));
    const e = errors[step.key];
    const err = (f: string) => (e && e.field === f ? e.message : undefined);
    const ops = operatorsFor(cond.field);
    const id = (f: string) => `${step.key}-${f}`;
    return (
        <div className="space-y-4">
            <div className="grid gap-4 sm:grid-cols-3">
                <SelectField
                    id={id('if-field')}
                    label="If"
                    value={cond.field}
                    onChange={(field) => {
                        const next = operatorsFor(field);
                        setIf({
                            field,
                            operator: next.some(
                                (o) => o.value === cond.operator,
                            )
                                ? cond.operator
                                : next[0]!.value,
                        });
                    }}
                    options={CONDITION_FIELDS}
                    error={err('if.field') ?? err('if')}
                />
                <SelectField
                    id={id('if-operator')}
                    label="Operator"
                    value={cond.operator}
                    onChange={(operator) => setIf({ operator })}
                    options={ops}
                    error={err('if.operator')}
                />
                {operatorNeedsValue(cond.operator) &&
                    (cond.field === 'contact.tag' ? (
                        <SuggestInput
                            id={id('if-value')}
                            label="Tag"
                            maxLength={500}
                            value={cond.value ?? ''}
                            onChange={(value) => setIf({ value })}
                            suggestions={options.tags}
                            error={err('if.value')}
                        />
                    ) : (
                        <Field
                            data-invalid={err('if.value') ? true : undefined}
                        >
                            <FieldLabel htmlFor={id('if-value')}>
                                Value
                            </FieldLabel>
                            <Input
                                id={id('if-value')}
                                value={cond.value ?? ''}
                                maxLength={500}
                                aria-invalid={
                                    err('if.value') ? true : undefined
                                }
                                onChange={(ev) =>
                                    setIf({ value: ev.target.value })
                                }
                            />
                        </Field>
                    ))}
            </div>
            <p className="text-xs text-muted-foreground">
                Text comparisons ignore upper and lower case.
            </p>
            {(['then', 'else'] as const).map((b) => (
                <section
                    key={b}
                    aria-label={`${label} ${b}`}
                    className={cn(
                        'space-y-3 border-l-2 pl-4',
                        b === 'then'
                            ? 'border-emerald-500'
                            : 'border-amber-500',
                    )}
                    data-testid={`branch-${b}`}
                >
                    <h4 className="text-xs font-semibold tracking-wide text-muted-foreground uppercase">
                        {b === 'then' ? 'Then (true)' : 'Else (false)'}
                    </h4>
                    {err(b) && (
                        <p role="alert" className="text-sm text-destructive">
                            {err(b)}
                        </p>
                    )}
                    <StepList
                        steps={step[b] ?? []}
                        list={`${step.key}:${b}`}
                        depth={depth + 1}
                        labelPrefix={`${label} › ${b} `}
                        branch={b}
                    />
                </section>
            ))}
        </div>
    );
}

function HttpForm({ step }: { step: EditorStep }) {
    const { editStep, errors, readOnly } = useEditor();
    const set = (patch: Partial<EditorStep>) =>
        editStep(step.key, (s) => ({ ...s, ...patch }));
    const e = errors[step.key];
    const err = (f: string) => (e && e.field === f ? e.message : undefined);
    const id = (f: string) => `${step.key}-${f}`;
    const headers = step.headers ?? [];
    const setHeader = (key: string, patch: { name?: string; value?: string }) =>
        set({
            headers: headers.map((h) =>
                h.key === key ? { ...h, ...patch } : h,
            ),
        });
    return (
        <div className="grid gap-4">
            <div className="grid gap-4 sm:grid-cols-[8rem_1fr]">
                <SelectField
                    id={id('method')}
                    label="Method"
                    value={step.method ?? 'POST'}
                    onChange={(method) => set({ method })}
                    options={HTTP_METHODS.map((m) => ({ value: m, label: m }))}
                    error={err('method')}
                />
                <PlaceholderText
                    id={id('url')}
                    label="URL"
                    value={step.url ?? ''}
                    onChange={(url) => set({ url })}
                    placeholder="https://example.com/hooks/messora"
                    error={err('url')}
                    readOnly={readOnly}
                    hint="https only. Private and internal addresses are blocked; 15 s timeout."
                />
            </div>
            <fieldset className="space-y-2">
                <legend className="mb-1 text-sm font-medium">Headers</legend>
                {headers.length === 0 && (
                    <p className="text-sm text-muted-foreground">No headers.</p>
                )}
                {headers.map((h, i) => (
                    <div key={h.key} className="flex gap-2">
                        <Input
                            aria-label={`Header ${i + 1} name`}
                            placeholder="X-Api-Key"
                            value={h.name}
                            aria-invalid={err('headers') ? true : undefined}
                            onChange={(ev) =>
                                setHeader(h.key, { name: ev.target.value })
                            }
                        />
                        <Input
                            aria-label={`Header ${i + 1} value`}
                            placeholder="Value (placeholders allowed)"
                            value={h.value}
                            onChange={(ev) =>
                                setHeader(h.key, { value: ev.target.value })
                            }
                        />
                        {!readOnly && (
                            <Button
                                type="button"
                                size="icon"
                                variant="ghost"
                                aria-label={`Remove header ${i + 1}`}
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
                        <Plus /> Add header
                    </Button>
                )}
            </fieldset>
            <PlaceholderText
                id={id('body')}
                label="Body"
                multiline
                value={step.body ?? ''}
                onChange={(body) => set({ body })}
                placeholder='{"phone": "{{phone}}"}'
                error={err('body')}
                readOnly={readOnly}
                hint="Sent as application/json when present."
            />
        </div>
    );
}

function TemplateForm({ step }: { step: EditorStep }) {
    const { editStep, errors, options, readOnly } = useEditor();
    const tpl = step.template ?? { name: '', language: '' };
    const setTpl = (patch: Partial<NonNullable<EditorStep['template']>>) =>
        editStep(step.key, (s) => ({
            ...s,
            template: { ...(s.template ?? tpl), ...patch },
        }));
    const e = errors[step.key];
    const err = e && e.field.startsWith('template') ? e.message : undefined;
    const id = (f: string) => `${step.key}-${f}`;
    const value = tpl.name ? `${tpl.name}|${tpl.language}` : '';
    const known = options.templates.find(
        (t) => t.name === tpl.name && t.language === tpl.language,
    );

    const spec = known
        ? templateVariables(known.components)
        : {
              body: varsSpec(tpl.body),
              header: varsSpec(tpl.header),
          };

    const fresh = (s: { named: boolean; names: string[] }) =>
        s.names.length === 0
            ? undefined
            : s.named
              ? Object.fromEntries(s.names.map((n) => [n, '']))
              : s.names.map(() => '');

    const choose = (v: string) => {
        const [name = '', language = ''] = v.split('|');
        const t = options.templates.find(
            (x) => x.name === name && x.language === language,
        );
        const sp = t ? templateVariables(t.components) : null;
        setTpl({
            name,
            language,
            body: sp ? fresh(sp.body) : undefined,
            header: sp ? fresh(sp.header) : undefined,
        });
    };

    const opts = options.templates.map((t) => ({
        value: `${t.name}|${t.language}`,
        label: `${t.name} · ${t.language}`,
    }));
    if (value && !known) {
        opts.push({
            value,
            label: `${tpl.name} · ${tpl.language} (not approved)`,
        });
    }

    return (
        <div className="grid gap-4">
            {options.templates.length > 0 || value ? (
                <SelectField
                    id={id('template')}
                    label="Template"
                    value={value}
                    onChange={choose}
                    options={opts}
                    placeholder="Choose an approved template"
                    error={err}
                />
            ) : (
                <div className="grid gap-4 sm:grid-cols-2">
                    <Field data-invalid={err ? true : undefined}>
                        <FieldLabel htmlFor={id('tpl-name')}>
                            Template name
                        </FieldLabel>
                        <Input
                            id={id('tpl-name')}
                            value={tpl.name}
                            aria-invalid={err ? true : undefined}
                            onChange={(ev) => setTpl({ name: ev.target.value })}
                        />
                    </Field>
                    <Field>
                        <FieldLabel htmlFor={id('tpl-lang')}>
                            Language
                        </FieldLabel>
                        <Input
                            id={id('tpl-lang')}
                            value={tpl.language}
                            placeholder="en_US"
                            onChange={(ev) =>
                                setTpl({ language: ev.target.value })
                            }
                        />
                    </Field>
                    {err && (
                        <p role="alert" className="text-sm text-destructive">
                            {err}
                        </p>
                    )}
                    <p className="text-xs text-muted-foreground sm:col-span-2">
                        {options.canReadTemplates
                            ? 'No approved templates yet.'
                            : 'You cannot list templates; enter the exact name and language.'}
                    </p>
                </div>
            )}
            {(['header', 'body'] as const).map((part) =>
                spec[part].names.length ? (
                    <fieldset key={part} className="grid gap-3">
                        <legend className="mb-1 text-sm font-medium">
                            {part === 'body' ? 'Body' : 'Header'} variables
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
                                    setTpl({
                                        [part]: writeVar(
                                            tpl[part],
                                            spec[part],
                                            n,
                                            i,
                                            v,
                                        ),
                                    })
                                }
                                readOnly={readOnly}
                                maxLength={1024}
                            />
                        ))}
                    </fieldset>
                ) : null,
            )}
            <p className="text-xs text-muted-foreground">
                A template is skipped when a variable resolves to an empty value
                or the contact opted out.
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
