'use client';

import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { useEffect, useMemo, useState, useTransition } from 'react';
import { AlertCircle, FolderTree, Plus, Users, X } from 'lucide-react';
import { TextField } from '@/components/forms';
import { notify } from '@/components/notify';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
    Card,
    CardContent,
    CardDescription,
    CardFooter,
    CardHeader,
    CardTitle,
} from '@/components/ui/card';
import { Checkbox } from '@/components/ui/checkbox';
import { Field, FieldGroup, FieldLabel } from '@/components/ui/field';
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
import { Textarea } from '@/components/ui/textarea';
import { DatePicker } from '@/components/ui/date-picker';
import { NumberField } from '@/components/ui/number-field';
import { useT } from '@/i18n/client';
import type { MessageKey } from '@/i18n/translate';
import { cn } from '@/lib/utils';
import type { Segment, SegmentPreview } from '@/lib/types';
import {
    deleteSegmentAction,
    previewSegmentAction,
    saveSegmentAction,
} from '../actions';
import { ConfirmDelete } from '../confirm-delete';
import { contactName } from '../shared';
import {
    FIELD_KEYS,
    MAX_DEPTH,
    MAX_RULES,
    OPERATORS,
    countRules,
    fieldKind,
    fromDefinition,
    localProblems,
    locateError,
    newGroup,
    newRule,
    toDefinition,
    updateNode,
    valueShape,
    type GroupNode,
    type RuleNode,
    type TreeNode,
} from './segment-tree';
import { NUMBER_LOCALE } from '@/lib/format';

type Option = { id: string; name: string };

type Ctx = {
    readOnly: boolean;
    tags: string[];
    groups: Option[];
    problems: Map<string, string>;
    canAdd: boolean;
    change: (id: string, next: (n: TreeNode) => TreeNode | null) => void;
};

// ── Rule row ───────────────────────────────────────────────────────────────

function RuleRow({ rule, ctx }: { rule: RuleNode; ctx: Ctx }) {
    const t = useT();
    const kind = fieldKind(rule.field);
    const shape = valueShape(rule.operator);
    const problem = ctx.problems.get(rule.id);
    const errId = `${rule.id}-error`;
    const set = (patch: Partial<RuleNode>) =>
        ctx.change(rule.id, (n) => ({ ...(n as RuleNode), ...patch }));
    const fieldLabel = (key: string) => {
        const k = `contacts.segments.fields.${key}` as MessageKey;
        const text = t(k);
        return text === k ? key : text;
    };

    return (
        <li
            className="space-y-1 rounded-md border bg-background p-2"
            data-testid="segment-rule"
        >
            <div className="flex flex-wrap items-center gap-2">
                <Select
                    value={rule.field}
                    disabled={ctx.readOnly}
                    onValueChange={(f) => {
                        const k = fieldKind(f);
                        const op =
                            k === kind ? rule.operator : OPERATORS[k][0]!.key;
                        set({
                            field: f,
                            operator: op,
                            value: k === kind ? rule.value : '',
                        });
                    }}
                >
                    <SelectTrigger
                        className="w-48"
                        aria-label={t('contacts.segments.field')}
                    >
                        <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                        {FIELD_KEYS.map((key) => (
                            <SelectItem key={key} value={key}>
                                {fieldLabel(key)}
                            </SelectItem>
                        ))}
                    </SelectContent>
                </Select>
                <Select
                    value={rule.operator}
                    disabled={ctx.readOnly}
                    onValueChange={(op) =>
                        set({
                            operator: op,
                            value: valueShape(op) === shape ? rule.value : '',
                        })
                    }
                >
                    <SelectTrigger
                        className="w-48"
                        aria-label={t('contacts.segments.condition', {
                            field: fieldLabel(rule.field),
                        })}
                    >
                        <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                        {OPERATORS[kind].map((o) => (
                            <SelectItem key={o.key} value={o.key}>
                                {t(
                                    `contacts.segments.ops.${kind}.${o.key}` as MessageKey,
                                )}
                            </SelectItem>
                        ))}
                    </SelectContent>
                </Select>
                <ValueInput
                    rule={rule}
                    ctx={ctx}
                    invalid={!!problem}
                    describedBy={problem ? errId : undefined}
                    onChange={(value) => set({ value })}
                />
                {rule.not && (
                    <Badge
                        variant="outline"
                        title={t('contacts.segments.negatedHint')}
                    >
                        {t('contacts.segments.negated')}
                    </Badge>
                )}
                {!ctx.readOnly && (
                    <Button
                        type="button"
                        variant="ghost"
                        size="icon-sm"
                        className="ml-auto"
                        aria-label={t('contacts.segments.removeRule')}
                        onClick={() => ctx.change(rule.id, () => null)}
                    >
                        <X />
                    </Button>
                )}
            </div>
            {problem && (
                <p id={errId} role="alert" className="text-sm text-destructive">
                    {problem}
                </p>
            )}
        </li>
    );
}

