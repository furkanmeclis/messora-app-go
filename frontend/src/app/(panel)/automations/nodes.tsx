'use client';

import { createContext, memo, use } from 'react';
import {
    BaseEdge,
    EdgeLabelRenderer,
    type EdgeProps,
    getSmoothStepPath,
    Handle,
    type NodeProps,
    Position,
} from '@xyflow/react';
import {
    AlertCircle,
    BellOff,
    CircleCheck,
    Clock,
    FileText,
    GitBranch,
    Globe,
    Hourglass,
    Mail,
    MessageSquare,
    PenLine,
    Puzzle,
    Tag,
    TagsIcon,
    UserCheck,
    UsersRound,
    X,
    XCircle,
    Zap,
} from 'lucide-react';
import type { Translate } from '@/i18n/translate';
import { cn } from '@/lib/utils';
import type { FlowEdge, Output, StepNode, TriggerNode } from './graph';
import {
    type Category,
    mk,
    stepCategory,
    stepLabel,
    summarizeStep,
} from './model';

export const STEP_ICONS: Record<
    string,
    React.ComponentType<{ className?: string }>
> = {
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

export function StepIcon({
    type,
    className,
}: {
    type: string;
    className?: string;
}) {
    const Icon = STEP_ICONS[type] ?? Puzzle;
    return <Icon className={className} />;
}

/** Accent colours per category; surfaces use the theme tokens. */
export const CATEGORY_STYLE: Record<
    Category,
    { chip: string; bar: string; dot: string }
> = {
    trigger: {
        chip: 'bg-primary text-primary-foreground',
        bar: 'bg-primary',
        dot: 'bg-primary',
    },
    message: {
        chip: 'bg-sky-500/15 text-sky-700 dark:text-sky-300',
        bar: 'bg-sky-500',
        dot: 'bg-sky-500',
    },
    data: {
        chip: 'bg-violet-500/15 text-violet-700 dark:text-violet-300',
        bar: 'bg-violet-500',
        dot: 'bg-violet-500',
    },
    flow: {
        chip: 'bg-amber-500/15 text-amber-700 dark:text-amber-300',
        bar: 'bg-amber-500',
        dot: 'bg-amber-500',
    },
    integration: {
        chip: 'bg-emerald-500/15 text-emerald-700 dark:text-emerald-300',
        bar: 'bg-emerald-500',
        dot: 'bg-emerald-500',
    },
};

/** Where a node stands in a run being viewed. */
export type TraceState = {
    /** Execution order of each visited node (1-based). */
    order: Map<string, number[]>;
    /** The node the run stopped at, and why. */
    stop?: { id: string; kind: 'failed' | 'waiting' | 'running' };
};

export type CanvasContextValue = {
    t: Translate;
    problems: Record<string, string[]>;
    triggerLabel: string;
    triggerSummary: string;
    lookup: { groups: Map<string, string>; users: Map<string, string> };
    readOnly: boolean;
    trace?: TraceState;
    onDeleteEdge?: (id: string) => void;
};

export const CanvasContext = createContext<CanvasContextValue | null>(null);

function useCanvas(): CanvasContextValue {
    const v = use(CanvasContext);
    if (!v) throw new Error('CanvasContext missing');
    return v;
}

const HANDLE =
    'size-3! rounded-full! border-2! border-card! transition-transform hover:scale-125';

function TraceBadge({ id }: { id: string }) {
    const { trace, t } = useCanvas();
    const order = trace?.order.get(id);
    if (!trace) return null;
    const stop = trace.stop?.id === id ? trace.stop.kind : undefined;
    if (stop === 'failed' || stop === 'waiting' || stop === 'running') {
        const Icon = stop === 'failed' ? XCircle : Hourglass;
        return (
            <span
                className={cn(
                    'absolute -top-2.5 -right-2.5 flex h-6 items-center gap-1 rounded-full px-2 text-[11px] font-semibold shadow-sm',
                    stop === 'failed'
                        ? 'bg-destructive text-white'
                        : 'bg-amber-500 text-white',
                )}
            >
                <Icon className="size-3.5" />
                {t(mk(`automations.trace.stop.${stop}`))}
            </span>
        );
    }
    if (!order?.length) return null;
    return (
        <span className="absolute -top-2.5 -right-2.5 flex h-6 min-w-6 items-center justify-center rounded-full bg-primary px-1.5 text-[11px] font-semibold text-primary-foreground tabular-nums shadow-sm">
            {order.join(', ')}
        </span>
    );
}

function Shell({
    id,
    selected,
    category,
    icon,
    kicker,
    title,
    summary,
    children,
    testType,
}: {
    id: string;
    selected: boolean;
    category: Category;
    icon: React.ReactNode;
    kicker: string;
    title: string;
    summary: string;
    children?: React.ReactNode;
    testType: string;
}) {
    const { problems, trace, t } = useCanvas();
    const errors = problems[id] ?? [];
    const style = CATEGORY_STYLE[category];
    const visited = !trace || trace.order.has(id) || id === 'trigger';
    const stop = trace?.stop?.id === id ? trace.stop.kind : undefined;
    return (
        <div
            data-testid="canvas-node"
            data-node-id={id}
            data-step-type={testType}
            data-invalid={errors.length ? 'true' : undefined}
            data-visited={trace ? String(visited) : undefined}
            className={cn(
                'group relative w-[272px] rounded-xl border bg-card text-card-foreground shadow-sm transition-[box-shadow,border-color,opacity]',
                'hover:shadow-md',
                errors.length > 0 &&
                    'border-destructive ring-2 ring-destructive/25',
                // Selection stays visible on an invalid node (ring only).
                selected &&
                    (errors.length > 0
                        ? 'ring-4 ring-primary/45'
                        : 'border-primary ring-2 ring-primary/40'),
                trace && !visited && 'opacity-45 saturate-50',
                trace && visited && !stop && 'border-primary/60',
                stop === 'failed' &&
                    'border-destructive ring-2 ring-destructive/40',
                (stop === 'waiting' || stop === 'running') &&
                    'border-amber-500 ring-2 ring-amber-500/40',
            )}
        >
            <span
                aria-hidden
                className={cn(
                    'absolute inset-y-3 left-0 w-1 rounded-r-full',
                    style.bar,
                )}
            />
            <div className="flex items-start gap-3 p-3 pl-4">
                <span
                    className={cn(
                        'flex size-9 shrink-0 items-center justify-center rounded-lg',
                        style.chip,
                    )}
                >
                    {icon}
                </span>
                <div className="min-w-0 flex-1">
                    <div className="text-[10px] font-medium tracking-wider text-muted-foreground uppercase">
                        {kicker}
                    </div>
                    <div className="truncate text-sm leading-tight font-semibold">
                        {title}
                    </div>
                    <p className="mt-0.5 line-clamp-2 text-xs break-words text-muted-foreground">
                        {summary}
                    </p>
                </div>
            </div>
            {children}
            {errors.length > 0 && (
                <div
                    className="flex items-start gap-1.5 rounded-b-xl border-t border-destructive/25 bg-destructive/5 px-3 py-1.5 text-xs text-destructive"
                    data-testid="node-error"
                >
                    <AlertCircle className="mt-px size-3.5 shrink-0" />
                    <span className="line-clamp-2">
                        {errors[0]}
                        {errors.length > 1 &&
                            ` ${t('automations.node.more', { count: errors.length - 1 })}`}
                    </span>
                </div>
            )}
            <TraceBadge id={id} />
        </div>
    );
}

export const TriggerNodeView = memo(function TriggerNodeView({
    id,
    selected,
}: NodeProps<TriggerNode>) {
    const { t, triggerLabel, triggerSummary, readOnly } = useCanvas();
    return (
        <>
            <Shell
                id={id}
                selected={!!selected}
                category="trigger"
                icon={<Zap className="size-4" />}
                kicker={t('automations.node.trigger')}
                title={triggerLabel}
                summary={triggerSummary}
                testType="trigger"
            />
            <Handle
                type="source"
                position={Position.Bottom}
                id="out"
                isConnectable={!readOnly}
                className={cn(HANDLE, 'bg-primary!')}
            />
        </>
    );
});

const OUTPUT_STYLE: Record<Output, { handle: string; label: string }> = {
    out: { handle: 'bg-muted-foreground!', label: '' },
    yes: {
        handle: 'bg-emerald-500!',
        label: 'text-emerald-700 dark:text-emerald-400',
    },
    no: { handle: 'bg-rose-500!', label: 'text-rose-700 dark:text-rose-400' },
    next: { handle: 'bg-muted-foreground!', label: 'text-muted-foreground' },
};

export const StepNodeView = memo(function StepNodeView({
    id,
    data,
    selected,
}: NodeProps<StepNode>) {
    const { t, lookup, readOnly } = useCanvas();
    const step = data.step;
    const category = stepCategory(step.type);
    const isCondition = step.type === 'condition';
    return (
        <>
            <Handle
                type="target"
                position={Position.Top}
                id="in"
                isConnectable={!readOnly}
                className={cn(HANDLE, 'bg-muted-foreground!')}
            />
            <Shell
                id={id}
                selected={!!selected}
                category={category}
                icon={<StepIcon type={step.type} className="size-4" />}
                kicker={t(mk(`automations.categories.${category}`))}
                title={stepLabel(step.type, t)}
                summary={summarizeStep(step, t, lookup)}
                testType={step.type}
            >
                {isCondition && (
                    <div className="grid grid-cols-3 border-t px-1 py-1 text-center text-[10px] font-semibold tracking-wide uppercase">
                        {(['yes', 'next', 'no'] as const).map((o) => (
                            <span key={o} className={OUTPUT_STYLE[o].label}>
                                {t(mk(`automations.outputs.${o}`))}
                            </span>
                        ))}
                    </div>
                )}
            </Shell>
            {isCondition ? (
                (['yes', 'next', 'no'] as const).map((o, i) => (
                    <Handle
                        key={o}
                        type="source"
                        position={Position.Bottom}
                        id={o}
                        isConnectable={!readOnly}
                        aria-label={t(mk(`automations.outputs.${o}`))}
                        style={{ left: `${(i * 2 + 1) * (100 / 6)}%` }}
                        className={cn(HANDLE, OUTPUT_STYLE[o].handle)}
                    />
                ))
            ) : (
                <Handle
                    type="source"
                    position={Position.Bottom}
                    id="out"
                    isConnectable={!readOnly}
                    className={cn(HANDLE, OUTPUT_STYLE.out.handle)}
                />
            )}
        </>
    );
});

const EDGE_STROKE: Record<string, string> = {
    yes: 'var(--flow-yes)',
    no: 'var(--flow-no)',
    next: 'var(--muted-foreground)',
    out: 'var(--muted-foreground)',
};

export const FlowEdgeView = memo(function FlowEdgeView({
    id,
    source,
    target,
    sourceX,
    sourceY,
    targetX,
    targetY,
    sourcePosition,
    targetPosition,
    sourceHandleId,
    selected,
    markerEnd,
}: EdgeProps<FlowEdge>) {
    const { t, trace, readOnly, onDeleteEdge } = useCanvas();
    const output = (sourceHandleId ?? 'out') as Output;
    const [path, labelX, labelY] = getSmoothStepPath({
        sourceX,
        sourceY,
        targetX,
        targetY,
        sourcePosition,
        targetPosition,
        borderRadius: 14,
        offset: 18,
    });
    const visited =
        !!trace &&
        (source === 'trigger' || trace.order.has(source)) &&
        trace.order.has(target);
    const label =
        output === 'out' ? '' : t(mk(`automations.outputs.${output}`));
    return (
        <>
            <BaseEdge
                id={id}
                path={path}
                markerEnd={markerEnd}
                className={cn(visited && 'flow-edge-visited')}
                style={{
                    stroke: visited ? 'var(--primary)' : EDGE_STROKE[output],
                    strokeWidth: visited ? 2.5 : selected ? 2.5 : 1.75,
                    opacity: trace && !visited ? 0.35 : 1,
                    strokeDasharray:
                        output === 'next' && !visited ? '6 4' : undefined,
                }}
            />
            {(label || (selected && !readOnly)) && (
                <EdgeLabelRenderer>
                    <div
                        className="nodrag nopan pointer-events-auto absolute flex items-center gap-1"
                        style={{
                            transform: `translate(-50%, -50%) translate(${labelX}px, ${labelY}px)`,
                        }}
                    >
                        {label && (
                            <span
                                className={cn(
                                    'rounded-full border bg-card px-2 py-0.5 text-[10px] font-semibold tracking-wide uppercase shadow-xs',
                                    OUTPUT_STYLE[output].label,
                                )}
                            >
                                {label}
                            </span>
                        )}
                        {selected && !readOnly && onDeleteEdge && (
                            <button
                                type="button"
                                onClick={() => onDeleteEdge(id)}
                                aria-label={t('automations.canvas.deleteEdge')}
                                className="flex size-5 items-center justify-center rounded-full border bg-card text-muted-foreground shadow-xs hover:text-destructive"
                            >
                                <X className="size-3" />
                            </button>
                        )}
                    </div>
                </EdgeLabelRenderer>
            )}
        </>
    );
});

export const nodeTypes = { trigger: TriggerNodeView, step: StepNodeView };
export const edgeTypes = { flow: FlowEdgeView };
