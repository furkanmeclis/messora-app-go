/**
 * The canvas graph and its conversion to and from the step tree the engine
 * runs. The graph is always a tree hanging from the trigger:
 *
 * - every node but the trigger has exactly one incoming edge;
 * - plain steps have one output ("out", the next step);
 * - conditions have three: "yes" (then), "no" (else) and "next" (what
 *   runs after either branch: the steps after the condition in its list);
 * - each output holds at most one edge, and edges never close a cycle.
 *
 * So `[A, if C then [B] else [D], E]` is trigger→A, A→C, C-yes→B, C-no→D,
 * C-next→E. The conversion is lossless both ways.
 */
import type { Edge, Node, XYPosition } from '@xyflow/react';
import type { Translate } from '@/i18n/translate';
import type { AutomationStep, Schemas } from '@/lib/types';
import {
    type EditorStep,
    MAX_DEPTH,
    MAX_STEPS,
    stepFromApi,
    stepProblems,
    stepToApi,
} from './model';

export const TRIGGER_ID = 'trigger';

export type Output = 'out' | 'yes' | 'no' | 'next';
export const CONDITION_OUTPUTS: Output[] = ['yes', 'no', 'next'];

export type StepNode = Node<{ step: EditorStep }, 'step'>;
export type TriggerNode = Node<Record<string, never>, 'trigger'>;
export type FlowNode = StepNode | TriggerNode;
export type FlowEdge = Edge<Record<string, never>, 'flow'>;

export type Layout = Schemas['AutomationLayout'];

/** Approximate node size, for the automatic layout. */
export const NODE_WIDTH = 272;
export const NODE_HEIGHT = 96;
const GAP_X = 48;
const GAP_Y = 64;

export function outputsOf(node: FlowNode | undefined): Output[] {
    if (!node) return [];
    if (node.type === 'step' && node.data.step.type === 'condition') {
        return CONDITION_OUTPUTS;
    }
    return ['out'];
}

export function edgeId(source: string, output: string): string {
    return `${source}:${output}`;
}

export function makeEdge(
    source: string,
    output: Output,
    target: string,
): FlowEdge {
    return {
        id: edgeId(source, output),
        source,
        sourceHandle: output,
        target,
        targetHandle: 'in',
        type: 'flow',
        data: {},
    };
}

// ── steps → graph ──────────────────────────────────────────────────────────

/**
 * Builds the canvas from stored steps. Steps without an id (never sent by
 * the API, which names them) get a positional one. Positions come from the
 * layout; nodes it does not place are laid out automatically.
 */
export function graphFromSteps(
    steps: AutomationStep[] | undefined,
    layout: Layout | undefined,
): { nodes: FlowNode[]; edges: FlowEdge[] } {
    const nodes: FlowNode[] = [
        {
            id: TRIGGER_ID,
            type: 'trigger',
            position: { x: 0, y: 0 },
            data: {},
            deletable: false,
        },
    ];
    const edges: FlowEdge[] = [];
    const seen = new Set<string>([TRIGGER_ID]);

    const chain = (
        list: AutomationStep[] | undefined,
        from: string,
        output: Output,
        prefix: string,
    ) => {
        let source = from;
        let out = output;
        (list ?? []).forEach((raw, i) => {
            let id = raw.id && !seen.has(raw.id) ? raw.id : `${prefix}${i}`;
            while (seen.has(id)) id = `${id}_`;
            seen.add(id);
            const step = stepFromApi(raw, id);
            nodes.push({
                id,
                type: 'step',
                position: { x: 0, y: 0 },
                data: { step },
            });
            edges.push(makeEdge(source, out, id));
            if (step.type === 'condition') {
                const r = raw as Record<string, unknown>;
                chain(r.then as AutomationStep[], id, 'yes', `${id}t`);
                chain(r.else as AutomationStep[], id, 'no', `${id}e`);
                source = id;
                out = 'next';
            } else {
                source = id;
                out = 'out';
            }
        });
    };
    chain(steps, TRIGGER_ID, 'out', 's');

    const auto = autoLayout(nodes, edges);
    const placed = layout ?? {};
    for (const n of nodes) {
        // A partly stored layout (e.g. steps added through the API) keeps
        // what it has; the rest follows the automatic layout.
        const p = placed[n.id];
        n.position = p ? { x: p.x, y: p.y } : auto[n.id]!;
    }
    return { nodes, edges };
}

// ── graph → steps ──────────────────────────────────────────────────────────

