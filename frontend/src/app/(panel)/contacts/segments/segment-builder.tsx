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
    FIELDS,
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
    const kind = fieldKind(rule.field);
    const shape = valueShape(rule.operator);
    const problem = ctx.problems.get(rule.id);
    const errId = `${rule.id}-error`;
    const set = (patch: Partial<RuleNode>) =>
        ctx.change(rule.id, (n) => ({ ...(n as RuleNode), ...patch }));
    const field = FIELDS.find((f) => f.key === rule.field);

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
                    <SelectTrigger className="w-48" aria-label="Field">
                        <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                        {FIELDS.map((f) => (
                            <SelectItem key={f.key} value={f.key}>
                                {f.label}
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
                        aria-label={`Condition on ${field?.label ?? rule.field}`}
                    >
                        <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                        {OPERATORS[kind].map((o) => (
                            <SelectItem key={o.key} value={o.key}>
                                {o.label}
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
                    <Badge variant="outline" title="This rule is negated">
                        negated
                    </Badge>
                )}
                {!ctx.readOnly && (
                    <Button
                        type="button"
                        variant="ghost"
                        size="icon-sm"
                        className="ml-auto"
                        aria-label="Remove rule"
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
                        aria-label="Group"
                        aria-invalid={invalid || undefined}
                        aria-describedby={describedBy}
                    >
                        <SelectValue
                            placeholder={
                                ctx.groups.length
                                    ? 'Choose a group'
                                    : 'No groups yet'
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
                                    Deleted group
                                </SelectItem>
                            )}
                    </SelectContent>
                </Select>
            );
        case 'days':
            return (
                <Input
                    type="number"
                    min={1}
                    max={3650}
                    step={1}
                    className="w-28"
                    aria-label="Number of days"
                    value={rule.value}
                    onChange={(e) => onChange(e.target.value)}
                    {...common}
                />
            );
        case 'date':
            return (
                <Input
                    type="date"
                    className="w-44"
                    aria-label="Date"
                    value={rule.value}
                    onChange={(e) => onChange(e.target.value)}
                    {...common}
                />
            );
        case 'list':
            return (
                <Input
                    className="min-w-48 flex-1"
                    aria-label="Values, separated by commas"
                    placeholder="Values, separated by commas"
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
                    aria-label="Value"
                    maxLength={200}
                    list={rule.field === 'tag' ? 'segment-tags' : undefined}
                    placeholder={rule.field === 'tag' ? 'Tag name' : 'Value'}
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
    const problem = ctx.problems.get(group.id);
    // A nested group's rules sit one level deeper, within MAX_DEPTH.
    const canNest = depth + 2 <= MAX_DEPTH;
    const label = depth === 1 ? 'Rules' : `Rule group (level ${depth})`;

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
                <span>Contacts matching</span>
                <Select
                    value={group.op}
                    disabled={ctx.readOnly}
                    onValueChange={(op) => set({ op: op as 'and' | 'or' })}
                >
                    <SelectTrigger
                        size="sm"
                        className="w-24"
                        aria-label={`${label}: combine with`}
                    >
                        <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                        <SelectItem value="and">all</SelectItem>
                        <SelectItem value="or">any</SelectItem>
                    </SelectContent>
                </Select>
                <span>of these</span>
                <span className="ml-auto flex items-center gap-2">
                    <Checkbox
                        id={`${group.id}-not`}
                        checked={group.not}
                        disabled={ctx.readOnly}
                        onCheckedChange={(v) => set({ not: v === true })}
                    />
                    <Label htmlFor={`${group.id}-not`} className="font-normal">
                        Exclude them instead
                    </Label>
                </span>
                {depth > 1 && !ctx.readOnly && (
                    <Button
                        type="button"
                        variant="ghost"
                        size="icon-sm"
                        aria-label="Remove rule group"
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
                        <Plus /> Add rule
                    </Button>
                    {canNest && (
                        <Button
                            type="button"
                            size="sm"
                            variant="ghost"
                            disabled={!ctx.canAdd}
                            onClick={() => add(newGroup())}
                        >
                            <FolderTree /> Add rule group
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
                    Matching contacts
                    {state.kind === 'loading' && <Spinner />}
                </CardTitle>
                <CardDescription>
                    Updates as you edit the rules.
                </CardDescription>
            </CardHeader>
            <CardContent className="space-y-3 text-sm">
                {state.kind === 'idle' && (
                    <p className="text-muted-foreground">
                        Complete the rules to see who matches.
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
                            {data.count.toLocaleString()}
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
                                                {contactName(c)}
                                            </Link>
                                        ) : (
                                            <span className="font-medium">
                                                {contactName(c)}
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
                                No contact matches yet.
                            </p>
                        )}
                        {data.count > data.sample.length && (
                            <p className="text-xs text-muted-foreground">
                                Showing the {data.sample.length} newest.
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

    const local = useMemo(() => localProblems(root), [root]);
    const { definition, paths } = useMemo(() => toDefinition(root), [root]);
    const json = JSON.stringify(definition);
    const rules = countRules(root);

    const problems = useMemo(() => {
        const m = new Map(local);
        if (apiProblem?.id && !m.has(apiProblem.id)) {
            m.set(apiProblem.id, apiProblem.message);
        }
        return m;
    }, [local, apiProblem]);

    // Debounced live preview of the current rules. Results are keyed by
    // the definition they belong to, so a stale answer is never shown.
    const ready = local.size === 0 && rules > 0;
    useEffect(() => {
        if (!ready) return;
        let cancelled = false;
        const t = setTimeout(async () => {
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
                            ? 'Fix the highlighted rule.'
                            : (located?.message ?? r.message),
                    },
                });
            }
        }, 500);
        return () => {
            cancelled = true;
            clearTimeout(t);
        };
    }, [json, ready, paths]);
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
        const errs: Record<string, string> = {};
        if (!name.trim()) errs.name = 'Give the segment a name.';
        if (rules === 0) errs.definition = 'Add at least one rule.';
        else if (local.size > 0)
            errs.definition = 'Fix the rules marked in red.';
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
                        ? 'Fix the rule marked in red.'
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
        <form onSubmit={save} noValidate aria-label="Segment">
            <datalist id="segment-tags">
                {tags.map((t) => (
                    <option key={t} value={t} />
                ))}
            </datalist>
            <div className="grid gap-6 lg:grid-cols-[1fr_18rem]">
                <div className="min-w-0 space-y-6">
                    <Card>
                        <CardContent>
                            <FieldGroup className="grid gap-4 sm:grid-cols-2">
                                <TextField
                                    label="Name"
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
                                        Description
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
                            <CardTitle>Rules</CardTitle>
                            <CardDescription>
                                Up to {MAX_RULES} rules, nested up to{' '}
                                {MAX_DEPTH} levels. Text comparisons ignore
                                upper and lower case.
                            </CardDescription>
                        </CardHeader>
                        <CardContent className="space-y-3">
                            {rules === 0 && root.rules.length === 0 ? (
                                <div className="flex flex-col items-start gap-2 text-sm text-muted-foreground">
                                    <p>No rules yet.</p>
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
                                            <Plus /> Add rule
                                        </Button>
                                    )}
                                </div>
                            ) : (
                                <GroupBox group={root} depth={1} ctx={ctx} />
                            )}
                            {!ctx.canAdd && !readOnly && (
                                <p className="text-sm text-muted-foreground">
                                    This segment has the maximum of {MAX_RULES}{' '}
                                    rules.
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
                                        label={`Delete segment ${segment.name}`}
                                        title={`Delete ${segment.name}?`}
                                        description="Campaigns that already used it keep their recipients."
                                        confirm="Delete segment"
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
                                    {segment ? 'Save changes' : 'Save segment'}
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
                                <Users /> Show all in contacts
                            </Link>
                        </Button>
                    )}
                </div>
            </div>
        </form>
    );
}
