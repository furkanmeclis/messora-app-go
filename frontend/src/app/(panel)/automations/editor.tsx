'use client';

import {
    useCallback,
    useEffect,
    useMemo,
    useRef,
    useState,
    useTransition,
} from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import {
    type Connection,
    type EdgeChangeset,
    type NodeChangeset,
    ReactFlowProvider,
    useReactFlow,
} from '@xyflow/react';
import {
    AlertCircle,
    AlertTriangle,
    ArrowLeft,
    Copy,
    History,
    LayoutTemplate,
    Maximize2,
    Minimize2,
    Plus,
    Redo2,
    Trash2,
    Undo2,
    Wrench,
    Zap,
} from 'lucide-react';
import { toast } from 'sonner';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import {
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import {
    Field,
    FieldDescription,
    FieldGroup,
    FieldLabel,
} from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { Spinner } from '@/components/ui/spinner';
import { Switch } from '@/components/ui/switch';
import { Textarea } from '@/components/ui/textarea';
import {
    Tooltip,
    TooltipContent,
    TooltipTrigger,
} from '@/components/ui/tooltip';
import { useT } from '@/i18n/client';
import type { Translate } from '@/i18n/translate';
import type {
    Automation,
    AutomationStep,
    AutomationTriggerType,
} from '@/lib/types';
import { cn } from '@/lib/utils';
import { createAutomationAction, updateAutomationAction } from './actions';
import { DeleteButton } from './automations-client';
import { FlowCanvas } from './canvas';
import type { EditorOptions } from './data';
import { type StepPatch, StepFields, TriggerFields } from './fields';
import {
    autoLayout,
    canConnect,
    type FlowEdge,
    type FlowNode,
    freeOutput,
    graphFromSteps,
    graphToSteps,
    type Layout,
    makeEdge,
    NODE_HEIGHT,
    NODE_WIDTH,
    outputsOf,
    type StepNode,
    TRIGGER_ID,
    validateGraph,
} from './graph';
import { useHistory } from './history';
import { FieldMessage } from './inputs';
import {
    cloneStep,
    describeTrigger,
    isAIStep,
    MAX_STEPS,
    mapErrors,
    mk,
    newStep,
    pendingSetup,
    setupByStep,
    type StepError,
    stepLabel,
    stepProblems,
    STEP_TYPES,
    type StepType,
    type TriggerForm,
    triggerFromApi,
    triggerLabel,
    triggerProblems,
    triggerToApi,
} from './model';
import { CanvasContext, type CanvasContextValue, StepIcon } from './nodes';
import { DND_TYPE, Palette } from './palette';

type Doc = {
    nodes: FlowNode[];
    edges: FlowEdge[];
    triggerType: AutomationTriggerType;
    trigger: TriggerForm;
};

/** An undone snapshot keeps the current measurements and selection. */
function restore(snap: Doc, cur: Doc): Doc {
    const now = new Map(cur.nodes.map((n) => [n.id, n]));
    return {
        ...snap,
        nodes: snap.nodes.map((n) => {
            const c = now.get(n.id);
            return (
                c
                    ? { ...n, measured: c.measured, selected: c.selected }
                    : { ...n, selected: false }
            ) as FlowNode;
        }),
        edges: snap.edges.map((e) => ({ ...e, selected: false })),
    };
}

function layoutOf(nodes: FlowNode[]): Layout {
    return Object.fromEntries(
        nodes.map((n) => [
            n.id,
            { x: Math.round(n.position.x), y: Math.round(n.position.y) },
        ]),
    );
}

const GAP = 64;

type ServerErrors = {
    steps: Record<string, StepError>;
    other: Record<string, string>;
    message?: string;
};
const NO_ERRORS: ServerErrors = { steps: {}, other: {} };

function isTyping(target: EventTarget | null): boolean {
    const el = target as HTMLElement | null;
    return (
        !!el &&
        (el.isContentEditable ||
            ['INPUT', 'TEXTAREA', 'SELECT'].includes(el.tagName))
    );
}

export function AutomationEditor(props: {
    automation: Automation | null;
    options: EditorOptions;
    canManage: boolean;
}) {
    return (
        <ReactFlowProvider>
            <Editor {...props} />
        </ReactFlowProvider>
    );
}

function Editor({
    automation,
    options,
    canManage,
}: {
    automation: Automation | null;
    options: EditorOptions;
    canManage: boolean;
}) {
    const t = useT();
    const router = useRouter();
    const flow = useReactFlow<FlowNode, FlowEdge>();
    const readOnly = !canManage;
    const wrapper = useRef<HTMLDivElement>(null);

    const [name, setName] = useState(automation?.name ?? '');
    const [description, setDescription] = useState(
        automation?.description ?? '',
    );
    const [active, setActive] = useState(automation?.active ?? false);
    const history = useHistory<Doc>(() => {
        const g = graphFromSteps(automation?.steps, automation?.layout);
        return {
            ...g,
            triggerType: automation?.trigger_type ?? 'message_received',
            trigger: triggerFromApi(automation?.trigger),
        };
    }, restore);
    const { nodes, edges, triggerType, trigger } = history.state;
    const [server, setServer] = useState<ServerErrors>(NO_ERRORS);
    const [pending, startTransition] = useTransition();
    const [fullscreen, setFullscreen] = useState(false);

    // ── Dirty state ────────────────────────────────────────────────────────
    const tree = useMemo(() => graphToSteps(nodes, edges), [nodes, edges]);
    const definitionOf = (d: Doc, steps: AutomationStep[]) =>
        JSON.stringify({
            t: d.triggerType,
            o: triggerToApi(d.triggerType, d.trigger),
            s: steps,
        });
    const definition = definitionOf(history.state, tree.steps);
    const layoutKey = JSON.stringify(layoutOf(nodes));
    const details = JSON.stringify({ name, description, active });
    const [baseline, setBaseline] = useState(() => ({
        definition,
        layout: layoutKey,
        details,
    }));
    const redefined = definition !== baseline.definition;
    const dirty =
        redefined ||
        layoutKey !== baseline.layout ||
        details !== baseline.details;

    useEffect(() => {
        if (!dirty || readOnly) return;
        const onBeforeUnload = (e: BeforeUnloadEvent) => e.preventDefault();
        window.addEventListener('beforeunload', onBeforeUnload);
        return () => window.removeEventListener('beforeunload', onBeforeUnload);
    }, [dirty, readOnly]);

    // ── Setup of an installed preset (values still to choose) ──────────────
    const setup = useMemo(
        () =>
            pendingSetup(
                automation?.setup,
                nodes.flatMap((n) => (n.type === 'step' ? [n.data.step] : [])),
            ),
        [automation?.setup, nodes],
    );
    const setupMap = useMemo(() => setupByStep(setup), [setup]);
    const openFields = useMemo(
        () =>
            new Map(
                [...setupMap].map(([id, items]) => [
                    id,
                    new Set(items.map((i) => i.field)),
                ]),
            ),
        [setupMap],
    );
    const setupBlocks = setup.length > 0;

    // ── Problems (client checks plus the last save's API errors) ───────────
    const problems = useMemo(() => {
        const p = validateGraph(nodes, edges, t, openFields);
        for (const [id, e] of Object.entries(server.steps)) {
            if (nodes.some((n) => n.id === id)) {
                const list = (p.nodes[id] ??= []);
                if (!list.includes(e.message)) list.unshift(e.message);
            }
        }
        return p;
    }, [nodes, edges, t, server.steps, openFields]);

    // ── Editing ────────────────────────────────────────────────────────────
    const { set, checkpoint, undo, redo } = history;

    const select = useCallback(
        (id: string | null) =>
            set(
                (d) => ({
                    ...d,
                    nodes: d.nodes.map((n) =>
                        n.selected === (n.id === id)
                            ? n
                            : { ...n, selected: n.id === id },
                    ),
                    edges: d.edges.map((e) =>
                        e.selected ? { ...e, selected: false } : e,
                    ),
                }),
                { record: false },
            ),
        [set],
    );

    const editStep = useCallback(
        (id: string): StepPatch =>
            (fn, group) => {
                setServer((s) => {
                    if (!s.steps[id]) return s;
                    const steps = { ...s.steps };
                    delete steps[id];
                    return { ...s, steps };
                });
                set(
                    (d) => ({
                        ...d,
                        nodes: d.nodes.map((n) =>
                            n.id === id && n.type === 'step'
                                ? { ...n, data: { step: fn(n.data.step) } }
                                : n,
                        ),
                    }),
                    { group: group ? `${id}:${group}` : undefined },
                );
            },
        [set],
    );

    /**
     * Removes nodes and edges in one undo step. A removed plain step that
     * sat between two nodes is bridged, so A→B→C minus B is A→C.
     */
    const remove = useCallback(
        (nodeIds: string[], edgeIds: string[] = []) => {
            const gone = new Set(nodeIds.filter((id) => id !== TRIGGER_ID));
            if (!gone.size && !edgeIds.length) return;
            set((d) => {
                let next = d.edges.filter((e) => !edgeIds.includes(e.id));
                for (const id of gone) {
                    const node = d.nodes.find((n) => n.id === id);
                    const incoming = next.find((e) => e.target === id);
                    const out = next.find(
                        (e) => e.source === id && e.sourceHandle === 'out',
                    );
                    next = next.filter(
                        (e) => e.source !== id && e.target !== id,
                    );
                    if (
                        node?.type === 'step' &&
                        node.data.step.type !== 'condition' &&
                        incoming &&
                        out &&
                        !gone.has(incoming.source) &&
                        !gone.has(out.target)
                    ) {
                        next.push(
                            makeEdge(
                                incoming.source,
                                (incoming.sourceHandle ?? 'out') as never,
                                out.target,
                            ),
                        );
                    }
                }
                return {
                    ...d,
                    nodes: d.nodes.filter((n) => !gone.has(n.id)),
                    edges: next,
                };
            });
        },
        [set],
    );

    const onNodesChange = useCallback(
        (changes: NodeChangeset<FlowNode>) => {
            // Removal goes through onDelete (one undo step, bridging).
            for (const c of [...changes]) {
                if (c.type === 'remove') changes.remove(c);
            }
            if (changes.isEmpty()) return;
            set((d) => ({ ...d, nodes: changes.applyTo(d.nodes) }), {
                record: false,
            });
        },
        [set],
    );

    const onEdgesChange = useCallback(
        (changes: EdgeChangeset<FlowEdge>) => {
            for (const c of [...changes]) {
                if (c.type === 'remove') changes.remove(c);
            }
            if (changes.isEmpty()) return;
            set((d) => ({ ...d, edges: changes.applyTo(d.edges) }), {
                record: false,
            });
        },
        [set],
    );

    const isValidConnection = useCallback(
        (c: FlowEdge | Connection) =>
            !readOnly &&
            canConnect(nodes, edges, {
                source: c.source,
                sourceHandle: c.sourceHandle,
                target: c.target,
            }),
        [nodes, edges, readOnly],
    );

    const onConnect = useCallback(
        (c: Connection) => {
            set((d) =>
                canConnect(d.nodes, d.edges, c)
                    ? {
                          ...d,
                          edges: [
                              ...d.edges,
                              makeEdge(
                                  c.source,
                                  (c.sourceHandle ?? 'out') as never,
                                  c.target,
                              ),
                          ],
                      }
                    : d,
            );
        },
        [set],
    );

    /** Adds a step: after the selected node (or the end of the flow). */
    const addStep = useCallback(
        (type: StepType, at?: { x: number; y: number }) => {
            if (readOnly) return;
            if (nodes.length - 1 >= MAX_STEPS) {
                toast.error(
                    t('automations.problems.tooMany', { max: MAX_STEPS }),
                );
                return;
            }
            const step = newStep(type);
            set((d) => {
                let edgesNext = d.edges;
                let position = at;
                if (!position) {
                    let anchor = d.nodes.find(
                        (n) => n.selected && freeOutput(n, d.edges),
                    );
                    if (!anchor) {
                        // The end of the main path.
                        let cur = d.nodes.find((n) => n.id === TRIGGER_ID);
                        for (;;) {
                            const o = cur && freeOutput(cur, d.edges);
                            const main = cur && outputsOf(cur).at(-1);
                            if (!cur || (o && o === main)) break;
                            const e = d.edges.find(
                                (x) =>
                                    x.source === cur!.id &&
                                    x.sourceHandle === main,
                            );
                            const nextNode = d.nodes.find(
                                (n) => n.id === e?.target,
                            );
                            if (!nextNode) break;
                            cur = nextNode;
                        }
                        anchor = cur;
                    }
                    const output = anchor && freeOutput(anchor, d.edges);
                    if (anchor && output) {
                        const dx =
                            output === 'yes'
                                ? -(NODE_WIDTH / 2 + GAP / 2)
                                : output === 'no'
                                  ? NODE_WIDTH / 2 + GAP / 2
                                  : 0;
                        const h = anchor.measured?.height ?? NODE_HEIGHT;
                        // What follows a condition goes below its branches,
                        // so it never covers the yes/no nodes.
                        const belowBranches =
                            output === 'next' ? NODE_HEIGHT + GAP : 0;
                        position = {
                            x: anchor.position.x + dx,
                            y: anchor.position.y + h + GAP + belowBranches,
                        };
                        edgesNext = [
                            ...d.edges,
                            makeEdge(anchor.id, output, step.id),
                        ];
                    } else {
                        const box = wrapper.current?.getBoundingClientRect();
                        position = box
                            ? flow.screenToFlowPosition({
                                  x: box.left + box.width / 2,
                                  y: box.top + box.height / 2,
                              })
                            : { x: 0, y: 0 };
                        position = {
                            x: position.x - NODE_WIDTH / 2,
                            y: position.y - NODE_HEIGHT / 2,
                        };
                    }
                }
                // Never drop a node onto another: slide it down to free space.
                position = freeSpot(position, d.nodes);
                const node: StepNode = {
                    id: step.id,
                    type: 'step',
                    position,
                    data: { step },
                    selected: true,
                };
                return {
                    ...d,
                    nodes: [
                        ...d.nodes.map((n) =>
                            n.selected ? { ...n, selected: false } : n,
                        ),
                        node,
                    ],
                    edges: edgesNext,
                };
            });
        },
        [readOnly, nodes.length, set, t, flow],
    );

    const duplicate = (node: StepNode) => {
        const step = cloneStep(node.data.step);
        set((d) => ({
            ...d,
            nodes: [
                ...d.nodes.map((n) =>
                    n.selected ? { ...n, selected: false } : n,
                ),
                {
                    id: step.id,
                    type: 'step',
                    position: {
                        x: node.position.x + 32,
                        y: node.position.y + 32,
                    },
                    data: { step },
                    selected: true,
                },
            ],
        }));
    };

    const arrange = () => {
        set((d) => {
            const pos = autoLayout(d.nodes, d.edges);
            return {
                ...d,
                nodes: d.nodes.map((n) => ({
                    ...n,
                    position: pos[n.id] ?? n.position,
                })),
            };
        });
        requestAnimationFrame(() =>
            flow.fitView({ padding: 0.2, maxZoom: 1, duration: 300 }),
        );
    };

    // Cmd/Ctrl+Z undoes, Shift+Cmd/Ctrl+Z (or Ctrl+Y) redoes; text fields
    // keep their own undo.
    useEffect(() => {
        if (readOnly) return;
        const onKey = (e: KeyboardEvent) => {
            if (!(e.metaKey || e.ctrlKey) || isTyping(e.target)) return;
            const k = e.key.toLowerCase();
            if (k === 'z') {
                e.preventDefault();
                if (e.shiftKey) redo();
                else undo();
            } else if (k === 'y') {
                e.preventDefault();
                redo();
            }
        };
        window.addEventListener('keydown', onKey);
        return () => window.removeEventListener('keydown', onKey);
    }, [readOnly, undo, redo]);

    useEffect(() => {
        if (!fullscreen) return;
        const onKey = (e: KeyboardEvent) => {
            if (e.key === 'Escape' && !isTyping(e.target)) setFullscreen(false);
        };
        window.addEventListener('keydown', onKey);
        return () => window.removeEventListener('keydown', onKey);
    }, [fullscreen]);

    // ── Saving ─────────────────────────────────────────────────────────────
    const focusNode = (id: string) => {
        select(id);
        const n = nodes.find((x) => x.id === id);
        if (n) {
            flow.setCenter(
                n.position.x + NODE_WIDTH / 2,
                n.position.y + NODE_HEIGHT / 2,
                { zoom: Math.max(flow.getZoom(), 0.8), duration: 300 },
            );
        }
    };

    const save = () => {
        const local = validateGraph(nodes, edges, t, openFields);
        const firstBad = Object.keys(local.nodes)[0];
        const badTrigger = triggerProblems(triggerType, trigger, t)[0];
        if (badTrigger) {
            toast.error(badTrigger.message);
            select(TRIGGER_ID);
            return;
        }
        if (active && setupBlocks) {
            toast.error(t('automations.setup.blocksActive'));
            select(null);
            return;
        }
        if (!name.trim()) {
            select(null);
            setServer((s) => ({
                ...s,
                other: {
                    ...s.other,
                    name: t('automations.editor.nameRequired'),
                },
            }));
            toast.error(t('automations.editor.nameRequired'));
            return;
        }
        if (firstBad || local.global.length) {
            toast.error(local.global[0] ?? t('automations.editor.fixProblems'));
            if (firstBad) focusNode(firstBad);
            return;
        }
        const steps = tree.steps;
        const apiTrigger = triggerToApi(triggerType, trigger);
        const layout = layoutOf(nodes);
        startTransition(async () => {
            const r = automation
                ? await updateAutomationAction(automation.id, {
                      name: name.trim(),
                      description,
                      active,
                      layout,
                      ...(redefined
                          ? {
                                trigger_type: triggerType,
                                trigger: apiTrigger,
                                steps,
                            }
                          : {}),
                  })
                : await createAutomationAction({
                      name: name.trim(),
                      description,
                      active,
                      trigger_type: triggerType,
                      trigger: apiTrigger,
                      steps,
                      layout,
                  });
            if (r.ok) {
                setServer(NO_ERRORS);
                setBaseline({ definition, layout: layoutKey, details });
                if (r.message) toast.success(r.message);
                if (!automation) router.replace(`/automations/${r.data.id}`);
                else router.refresh();
                return;
            }
            const mapped = mapErrors(steps, r.fieldErrors);
            for (const k of Object.keys(mapped.other)) {
                if (k.startsWith('trigger.keywords.')) {
                    mapped.other['trigger.keywords'] ??= mapped.other[k]!;
                }
            }
            const shown = (k: string) =>
                ['name', 'description', 'trigger_type', 'active'].includes(k) ||
                k === 'trigger' ||
                k.startsWith('trigger.');
            const unshown = Object.entries(mapped.other)
                .filter(([k]) => !shown(k))
                .map(([, v]) => v);
            const handled =
                unshown.length === 0 &&
                (Object.keys(mapped.steps).length > 0 ||
                    Object.keys(mapped.other).length > 0);
            setServer({
                ...mapped,
                message: handled
                    ? undefined
                    : [r.message, ...unshown].join(' '),
            });
            toast.error(
                handled ? t('automations.editor.fixProblems') : r.message,
            );
            const first = Object.keys(mapped.steps)[0];
            if (first) focusNode(first);
            else if (
                Object.keys(mapped.other).some((k) => k.startsWith('trigger'))
            )
                select(TRIGGER_ID);
            else select(null);
        });
    };

    // ── Canvas context ─────────────────────────────────────────────────────
    const apiTrigger = triggerToApi(triggerType, trigger);
    const ctx: CanvasContextValue = useMemo(
        () => ({
            t,
            problems: problems.nodes,
            triggerLabel: triggerLabel(triggerType, t),
            triggerSummary: describeTrigger(triggerType, apiTrigger, t, {
                phones: new Map(options.phones.map((p) => [p.id, p.label])),
                campaigns: new Map(
                    options.campaigns.map((c) => [c.id, c.label]),
                ),
            }),
            lookup: {
                groups: new Map(options.groups.map((g) => [g.id, g.label])),
                users: new Map(
                    (options.users ?? [options.me]).map((u) => [u.id, u.label]),
                ),
            },
            readOnly,
            onDeleteEdge: (id) => remove([], [id]),
            aiEnabled: options.aiEnabled,
            setup: setupMap,
        }),
        // apiTrigger is derived from trigger and triggerType.
        // eslint-disable-next-line react-hooks/exhaustive-deps
        [
            t,
            problems.nodes,
            triggerType,
            trigger,
            options,
            readOnly,
            remove,
            setupMap,
        ],
    );

    const selectedNodes = nodes.filter((n) => n.selected);
    const selected = selectedNodes.length === 1 ? selectedNodes[0] : undefined;
    const triggerErrors = Object.keys(server.other).some(
        (k) => k === 'trigger_type' || k.startsWith('trigger'),
    );

    const toolbarButton = (
        label: string,
        icon: React.ReactNode,
        onClick: () => void,
        disabled?: boolean,
        shortcut?: string,
    ) => (
        <Tooltip>
            <TooltipTrigger asChild>
                <Button
                    type="button"
                    variant="ghost"
                    size="icon-sm"
                    aria-label={label}
                    disabled={disabled}
                    onClick={onClick}
                >
                    {icon}
                </Button>
            </TooltipTrigger>
            <TooltipContent>
                {label}
                {shortcut && (
                    <kbd className="ml-2 font-mono text-[10px] opacity-70">
                        {shortcut}
                    </kbd>
                )}
            </TooltipContent>
        </Tooltip>
    );

    return (
        <form
            noValidate
            aria-label={
                automation
                    ? t('automations.editor.editLabel')
                    : t('automations.editor.newLabel')
            }
            onSubmit={(e) => {
                e.preventDefault();
                if (!readOnly) save();
            }}
            className="space-y-3"
        >
            {readOnly && (
                <Alert>
                    <AlertCircle />
                    <AlertDescription>
                        {t('automations.editor.readOnly')}
                    </AlertDescription>
                </Alert>
            )}
            {server.message && (
                <Alert variant="destructive">
                    <AlertCircle />
                    <AlertDescription>{server.message}</AlertDescription>
                </Alert>
            )}

            <div
                data-testid="automation-canvas"
                className={cn(
                    'automation-canvas flex flex-col overflow-hidden border bg-card shadow-xs',
                    fullscreen
                        ? 'fixed inset-0 z-50 rounded-none'
                        : 'h-[calc(100dvh-12rem)] min-h-[560px] rounded-xl',
                )}
            >
                {/* Toolbar */}
                <div className="flex flex-wrap items-center gap-1 border-b bg-background/60 px-2 py-1.5 backdrop-blur">
                    <div className="mr-1 flex min-w-0 items-center gap-2 pl-1">
                        <span className="flex size-7 shrink-0 items-center justify-center rounded-md bg-primary/10 text-primary">
                            <Zap className="size-4" />
                        </span>
                        <span className="max-w-48 truncate text-sm font-semibold">
                            {name.trim() || t('automations.editor.untitled')}
                        </span>
                        <span
                            className={cn(
                                'rounded-full px-2 py-0.5 text-[11px] font-medium',
                                active
                                    ? 'bg-emerald-500/15 text-emerald-700 dark:text-emerald-300'
                                    : 'bg-muted text-muted-foreground',
                            )}
                        >
                            {active
                                ? t('automations.status.active')
                                : t('automations.status.inactive')}
                        </span>
                    </div>
                    {!readOnly && (
                        <>
                            <span className="mx-1 h-5 w-px bg-border" />
                            {toolbarButton(
                                t('automations.canvas.undo'),
                                <Undo2 />,
                                undo,
                                !history.canUndo,
                                '⌘Z',
                            )}
                            {toolbarButton(
                                t('automations.canvas.redo'),
                                <Redo2 />,
                                redo,
                                !history.canRedo,
                                '⇧⌘Z',
                            )}
                            {toolbarButton(
                                t('automations.canvas.autoLayout'),
                                <LayoutTemplate />,
                                arrange,
                            )}
                            <DropdownMenu>
                                <DropdownMenuTrigger asChild>
                                    <Button
                                        type="button"
                                        variant="ghost"
                                        size="sm"
                                        className="md:hidden"
                                    >
                                        <Plus />{' '}
                                        {t('automations.palette.addStep')}
                                    </Button>
                                </DropdownMenuTrigger>
                                <DropdownMenuContent align="start">
                                    {STEP_TYPES.map(({ type }) => (
                                        <DropdownMenuItem
                                            key={type}
                                            onSelect={() => addStep(type)}
                                        >
                                            <StepIcon type={type} />
                                            {stepLabel(type, t)}
                                        </DropdownMenuItem>
                                    ))}
                                </DropdownMenuContent>
                            </DropdownMenu>
                        </>
                    )}
                    {toolbarButton(
                        fullscreen
                            ? t('automations.canvas.exitFullscreen')
                            : t('automations.canvas.fullscreen'),
                        fullscreen ? <Minimize2 /> : <Maximize2 />,
                        () => setFullscreen((f) => !f),
                    )}
                    <div className="ml-auto flex flex-wrap items-center gap-2">
                        {automation && redefined && !readOnly && (
                            <p
                                className="hidden items-center gap-1.5 text-xs text-amber-700 xl:flex dark:text-amber-400"
                                data-testid="redefine-warning"
                            >
                                <AlertTriangle className="size-3.5 shrink-0" />
                                {t('automations.editor.redefineWarning')}
                            </p>
                        )}
                        {automation && (
                            <Button asChild variant="ghost" size="sm">
                                <Link
                                    href={`/automations/${automation.id}/runs`}
                                >
                                    <History />{' '}
                                    {t('automations.editor.runHistory')}
                                </Link>
                            </Button>
                        )}
                        {automation && canManage && (
                            <DeleteButton
                                automation={automation}
                                redirectTo="/automations"
                            />
                        )}
                        {!readOnly && (
                            <>
                                <Button asChild variant="ghost" size="sm">
                                    <Link href="/automations">
                                        {t('common.cancel')}
                                    </Link>
                                </Button>
                                <Button
                                    type="submit"
                                    size="sm"
                                    disabled={
                                        pending || (!!automation && !dirty)
                                    }
                                >
                                    {pending && <Spinner />}
                                    {automation
                                        ? t('automations.editor.save')
                                        : t('automations.editor.create')}
                                </Button>
                            </>
                        )}
                    </div>
                </div>

                <div className="flex min-h-0 flex-1 flex-col lg:flex-row">
                    {!readOnly && (
                        <aside className="hidden w-52 shrink-0 border-r bg-background/40 md:block">
                            <Palette onAdd={(type) => addStep(type)} />
                        </aside>
                    )}
                    <div
                        ref={wrapper}
                        className="relative min-h-[360px] min-w-0 flex-1"
                        onDragOver={(e) => {
                            if (readOnly) return;
                            if (e.dataTransfer.types.includes(DND_TYPE)) {
                                e.preventDefault();
                                e.dataTransfer.dropEffect = 'copy';
                            }
                        }}
                        onDrop={(e) => {
                            const type = e.dataTransfer.getData(DND_TYPE);
                            if (!type || readOnly) return;
                            e.preventDefault();
                            const p = flow.screenToFlowPosition({
                                x: e.clientX,
                                y: e.clientY,
                            });
                            addStep(type as StepType, {
                                x: p.x - NODE_WIDTH / 2,
                                y: p.y - 24,
                            });
                        }}
                    >
                        <CanvasContext value={ctx}>
                            <FlowCanvas
                                nodes={nodes}
                                edges={edges}
                                onNodesChange={onNodesChange}
                                onEdgesChange={onEdgesChange}
                                onConnect={onConnect}
                                isValidConnection={isValidConnection}
                                onDelete={({ nodes: ns, edges: es }) =>
                                    remove(
                                        ns.map((n) => n.id),
                                        es.map((e) => e.id),
                                    )
                                }
                                onNodeDragStart={() => checkpoint()}
                                deleteKeyCode={
                                    readOnly ? null : ['Backspace', 'Delete']
                                }
                                nodesDraggable={!readOnly}
                                nodesConnectable={!readOnly}
                                edgesReconnectable={false}
                            />
                        </CanvasContext>
                        {problems.global.length > 0 && (
                            <div
                                role="status"
                                data-testid="canvas-problems"
                                className="pointer-events-none absolute top-3 left-1/2 flex -translate-x-1/2 items-center gap-2 rounded-full border bg-card/95 px-3 py-1 text-xs text-muted-foreground shadow-sm"
                            >
                                <AlertCircle className="size-3.5 text-amber-500" />
                                {problems.global[0]}
                            </div>
                        )}
                    </div>
                    <aside
                        aria-label={t('automations.inspector.title')}
                        className="max-h-[50vh] w-full shrink-0 overflow-y-auto border-t bg-background/40 lg:max-h-none lg:w-[22rem] lg:border-t-0 lg:border-l"
                        data-testid="inspector"
                    >
                        {selected?.type === 'trigger' ? (
                            <Panel
                                title={t('automations.inspector.trigger')}
                                icon={<Zap className="size-4" />}
                                onBack={() => select(null)}
                                t={t}
                                invalid={triggerErrors}
                            >
                                <TriggerFields
                                    type={triggerType}
                                    trigger={trigger}
                                    onType={(v) =>
                                        set((d) => ({ ...d, triggerType: v }))
                                    }
                                    onChange={(patch, group) =>
                                        set(
                                            (d) => ({
                                                ...d,
                                                trigger: {
                                                    ...d.trigger,
                                                    ...patch,
                                                },
                                            }),
                                            { group },
                                        )
                                    }
                                    options={options}
                                    errors={server.other}
                                    readOnly={readOnly}
                                />
                            </Panel>
                        ) : selected?.type === 'step' ? (
                            <Panel
                                title={stepLabel(selected.data.step.type, t)}
                                icon={
                                    <StepIcon
                                        type={selected.data.step.type}
                                        className="size-4"
                                    />
                                }
                                onBack={() => select(null)}
                                t={t}
                                invalid={!!problems.nodes[selected.id]?.length}
                                actions={
                                    !readOnly && (
                                        <>
                                            <Button
                                                type="button"
                                                size="icon-sm"
                                                variant="ghost"
                                                aria-label={t(
                                                    'automations.inspector.duplicate',
                                                )}
                                                disabled={
                                                    nodes.length - 1 >=
                                                    MAX_STEPS
                                                }
                                                onClick={() =>
                                                    duplicate(selected)
                                                }
                                            >
                                                <Copy />
                                            </Button>
                                            <Button
                                                type="button"
                                                size="icon-sm"
                                                variant="ghost"
                                                aria-label={t(
                                                    'automations.inspector.delete',
                                                )}
                                                onClick={() =>
                                                    remove([selected.id])
                                                }
                                            >
                                                <Trash2 />
                                            </Button>
                                        </>
                                    )
                                }
                            >
                                <StepInspector
                                    key={selected.id}
                                    node={selected}
                                    edit={editStep(selected.id)}
                                    serverError={server.steps[selected.id]}
                                    problems={problems.nodes[selected.id] ?? []}
                                    open={openFields.get(selected.id)}
                                    setupItems={setupMap.get(selected.id)}
                                    options={options}
                                    readOnly={readOnly}
                                    t={t}
                                />
                            </Panel>
                        ) : (
                            <div className="space-y-5 p-4">
                                {selectedNodes.length > 1 && (
                                    <p className="rounded-lg border bg-muted/40 p-3 text-sm text-muted-foreground">
                                        {t('automations.inspector.many', {
                                            count: selectedNodes.length,
                                        })}
                                    </p>
                                )}
                                <div>
                                    <h2 className="text-sm font-semibold">
                                        {t('automations.inspector.details')}
                                    </h2>
                                    <p className="text-xs text-muted-foreground">
                                        {t('automations.inspector.detailsHint')}
                                    </p>
                                </div>
                                <fieldset
                                    disabled={readOnly}
                                    className="min-w-0"
                                >
                                    <FieldGroup>
                                        <Field
                                            data-invalid={
                                                server.other.name
                                                    ? true
                                                    : undefined
                                            }
                                        >
                                            <FieldLabel htmlFor="automation-name">
                                                {t(
                                                    'automations.inspector.name',
                                                )}
                                            </FieldLabel>
                                            <Input
                                                id="automation-name"
                                                maxLength={120}
                                                value={name}
                                                placeholder={t(
                                                    'automations.inspector.namePlaceholder',
                                                )}
                                                aria-invalid={
                                                    server.other.name
                                                        ? true
                                                        : undefined
                                                }
                                                onChange={(e) => {
                                                    setName(e.target.value);
                                                    if (server.other.name) {
                                                        setServer((s) => {
                                                            const other = {
                                                                ...s.other,
                                                            };
                                                            delete other.name;
                                                            return {
                                                                ...s,
                                                                other,
                                                            };
                                                        });
                                                    }
                                                }}
                                            />
                                            <FieldMessage
                                                id="automation-name"
                                                error={server.other.name}
                                            />
                                        </Field>
                                        <Field>
                                            <FieldLabel htmlFor="automation-description">
                                                {t(
                                                    'automations.inspector.description',
                                                )}
                                            </FieldLabel>
                                            <Textarea
                                                id="automation-description"
                                                maxLength={500}
                                                rows={2}
                                                value={description}
                                                onChange={(e) =>
                                                    setDescription(
                                                        e.target.value,
                                                    )
                                                }
                                            />
                                            <FieldMessage
                                                id="automation-description"
                                                error={server.other.description}
                                            />
                                        </Field>
                                        <Field
                                            orientation="horizontal"
                                            data-invalid={
                                                server.other.active
                                                    ? true
                                                    : undefined
                                            }
                                        >
                                            <Switch
                                                id="automation-active"
                                                checked={active}
                                                disabled={
                                                    setupBlocks && !active
                                                }
                                                onCheckedChange={(v) => {
                                                    setActive(v);
                                                    if (server.other.active) {
                                                        setServer((s) => {
                                                            const other = {
                                                                ...s.other,
                                                            };
                                                            delete other.active;
                                                            return {
                                                                ...s,
                                                                other,
                                                            };
                                                        });
                                                    }
                                                }}
                                            />
                                            <div>
                                                <FieldLabel htmlFor="automation-active">
                                                    {t(
                                                        'automations.inspector.active',
                                                    )}
                                                </FieldLabel>
                                                <FieldDescription>
                                                    {setupBlocks
                                                        ? t(
                                                              'automations.setup.blocksActive',
                                                          )
                                                        : t(
                                                              'automations.inspector.activeHint',
                                                          )}
                                                </FieldDescription>
                                                <FieldMessage
                                                    id="automation-active"
                                                    error={server.other.active}
                                                />
                                            </div>
                                        </Field>
                                        {setup.length > 0 && (
                                            <div
                                                className="space-y-1 rounded-lg border border-sky-500/40 bg-sky-500/10 p-3 text-xs text-sky-800 dark:text-sky-300"
                                                data-testid="setup-list"
                                            >
                                                <p className="font-medium">
                                                    {t(
                                                        'automations.setup.title',
                                                    )}
                                                </p>
                                                <ul className="list-disc space-y-0.5 pl-4">
                                                    {setup.map((s) => (
                                                        <li
                                                            key={`${s.step_id}-${s.field}`}
                                                        >
                                                            <button
                                                                type="button"
                                                                className="text-left underline-offset-2 hover:underline"
                                                                onClick={() =>
                                                                    focusNode(
                                                                        s.step_id,
                                                                    )
                                                                }
                                                            >
                                                                {s.label}
                                                            </button>
                                                        </li>
                                                    ))}
                                                </ul>
                                            </div>
                                        )}
                                    </FieldGroup>
                                </fieldset>
                                <div className="space-y-2 rounded-lg border bg-muted/30 p-3 text-xs text-muted-foreground">
                                    <p className="font-medium text-foreground">
                                        {t('automations.inspector.howTo')}
                                    </p>
                                    <ul className="list-disc space-y-1 pl-4">
                                        <li>
                                            {t('automations.inspector.tipAdd')}
                                        </li>
                                        <li>
                                            {t(
                                                'automations.inspector.tipConnect',
                                            )}
                                        </li>
                                        <li>
                                            {t(
                                                'automations.inspector.tipBranch',
                                            )}
                                        </li>
                                        <li>
                                            {t('automations.inspector.tipKeys')}
                                        </li>
                                    </ul>
                                    <p>
                                        {t('automations.inspector.count', {
                                            count: tree.count,
                                            max: MAX_STEPS,
                                        })}
                                    </p>
                                </div>
                            </div>
                        )}
                    </aside>
                </div>
            </div>
        </form>
    );
}

function Panel({
    title,
    icon,
    onBack,
    actions,
    invalid,
    children,
    t,
}: {
    title: string;
    icon: React.ReactNode;
    onBack: () => void;
    actions?: React.ReactNode;
    invalid?: boolean;
    children: React.ReactNode;
    t: Translate;
}) {
    return (
        <div>
            <div className="sticky top-0 z-10 flex items-center gap-2 border-b bg-card/95 px-3 py-2 backdrop-blur">
                <Button
                    type="button"
                    size="icon-sm"
                    variant="ghost"
                    aria-label={t('automations.inspector.back')}
                    onClick={onBack}
                >
                    <ArrowLeft />
                </Button>
                <span
                    className={cn(
                        'flex size-7 items-center justify-center rounded-md bg-muted',
                        invalid && 'bg-destructive/10 text-destructive',
                    )}
                >
                    {icon}
                </span>
                <h2 className="min-w-0 flex-1 truncate text-sm font-semibold">
                    {title}
                </h2>
                {actions}
            </div>
            <div className="p-4">{children}</div>
        </div>
    );
}

function StepInspector({
    node,
    edit,
    serverError,
    problems,
    open,
    setupItems,
    options,
    readOnly,
    t,
}: {
    node: StepNode;
    edit: StepPatch;
    serverError?: StepError;
    problems: string[];
    open?: ReadonlySet<string>;
    setupItems?: { field: string; label: string }[];
    options: EditorOptions;
    readOnly: boolean;
    t: Translate;
}) {
    const step = node.data.step;
    const fieldErrors: StepError[] = [
        ...(serverError ? [serverError] : []),
        ...stepProblems(step, t, open),
    ];
    // Problems no field shows (not connected, nesting, no branch …).
    const shownByFields = new Set(fieldErrors.map((e) => e.message));
    const general = problems.filter((p) => !shownByFields.has(p));
    if (serverError && serverError.field === '' && step.type !== 'send_email') {
        general.unshift(serverError.message);
    }
    return (
        <div className="space-y-4">
            <p className="text-xs text-muted-foreground">
                {t(mk(`automations.steps.${step.type}.hint`))}
            </p>
            {isAIStep(step.type) && options.aiEnabled === false && (
                <Alert>
                    <AlertTriangle />
                    <AlertDescription>
                        {t('automations.node.aiOffLong')}{' '}
                        <Link
                            href="/settings/ai"
                            className="font-medium underline underline-offset-2"
                        >
                            {t('automations.node.aiSettings')}
                        </Link>
                    </AlertDescription>
                </Alert>
            )}
            {setupItems?.map((s) => (
                <Alert key={s.field}>
                    <Wrench />
                    <AlertDescription>
                        <span className="font-medium">
                            {t('automations.node.setupNeeded')}
                        </span>
                        {`: ${s.label}`}
                    </AlertDescription>
                </Alert>
            ))}
            {general.length > 0 && (
                <Alert variant="destructive">
                    <AlertCircle />
                    <AlertDescription data-testid="step-error">
                        {[...new Set(general)].join(' ')}
                    </AlertDescription>
                </Alert>
            )}
            <StepFields
                step={step}
                edit={edit}
                errors={fieldErrors}
                options={options}
                readOnly={readOnly}
            />
        </div>
    );
}

/** The nearest spot below `p` where a new node covers no existing one. */
function freeSpot(
    p: { x: number; y: number },
    nodes: {
        position: { x: number; y: number };
        measured?: { width?: number; height?: number };
    }[],
): { x: number; y: number } {
    const pad = 24;
    const hit = (y: number) =>
        nodes.some((n) => {
            const w = n.measured?.width ?? NODE_WIDTH;
            const h = n.measured?.height ?? NODE_HEIGHT;
            return (
                p.x < n.position.x + w + pad &&
                p.x + NODE_WIDTH + pad > n.position.x &&
                y < n.position.y + h + pad &&
                y + NODE_HEIGHT + pad > n.position.y
            );
        });
    let y = p.y;
    for (let i = 0; i < 200 && hit(y); i++) y += 24;
    return { x: p.x, y };
}