export type Tree = {
    /** The steps as the API takes them (ids included). */
    steps: AutomationStep[];
    /** Nodes not reachable from the trigger. */
    orphans: string[];
    /** Depth of the step list each node sits in (root list = 1). */
    depth: Map<string, number>;
    /** Steps counted like the API does. */
    count: number;
};

/** Index of the edges leaving each node, by output. */
export function outgoing(edges: FlowEdge[]): Map<string, Map<string, string>> {
    const out = new Map<string, Map<string, string>>();
    for (const e of edges) {
        const m = out.get(e.source) ?? new Map<string, string>();
        m.set(e.sourceHandle ?? 'out', e.target);
        out.set(e.source, m);
    }
    return out;
}

export function graphToSteps(nodes: FlowNode[], edges: FlowEdge[]): Tree {
    const byId = new Map(nodes.map((n) => [n.id, n]));
    const out = outgoing(edges);
    const visited = new Set<string>([TRIGGER_ID]);
    const depth = new Map<string, number>();
    let count = 0;

    const chain = (
        start: string | undefined,
        level: number,
    ): AutomationStep[] => {
        const list: AutomationStep[] = [];
        let cur = start;
        while (cur && !visited.has(cur)) {
            const node = byId.get(cur);
            if (!node || node.type !== 'step') break;
            visited.add(cur);
            depth.set(cur, level);
            count += 1;
            const step = stepToApi(node.data.step);
            const outs = out.get(cur);
            if (node.data.step.type === 'condition') {
                const then = chain(outs?.get('yes'), level + 1);
                const els = chain(outs?.get('no'), level + 1);
                if (then.length) step.then = then;
                if (els.length) step.else = els;
                cur = outs?.get('next');
            } else {
                cur = outs?.get('out');
            }
            list.push(step);
        }
        return list;
    };
    const steps = chain(out.get(TRIGGER_ID)?.get('out'), 1);
    const orphans = nodes.filter((n) => !visited.has(n.id)).map((n) => n.id);
    return { steps, orphans, depth, count };
}

// ── Connections ────────────────────────────────────────────────────────────

/**
 * Whether an edge from source/output to target keeps the graph a tree:
 * the output is free, the target has no incoming edge yet and is not the
 * trigger, and the target is not an ancestor of the source (no cycle).
 */
export function canConnect(
    nodes: FlowNode[],
    edges: FlowEdge[],
    c: { source: string; sourceHandle?: string | null; target: string },
): boolean {
    const output = c.sourceHandle ?? 'out';
    if (!c.target || c.target === TRIGGER_ID || c.source === c.target) {
        return false;
    }
    const source = nodes.find((n) => n.id === c.source);
    if (!outputsOf(source).includes(output as Output)) return false;
    if (
        edges.some(
            (e) =>
                e.source === c.source && (e.sourceHandle ?? 'out') === output,
        )
    ) {
        return false;
    }
    if (edges.some((e) => e.target === c.target)) return false;
    // Walk up from the source: each node has at most one parent.
    const parent = new Map(edges.map((e) => [e.target, e.source]));
    for (let n: string | undefined = c.source; n; n = parent.get(n)) {
        if (n === c.target) return false;
    }
    return true;
}

/** The first free output of a node, in the order yes, no, next / out. */
export function freeOutput(
    node: FlowNode | undefined,
    edges: FlowEdge[],
): Output | null {
    for (const o of outputsOf(node)) {
        if (
            !edges.some(
                (e) => e.source === node!.id && (e.sourceHandle ?? 'out') === o,
            )
        ) {
            return o;
        }
    }
    return null;
}

// ── Automatic layout ───────────────────────────────────────────────────────

type Box = { width: number; height: number };

/**
 * Top-to-bottom tree layout: a step list is a column; a condition puts its
 * yes and no branches side by side under it and continues ("next") below
 * the taller branch. Unconnected nodes line up to the right of the tree.
 */