function ValueInput({
    rule,
    ctx,
    invalid,
    describedBy,
    onChange,
}: {
    rule: RuleNode;
    ctx: Ctx;
    invalid: boolean;
    describedBy?: string;
    onChange: (v: string) => void;
}) {
    const shape = valueShape(rule.operator);
    const t = useT();
    const common = {
        'aria-invalid': invalid || undefined,
        'aria-describedby': describedBy,
        disabled: ctx.readOnly,
    };
    switch (shape) {
        case 'none':
            return null;
        case 'group':
            return (
                <Select
                    value={rule.value || undefined}
                    disabled={ctx.readOnly}
                    onValueChange={onChange}
                >
                    <SelectTrigger
                        className="w-48"
                        aria-label={t('contacts.filters.group')}
                        aria-invalid={invalid || undefined}
                        aria-describedby={describedBy}
                    >
                        <SelectValue
                            placeholder={
                                ctx.groups.length
                                    ? t('contacts.table.chooseGroup')
                                    : t('contacts.groups.empty')
                            }
                        />
                    </SelectTrigger>
                    <SelectContent>
                        {ctx.groups.map((g) => (
                            <SelectItem key={g.id} value={g.id}>
                                {g.name}
                            </SelectItem>
                        ))}
                        {rule.value &&
                            !ctx.groups.some((g) => g.id === rule.value) && (
                                <SelectItem value={rule.value}>
                                    {t('contacts.segments.deletedGroup')}
                                </SelectItem>
                            )}
                    </SelectContent>
                </Select>
            );
        case 'days':
            return (
                <NumberField
                    min={1}
                    max={3650}
                    allowEmpty
                    className="w-32"
                    aria-label={t('contacts.segments.days')}
                    aria-invalid={invalid || undefined}
                    disabled={ctx.readOnly}
                    value={rule.value === '' ? null : Number(rule.value)}
                    onValueChange={(v) => onChange(v === null ? '' : String(v))}
                />
            );
        case 'date':
            return (
                <>
                    <Label htmlFor={`${rule.id}-date`} className="sr-only">
                        {t('contacts.segments.date')}
                    </Label>
                    <DatePicker
                        id={`${rule.id}-date`}
                        className="w-44"
                        disabled={ctx.readOnly}
                        value={rule.value}
                        onValueChange={onChange}
                    />
                </>
            );
        case 'list':
            return (
                <Input
                    className="min-w-48 flex-1"
                    aria-label={t('contacts.segments.values')}
                    placeholder={t('contacts.segments.values')}
                    list={rule.field === 'tag' ? 'segment-tags' : undefined}
                    value={rule.value}
                    onChange={(e) => onChange(e.target.value)}
                    {...common}
                />
            );
        default:
            return (
                <Input
                    className="min-w-48 flex-1"
                    aria-label={t('contacts.segments.value')}
                    maxLength={200}
                    list={rule.field === 'tag' ? 'segment-tags' : undefined}
                    placeholder={
                        rule.field === 'tag'
                            ? t('contacts.segments.tagName')
                            : t('contacts.segments.value')
                    }
                    value={rule.value}
                    onChange={(e) => onChange(e.target.value)}
                    {...common}
                />
            );
    }
}

// ── Group box ──────────────────────────────────────────────────────────────

