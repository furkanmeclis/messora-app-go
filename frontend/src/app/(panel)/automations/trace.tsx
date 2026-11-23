'use client';

import { useMemo, useState } from 'react';
import { ReactFlowProvider, useEdgesState, useNodesState } from '@xyflow/react';
import { AlertTriangle, Info } from 'lucide-react';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { useT } from '@/i18n/client';
import type { Automation, AutomationRun, AutomationRunStep } from '@/lib/types';
import { cn } from '@/lib/utils';
import { FlowCanvas } from './canvas';
import { graphFromSteps, TRIGGER_ID } from './graph';
import { describeTrigger, stepLabel, triggerLabel } from './model';
import {
    CanvasContext,
    type CanvasContextValue,
    type TraceState,
} from './nodes';
import { RunSummary, StatusBadge, TimelineItem } from './[id]/runs/runs-client';

type Names = {
    phones: { id: string; label: string }[];
    campaigns: { id: string; label: string }[];
};

/**
 * A run drawn on the automation's canvas (read-only): the nodes it went
 * through are numbered in execution order, the path between them flows,
 * and the node where it failed or waits is marked.
 */
export function RunTrace(props: {
    automation: Automation;
    run: AutomationRun;
    names: Names;
}) {
    return (
        <ReactFlowProvider>
            <Trace {...props} />
        </ReactFlowProvider>
    );
}

function Trace({
    automation,
    run,
    names,
}: {
    automation: Automation;
    run: AutomationRun;
    names: Names;
}) {
    const t = useT();
    const graph = useMemo(
        () => graphFromSteps(automation.steps, automation.layout),
        [automation.steps, automation.layout],
    );
    const [nodes, , onNodesChange] = useNodesState(graph.nodes);
    const [edges, , onEdgesChange] = useEdgesState(graph.edges);
    const [focus, setFocus] = useState<string | null>(null);

    const visited = useMemo(() => run.visited ?? [], [run.visited]);
    const known = new Set(graph.nodes.map((n) => n.id));
    const missing = visited.some((id) => !known.has(id));

    const trace: TraceState = useMemo(() => {
        const order = new Map<string, number[]>();
        visited.forEach((id, i) =>
            order.set(id, [...(order.get(id) ?? []), i + 1]),
        );
        const last = visited.at(-1);
        const kind =
            run.status === 'failed'
                ? 'failed'
                : run.status === 'waiting'
                  ? 'waiting'
                  : run.status === 'running' || run.status === 'queued'
                    ? 'running'
                    : undefined;
        return {
            order,
            stop: kind && last ? { id: last, kind } : undefined,
        };
    }, [visited, run.status]);

    const ctx: CanvasContextValue = useMemo(
        () => ({
            t,
            problems: {},
            triggerLabel: triggerLabel(automation.trigger_type, t),
            triggerSummary: describeTrigger(
                automation.trigger_type,
                automation.trigger,
                t,
                {
                    phones: new Map(names.phones.map((p) => [p.id, p.label])),
                    campaigns: new Map(
                        names.campaigns.map((c) => [c.id, c.label]),
                    ),
                },
            ),
            lookup: { groups: new Map(), users: new Map() },
            readOnly: true,
            trace,
        }),
        [t, automation, names, trace],
    );

    // Run log entries and visited ids are written together, one per
    // executed instruction, so they line up by index.
    const steps = run.steps ?? [];
    const entries: { id?: string; step: AutomationRunStep }[] = steps.map(
        (step, i) => ({ id: visited[i], step }),
    );
    const shown = focus ? entries.filter((e) => e.id === focus) : entries;
    const focusNode = graph.nodes.find((n) => n.id === focus);

    return (
        <div className="space-y-3">
            {visited.length === 0 && (
                <Alert>
                    <Info />
                    <AlertDescription>
                        {t('automations.trace.noTrace')}
                    </AlertDescription>
                </Alert>
            )}
            {missing && (
                <Alert>
                    <AlertTriangle />
                    <AlertDescription>
                        {t('automations.trace.changed')}
                    </AlertDescription>
                </Alert>
            )}
            <div
                data-testid="run-canvas"
                className="automation-canvas flex h-[calc(100dvh-14rem)] min-h-[520px] flex-col overflow-hidden rounded-xl border bg-card shadow-xs lg:flex-row"
            >
                <div className="relative min-h-[360px] min-w-0 flex-1">
                    <CanvasContext value={ctx}>
                        <FlowCanvas
                            nodes={nodes}
                            edges={edges}
                            onNodesChange={onNodesChange}
                            onEdgesChange={onEdgesChange}
                            nodesDraggable={false}
                            nodesConnectable={false}
                            edgesReconnectable={false}
                            deleteKeyCode={null}
                            onNodeClick={(_, n) =>
                                setFocus(n.id === TRIGGER_ID ? null : n.id)
                            }
                            onPaneClick={() => setFocus(null)}
                        />
                    </CanvasContext>
                    <Legend />
                </div>
                <aside className="max-h-[50vh] w-full shrink-0 space-y-5 overflow-y-auto border-t p-4 lg:max-h-none lg:w-[22rem] lg:border-t-0 lg:border-l">
                    <div className="flex items-center justify-between gap-2">
                        <h2 className="text-sm font-semibold">
                            {t('automations.trace.run')}
                        </h2>
                        <StatusBadge status={run.status} />
                    </div>
                    <RunSummary run={run} />
                    <section
                        aria-labelledby="trace-steps"
                        className="space-y-3"
                    >
                        <div className="flex items-baseline justify-between gap-2">
                            <h3
                                id="trace-steps"
                                className="text-sm font-medium"
                            >
                                {focusNode?.type === 'step'
                                    ? stepLabel(focusNode.data.step.type, t)
                                    : t('automations.runs.executedSteps')}
                            </h3>
                            {focus && (
                                <button
                                    type="button"
                                    className="text-xs text-muted-foreground underline-offset-4 hover:underline"
                                    onClick={() => setFocus(null)}
                                >
                                    {t('automations.trace.showAll')}
                                </button>
                            )}
                        </div>
                        {shown.length === 0 ? (
                            <p className="text-sm text-muted-foreground">
                                {focus
                                    ? t('automations.trace.notReached')
                                    : t('automations.runs.noStepYet')}
                            </p>
                        ) : (
                            <ol className="relative space-y-4 border-l pl-5">
                                {shown.map((e, i) => (
                                    <TimelineItem
                                        key={`${e.step.pc}-${i}`}
                                        step={e.step}
                                        onSelect={
                                            e.id && known.has(e.id)
                                                ? () => setFocus(e.id!)
                                                : undefined
                                        }
                                    />
                                ))}
                            </ol>
                        )}
                    </section>
                </aside>
            </div>
        </div>
    );
}

function Legend() {
    const t = useT();
    const item = (cls: string, label: string) => (
        <span className="flex items-center gap-1.5">
            <span className={cn('size-2.5 rounded-full', cls)} />
            {label}
        </span>
    );
    return (
        <div className="pointer-events-none absolute top-3 left-3 flex flex-wrap gap-3 rounded-lg border bg-card/95 px-3 py-1.5 text-xs text-muted-foreground shadow-sm">
            {item('bg-primary', t('automations.trace.legend.visited'))}
            {item(
                'bg-muted-foreground/40',
                t('automations.trace.legend.skipped'),
            )}
            {item('bg-amber-500', t('automations.trace.legend.waiting'))}
            {item('bg-destructive', t('automations.trace.legend.failed'))}
        </div>
    );
}