export function autoLayout(
    nodes: FlowNode[],
    edges: FlowEdge[],
): Record<string, XYPosition> {
    const byId = new Map(nodes.map((n) => [n.id, n]));
    const out = outgoing(edges);
    const pos: Record<string, XYPosition> = {};
    const placed = new Set<string>();

    const isCondition = (id: string) => {
        const n = byId.get(id);
        return n?.type === 'step' && n.data.step.type === 'condition';
    };
    const nextOf = (id: string) =>
        out.get(id)?.get(isCondition(id) ? 'next' : 'out');

    // Measure a chain starting at id (guarding against malformed input).
    const measureMemo = new Map<string, Box>();
    const measure = (start: string | undefined, guard: Set<string>): Box => {
        let width = 0;
        let height = 0;
        for (
            let id = start;
            id && byId.has(id) && !guard.has(id);
            id = nextOf(id)
        ) {
            guard.add(id);
            let w = NODE_WIDTH;
            let h = NODE_HEIGHT;
            if (isCondition(id)) {
                const yes = measure(out.get(id)?.get('yes'), guard);
                const no = measure(out.get(id)?.get('no'), guard);
                const yw = Math.max(yes.width, NODE_WIDTH);
                const nw = Math.max(no.width, NODE_WIDTH);
                w = Math.max(NODE_WIDTH, yw + GAP_X + nw);
                const branch = Math.max(yes.height, no.height);
                h = NODE_HEIGHT + (branch ? GAP_Y + branch : 0);
            }
            measureMemo.set(id, { width: w, height: h });
            width = Math.max(width, w);
            height += (height ? GAP_Y : 0) + h;
        }
        return { width, height };
    };

    const place = (start: string | undefined, cx: number, y: number) => {
        let top = y;
        for (
            let id = start;
            id && byId.has(id) && !placed.has(id);
            id = nextOf(id)
        ) {
            placed.add(id);
            pos[id] = { x: cx - NODE_WIDTH / 2, y: top };
            const box = measureMemo.get(id) ?? {
                width: NODE_WIDTH,
                height: NODE_HEIGHT,
            };
            if (isCondition(id)) {
                const yes = out.get(id)?.get('yes');
                const no = out.get(id)?.get('no');
                const yw = Math.max(
                    measure(yes, new Set(placed)).width,
                    NODE_WIDTH,
                );
                const nw = Math.max(
                    measure(no, new Set(placed)).width,
                    NODE_WIDTH,
                );
                const left = cx - (yw + GAP_X + nw) / 2;
                const by = top + NODE_HEIGHT + GAP_Y;
                place(yes, left + yw / 2, by);
                place(no, left + yw + GAP_X + nw / 2, by);
            }
            top += box.height + GAP_Y;
        }
        return top;
    };

    pos[TRIGGER_ID] = { x: -NODE_WIDTH / 2, y: 0 };
    placed.add(TRIGGER_ID);
    const first = out.get(TRIGGER_ID)?.get('out');
    const tree = measure(first, new Set([TRIGGER_ID]));
    place(first, 0, NODE_HEIGHT + GAP_Y);

    // Whatever is not connected to the trigger: each loose subtree in a
    // column to the right.
    let x = Math.max(tree.width, NODE_WIDTH) / 2 + GAP_X * 2;
    const targets = new Set(edges.map((e) => e.target));
    for (const n of nodes) {
        if (placed.has(n.id) || targets.has(n.id)) continue;
        const box = measure(n.id, new Set(placed));
        place(n.id, x + box.width / 2, NODE_HEIGHT + GAP_Y);
        x += box.width + GAP_X * 2;
    }
    // Nodes in a cycle or behind a broken edge.
    for (const n of nodes) {
        if (!pos[n.id]) {
            pos[n.id] = { x, y: NODE_HEIGHT + GAP_Y };
            x += NODE_WIDTH + GAP_X;
        }
    }
    return pos;
}

// ── Validation ─────────────────────────────────────────────────────────────

export type Problems = {
    /** Messages per node id. */
    nodes: Record<string, string[]>;
    /** Problems of the whole automation. */
    global: string[];
};

/** Structural and field problems, shown on the nodes while editing. */
export function validateGraph(
    nodes: FlowNode[],
    edges: FlowEdge[],
    t: Translate,
): Problems {
    const tree = graphToSteps(nodes, edges);
    const problems: Problems = { nodes: {}, global: [] };
    const add = (id: string, message: string) =>
        (problems.nodes[id] ??= []).push(message);
    if (tree.count === 0)
        problems.global.push(t('automations.problems.noSteps'));
    if (nodes.length - 1 > MAX_STEPS) {
        problems.global.push(
            t('automations.problems.tooMany', { max: MAX_STEPS }),
        );
    }
    for (const id of tree.orphans) add(id, t('automations.problems.orphan'));
    const out = outgoing(edges);
    for (const n of nodes) {
        if (n.type !== 'step') continue;
        const step = n.data.step;
        const level = tree.depth.get(n.id);
        if (level && level > MAX_DEPTH) {
            add(n.id, t('automations.problems.tooDeep', { max: MAX_DEPTH }));
        }
        if (step.type === 'condition') {
            const outs = out.get(n.id);
            if (!outs?.get('yes') && !outs?.get('no')) {
                add(n.id, t('automations.problems.noBranch'));
            }
        }
        for (const p of stepProblems(step, t)) add(n.id, p.message);
    }
    return problems;
}