function GroupBox({
    group,
    depth,
    ctx,
}: {
    group: GroupNode;
    depth: number;
    ctx: Ctx;
}) {
    const set = (patch: Partial<GroupNode>) =>
        ctx.change(group.id, (n) => ({ ...(n as GroupNode), ...patch }));
    const add = (node: TreeNode) =>
        ctx.change(group.id, (n) => ({
            ...(n as GroupNode),
            rules: [...(n as GroupNode).rules, node],
        }));
    const t = useT();
    const problem = ctx.problems.get(group.id);
    // A nested group's rules sit one level deeper, within MAX_DEPTH.
    const canNest = depth + 2 <= MAX_DEPTH;
    const label =
        depth === 1
            ? t('contacts.segments.rules')
            : t('contacts.segments.ruleGroup', { depth });

    return (
        <fieldset
            className={cn(
                'space-y-3 rounded-lg border p-3',
                depth > 1 && 'bg-muted/40',
                group.not && 'border-destructive/50',
            )}
            data-testid="segment-group"
        >
            <legend className="sr-only">{label}</legend>
            <div className="flex flex-wrap items-center gap-2 text-sm">
                <span>{t('contacts.segments.matching')}</span>
                <Select
                    value={group.op}
                    disabled={ctx.readOnly}
                    onValueChange={(op) => set({ op: op as 'and' | 'or' })}
                >
                    <SelectTrigger
                        size="sm"
                        className="w-24"
                        aria-label={t('contacts.segments.combine', { label })}
                    >
                        <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                        <SelectItem value="and">
                            {t('contacts.segments.all')}
                        </SelectItem>
                        <SelectItem value="or">
                            {t('contacts.segments.any')}
                        </SelectItem>
                    </SelectContent>
                </Select>
                <span>{t('contacts.segments.ofThese')}</span>
                <span className="ml-auto flex items-center gap-2">
                    <Checkbox
                        id={`${group.id}-not`}
                        checked={group.not}
                        disabled={ctx.readOnly}
                        onCheckedChange={(v) => set({ not: v === true })}
                    />
                    <Label htmlFor={`${group.id}-not`} className="font-normal">
                        {t('contacts.segments.exclude')}
                    </Label>
                </span>
                {depth > 1 && !ctx.readOnly && (
                    <Button
                        type="button"
                        variant="ghost"
                        size="icon-sm"
                        aria-label={t('contacts.segments.removeGroup')}
                        onClick={() => ctx.change(group.id, () => null)}
                    >
                        <X />
                    </Button>
                )}
            </div>
            <ul className="space-y-2">
                {group.rules.map((r) =>
                    r.type === 'rule' ? (
                        <RuleRow key={r.id} rule={r} ctx={ctx} />
                    ) : (
                        <li key={r.id}>
                            <GroupBox group={r} depth={depth + 1} ctx={ctx} />
                        </li>
                    ),
                )}
            </ul>
            {problem && (
                <p role="alert" className="text-sm text-destructive">
                    {problem}
                </p>
            )}
            {!ctx.readOnly && (
                <div className="flex flex-wrap gap-2">
                    <Button
                        type="button"
                        size="sm"
                        variant="outline"
                        disabled={!ctx.canAdd}
                        onClick={() => add(newRule())}
                    >
                        <Plus /> {t('contacts.segments.addRule')}
                    </Button>
                    {canNest && (
                        <Button
                            type="button"
                            size="sm"
                            variant="ghost"
                            disabled={!ctx.canAdd}
                            onClick={() => add(newGroup())}
                        >
                            <FolderTree /> {t('contacts.segments.addGroup')}
                        </Button>
                    )}
                </div>
            )}
        </fieldset>
    );
}

// ── Preview ────────────────────────────────────────────────────────────────

type PreviewState =
    | { kind: 'idle' }
    | { kind: 'loading'; last?: SegmentPreview }
    | { kind: 'ok'; data: SegmentPreview }
    | { kind: 'error'; message: string };

function PreviewCard({
    state,
    canViewContacts,
}: {
    state: PreviewState;
    canViewContacts: boolean;
}) {
    const t = useT();
    const data =
        state.kind === 'ok'
            ? state.data
            : state.kind === 'loading'
              ? state.last
              : undefined;
    return (
        <Card aria-busy={state.kind === 'loading'}>
            <CardHeader>
                <CardTitle className="flex items-center gap-2">
                    {t('contacts.segments.preview')}
                    {state.kind === 'loading' && <Spinner />}
                </CardTitle>
                <CardDescription>
                    {t('contacts.segments.previewHint')}
                </CardDescription>
            </CardHeader>
            <CardContent className="space-y-3 text-sm">
                {state.kind === 'idle' && (
                    <p className="text-muted-foreground">
                        {t('contacts.segments.previewIdle')}
                    </p>
                )}
                {state.kind === 'error' && (
                    <p className="text-destructive">{state.message}</p>
                )}
                {data && (
                    <>
                        <p
                            className="text-3xl font-semibold tabular-nums"
                            aria-live="polite"
                            data-testid="segment-count"
                        >
                            {data.count.toLocaleString(NUMBER_LOCALE)}
                        </p>
                        {data.sample.length > 0 ? (
                            <ul className="divide-y">
                                {data.sample.map((c) => (
                                    <li key={c.id} className="py-1.5">
                                        {canViewContacts ? (
                                            <Link
                                                href={`/contacts/${c.id}`}
                                                className="font-medium hover:underline"
                                            >
                                                {contactName(
                                                    c,
                                                    t('contacts.unnamed'),
                                                )}
                                            </Link>
                                        ) : (
                                            <span className="font-medium">
                                                {contactName(
                                                    c,
                                                    t('contacts.unnamed'),
                                                )}
                                            </span>
                                        )}
                                        <span className="block font-mono text-xs text-muted-foreground">
                                            {c.phone}
                                        </span>
                                    </li>
                                ))}
                            </ul>
                        ) : (
                            <p className="text-muted-foreground">
                                {t('contacts.segments.noMatch')}
                            </p>
                        )}
                        {data.count > data.sample.length && (
                            <p className="text-xs text-muted-foreground">
                                {t('contacts.segments.showingNewest', {
                                    count: data.sample.length,
                                })}
                            </p>
                        )}
                    </>
                )}
            </CardContent>
        </Card>
    );
}

// ── Builder ────────────────────────────────────────────────────────────────

export function SegmentBuilder({
    segment,
    tags,
    groups,
    canManage,
    canViewContacts,
}: {
    segment?: Segment;
    tags: string[];
    groups: Option[];
    canManage: boolean;
    canViewContacts: boolean;
}) {
    const t = useT();
    const router = useRouter();
    const [name, setName] = useState(segment?.name ?? '');
    const [description, setDescription] = useState(segment?.description ?? '');
    const [root, setRoot] = useState<GroupNode>(() =>
        fromDefinition(segment?.definition),
    );
    const [apiProblem, setApiProblem] = useState<{
        id?: string;
        message: string;
    } | null>(null);
    const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});
    const [result, setResult] = useState<{
        json: string;
        state: PreviewState;
    } | null>(null);
    const [lastOk, setLastOk] = useState<SegmentPreview>();
    const [saving, startSaving] = useTransition();
    const readOnly = !canManage;
    // Incomplete rules are only flagged once the user tries to save.
    const [showLocal, setShowLocal] = useState(false);

    const local = useMemo(() => localProblems(root, t), [root, t]);
    const { definition, paths } = useMemo(() => toDefinition(root), [root]);
    const json = JSON.stringify(definition);
    const rules = countRules(root);

    const problems = useMemo(() => {
        const m = new Map(showLocal ? local : undefined);
        if (apiProblem?.id && !m.has(apiProblem.id)) {
            m.set(apiProblem.id, apiProblem.message);
        }
        return m;
    }, [local, apiProblem, showLocal]);

    // Debounced live preview of the current rules. Results are keyed by
    // the definition they belong to, so a stale answer is never shown.
    const ready = local.size === 0 && rules > 0;
    useEffect(() => {
        if (!ready) return;
        let cancelled = false;
        const timer = setTimeout(async () => {
            const r = await previewSegmentAction(JSON.parse(json));
            if (cancelled) return;
            if (r.ok) {
                setApiProblem(null);
                setLastOk(r.data);
                setResult({ json, state: { kind: 'ok', data: r.data } });
            } else {
                const msg = r.fieldErrors?.definition;
                const located = msg ? locateError(msg, paths) : null;
                setApiProblem(located);
                setResult({
                    json,
                    state: {
                        kind: 'error',
                        message: located?.id
                            ? t('contacts.segments.fixHighlighted')
                            : (located?.message ?? r.message),
                    },
                });
            }
        }, 500);
        return () => {
            cancelled = true;
            clearTimeout(timer);
        };
    }, [json, ready, paths, t]);
    const preview: PreviewState = !ready
        ? { kind: 'idle' }
        : result?.json === json
          ? result.state
          : { kind: 'loading', last: lastOk };

    const ctx: Ctx = {
        readOnly,
        tags,
        groups,
        problems,
        canAdd: rules < MAX_RULES,
        change: (id, next) => {
            setApiProblem(null);
            setRoot((r) => updateNode(r, id, next));
        },
    };

    const save = (e: React.FormEvent) => {
        e.preventDefault();
        setShowLocal(true);
        const errs: Record<string, string> = {};
        if (!name.trim()) errs.name = t('contacts.segments.errName');
        if (rules === 0) errs.definition = t('contacts.segments.errNoRules');
        else if (local.size > 0)
            errs.definition = t('contacts.segments.errRules');
        setFieldErrors(errs);
        if (Object.keys(errs).length > 0) return;
        startSaving(async () => {
            const r = await saveSegmentAction(segment?.id ?? null, {
                name,
                description,
                definition,
            });
            if (!r.ok) {
                const fe = { ...(r.fieldErrors ?? {}) };
                if (fe.definition) {
                    const located = locateError(fe.definition, paths);
                    setApiProblem(located);
                    fe.definition = located.id
                        ? t('contacts.segments.errRule')
                        : located.message;
                }
                setFieldErrors(fe);
                notify(r);
                return;
            }
            setFieldErrors({});
            notify(r);
            if (!segment) router.push(`/contacts/segments/${r.data.id}`);
            else router.refresh();
        });
    };

    return (
        <form
            onSubmit={save}
            noValidate
            aria-label={t('contacts.segments.formLabel')}
        >
            <datalist id="segment-tags">
                {tags.map((tag) => (
                    <option key={tag} value={tag} />
                ))}
            </datalist>
            <div className="grid gap-6 lg:grid-cols-[1fr_18rem]">
                <div className="min-w-0 space-y-6">
                    <Card>
                        <CardContent>
                            <FieldGroup className="grid gap-4 sm:grid-cols-2">
                                <TextField
                                    label={t('contacts.fields.name')}
                                    name="name"
                                    required
                                    maxLength={120}
                                    value={name}
                                    readOnly={readOnly}
                                    onChange={(e) => setName(e.target.value)}
                                    error={fieldErrors.name}
                                />
                                <Field>
                                    <FieldLabel htmlFor="segment-description">
                                        {t('contacts.fields.description')}
                                    </FieldLabel>
                                    <Textarea
                                        id="segment-description"
                                        rows={1}
                                        maxLength={500}
                                        value={description}
                                        readOnly={readOnly}
                                        onChange={(e) =>
                                            setDescription(e.target.value)
                                        }
                                    />
                                </Field>
                            </FieldGroup>
                        </CardContent>
                    </Card>
                    <Card>
                        <CardHeader>
                            <CardTitle>
                                {t('contacts.segments.rules')}
                            </CardTitle>
                            <CardDescription>
                                {t('contacts.segments.rulesHint', {
                                    rules: MAX_RULES,
                                    depth: MAX_DEPTH,
                                })}
                            </CardDescription>
                        </CardHeader>
                        <CardContent className="space-y-3">
                            {rules === 0 && root.rules.length === 0 ? (
                                <div className="flex flex-col items-start gap-2 text-sm text-muted-foreground">
                                    <p>{t('contacts.segments.noRules')}</p>
                                    {!readOnly && (
                                        <Button
                                            type="button"
                                            size="sm"
                                            variant="outline"
                                            onClick={() =>
                                                setRoot((r) => ({
                                                    ...r,
                                                    rules: [newRule()],
                                                }))
                                            }
                                        >
                                            <Plus />{' '}
                                            {t('contacts.segments.addRule')}
                                        </Button>
                                    )}
                                </div>
                            ) : (
                                <GroupBox group={root} depth={1} ctx={ctx} />
                            )}
                            {!ctx.canAdd && !readOnly && (
                                <p className="text-sm text-muted-foreground">
                                    {t('contacts.segments.maxRules', {
                                        max: MAX_RULES,
                                    })}
                                </p>
                            )}
                            {fieldErrors.definition && (
                                <Alert variant="destructive">
                                    <AlertCircle />
                                    <AlertDescription role="alert">
                                        {fieldErrors.definition}
                                    </AlertDescription>
                                </Alert>
                            )}
                        </CardContent>
                        {canManage && (
                            <CardFooter className="flex-wrap justify-end gap-2">
                                {segment && (
                                    <ConfirmDelete
                                        label={t(
                                            'contacts.segments.deleteLabel',
                                            {
                                                name: segment.name,
                                            },
                                        )}
                                        title={t(
                                            'contacts.segments.deleteTitle',
                                            {
                                                name: segment.name,
                                            },
                                        )}
                                        description={t(
                                            'contacts.segments.deleteBody',
                                        )}
                                        confirm={t(
                                            'contacts.segments.deleteConfirm',
                                        )}
                                        action={() =>
                                            deleteSegmentAction(segment.id)
                                        }
                                        onDone={() =>
                                            router.push('/contacts/segments')
                                        }
                                    />
                                )}
                                <Button type="submit" disabled={saving}>
                                    {saving && <Spinner />}
                                    {segment
                                        ? t('contacts.detail.save')
                                        : t('contacts.segments.save')}
                                </Button>
                            </CardFooter>
                        )}
                    </Card>
                </div>
                <div className="space-y-4">
                    <PreviewCard
                        state={preview}
                        canViewContacts={canViewContacts}
                    />
                    {segment && canViewContacts && (
                        <Button variant="outline" className="w-full" asChild>
                            <Link href={`/contacts?segment_id=${segment.id}`}>
                                <Users /> {t('contacts.segments.showAll')}
                            </Link>
                        </Button>
                    )}
                </div>
            </div>
        </form>
    );
}
