/**
 * The segment rule tree as the editor holds it, and its conversion to and
 * from the API's definition. Fields, operators and value shapes mirror
 * internal/modules/contacts/segment/segment.go exactly.
 */

export const MAX_DEPTH = 5; // root group = level 1; rules may sit at level 5
export const MAX_RULES = 50;
export const MAX_TEXT = 200;
export const MAX_VALUES = 500;

export type FieldKind = 'text' | 'time' | 'bool' | 'tag' | 'group';

export const FIELDS: { key: string; label: string; kind: FieldKind }[] = [
    { key: 'name', label: 'Name', kind: 'text' },
    { key: 'profile_name', label: 'WhatsApp profile name', kind: 'text' },
    { key: 'phone', label: 'Phone number', kind: 'text' },
    { key: 'email', label: 'E-mail', kind: 'text' },
    { key: 'var1', label: 'Variable 1', kind: 'text' },
    { key: 'var2', label: 'Variable 2', kind: 'text' },
    { key: 'var3', label: 'Variable 3', kind: 'text' },
    { key: 'var4', label: 'Variable 4', kind: 'text' },
    { key: 'var5', label: 'Variable 5', kind: 'text' },
    { key: 'created_at', label: 'Added', kind: 'time' },
    { key: 'last_inbound_at', label: 'Last message from them', kind: 'time' },
    { key: 'opted_out', label: 'Opted out', kind: 'bool' },
    { key: 'tag', label: 'Tag', kind: 'tag' },
    { key: 'group', label: 'Group', kind: 'group' },
];

export const OPERATORS: Record<FieldKind, { key: string; label: string }[]> = {
    text: [
        { key: 'eq', label: 'is' },
        { key: 'neq', label: 'is not' },
        { key: 'contains', label: 'contains' },
        { key: 'not_contains', label: 'does not contain' },
        { key: 'starts_with', label: 'starts with' },
        { key: 'in', label: 'is one of' },
        { key: 'not_in', label: 'is none of' },
        { key: 'is_set', label: 'is filled in' },
        { key: 'is_not_set', label: 'is empty' },
    ],
    time: [
        { key: 'within_days', label: 'in the last … days' },
        { key: 'older_than_days', label: 'more than … days ago' },
        { key: 'before', label: 'before' },
        { key: 'after', label: 'after' },
        { key: 'is_set', label: 'is known' },
        { key: 'is_not_set', label: 'is unknown' },
    ],
    bool: [
        { key: 'is_true', label: 'yes' },
        { key: 'is_false', label: 'no' },
    ],
    tag: [
        { key: 'has', label: 'has' },
        { key: 'not_has', label: 'does not have' },
        { key: 'has_any', label: 'has any of' },
        { key: 'has_none', label: 'has none of' },
    ],
    group: [
        { key: 'in_group', label: 'is in' },
        { key: 'not_in_group', label: 'is not in' },
    ],
};

export type ValueShape = 'none' | 'text' | 'list' | 'days' | 'date' | 'group';

export function valueShape(operator: string): ValueShape {
    switch (operator) {
        case 'is_set':
        case 'is_not_set':
        case 'is_true':
        case 'is_false':
            return 'none';
        case 'in':
        case 'not_in':
        case 'has_any':
        case 'has_none':
            return 'list';
        case 'within_days':
        case 'older_than_days':
            return 'days';
        case 'before':
        case 'after':
            return 'date';
        case 'in_group':
        case 'not_in_group':
            return 'group';
        default:
            return 'text';
    }
}

export function fieldKind(field: string): FieldKind {
    return FIELDS.find((f) => f.key === field)?.kind ?? 'text';
}

export type RuleNode = {
    type: 'rule';
    id: string;
    field: string;
    operator: string;
    /** Always text in the editor; lists are comma separated. */
    value: string;
    not: boolean;
};

export type GroupNode = {
    type: 'group';
    id: string;
    op: 'and' | 'or';
    not: boolean;
    rules: TreeNode[];
};

export type TreeNode = RuleNode | GroupNode;

// Ids of nodes added in the browser; loaded trees number their nodes
// themselves so server and client render the same ids.
let seq = 0;
export const newId = () => `u${++seq}`;

export function newRule(id = newId()): RuleNode {
    return {
        type: 'rule',
        id,
        field: 'tag',
        operator: 'has',
        value: '',
        not: false,
    };
}

export function newGroup(): GroupNode {
    return {
        type: 'group',
        id: newId(),
        op: 'and',
        not: false,
        rules: [newRule()],
    };
}

type Raw = {
    op?: unknown;
    not?: unknown;
    rules?: unknown;
    field?: unknown;
    operator?: unknown;
    value?: unknown;
};

function fromRaw(raw: Raw, id: () => string): TreeNode {
    if (raw.op !== undefined || Array.isArray(raw.rules)) {
        return {
            type: 'group',
            id: id(),
            op: raw.op === 'or' ? 'or' : 'and',
            not: raw.not === true,
            rules: Array.isArray(raw.rules)
                ? (raw.rules as Raw[]).map((r) => fromRaw(r, id))
                : [],
        };
    }
    let value = '';
    if (Array.isArray(raw.value)) value = raw.value.map(String).join(', ');
    else if (typeof raw.value === 'string' || typeof raw.value === 'number')
        value = String(raw.value);
    if (valueShape(String(raw.operator)) === 'date') value = value.slice(0, 10);
    return {
        type: 'rule',
        id: id(),
        field: String(raw.field ?? 'name'),
        operator: String(raw.operator ?? 'eq'),
        value,
        not: raw.not === true,
    };
}

/** The editor tree for a saved definition; the root is always a group. */
export function fromDefinition(def: unknown): GroupNode {
    let n = 0;
    const id = () => `t${++n}`;
    if (!def || typeof def !== 'object' || Object.keys(def).length === 0) {
        return {
            type: 'group',
            id: id(),
            op: 'and',
            not: false,
            rules: [newRule(id())],
        };
    }
    const node = fromRaw(def as Raw, id);
    if (node.type === 'group') return node;
    return { type: 'group', id: id(), op: 'and', not: false, rules: [node] };
}

function splitList(v: string): string[] {
    return v
        .split(',')
        .map((s) => s.trim())
        .filter(Boolean);
}

/**
 * The API definition for a tree, plus a map from each node's JSON path
 * ("$.rules[0]") to its editor id so API errors can be shown in place.
 */
export function toDefinition(root: GroupNode): {
    definition: Record<string, unknown>;
    paths: Map<string, string>;
} {
    const paths = new Map<string, string>();
    const walk = (n: TreeNode, path: string): Record<string, unknown> => {
        paths.set(path, n.id);
        if (n.type === 'group') {
            return {
                op: n.op,
                ...(n.not ? { not: true } : {}),
                rules: n.rules.map((r, i) => walk(r, `${path}.rules[${i}]`)),
            };
        }
        const out: Record<string, unknown> = {
            field: n.field,
            operator: n.operator,
        };
        if (n.not) out.not = true;
        switch (valueShape(n.operator)) {
            case 'none':
                break;
            case 'list':
                out.value = splitList(n.value);
                break;
            case 'days':
                out.value = Number(n.value);
                break;
            default:
                out.value = n.value.trim();
        }
        return out;
    };
    return { definition: walk(root, '$'), paths };
}

/**
 * Checks what the editor can check before asking the API; returns
 * problems by node id.
 */
export function localProblems(root: GroupNode): Map<string, string> {
    const out = new Map<string, string>();
    const walk = (n: TreeNode, depth: number) => {
        if (n.type === 'group') {
            if (n.rules.length === 0) {
                out.set(n.id, 'Add at least one rule to this group.');
            }
            n.rules.forEach((r) => walk(r, depth + 1));
            return;
        }
        const v = n.value.trim();
        switch (valueShape(n.operator)) {
            case 'none':
                return;
            case 'list': {
                const l = splitList(n.value);
                if (l.length === 0) out.set(n.id, 'Enter at least one value.');
                else if (l.length > MAX_VALUES)
                    out.set(n.id, `At most ${MAX_VALUES} values.`);
                else if (l.some((s) => s.length > MAX_TEXT))
                    out.set(n.id, `Values are up to ${MAX_TEXT} characters.`);
                return;
            }
            case 'days': {
                const d = Number(v);
                if (!Number.isInteger(d) || d < 1 || d > 3650)
                    out.set(n.id, 'Enter a whole number of days (1–3650).');
                return;
            }
            case 'date':
                if (!/^\d{4}-\d{2}-\d{2}$/.test(v))
                    out.set(n.id, 'Choose a date.');
                return;
            case 'group':
                if (!v) out.set(n.id, 'Choose a group.');
                return;
            default:
                if (!v) out.set(n.id, 'Enter a value.');
                else if (v.length > MAX_TEXT)
                    out.set(n.id, `Up to ${MAX_TEXT} characters.`);
        }
    };
    walk(root, 1);
    return out;
}

export function countRules(n: TreeNode): number {
    return n.type === 'rule'
        ? 1
        : n.rules.reduce((sum, r) => sum + countRules(r), 0);
}

/** Maps an API error ("$.rules[0].value: must be …") to a node id. */
export function locateError(
    message: string,
    paths: Map<string, string>,
): { id?: string; message: string } {
    const m = /^(\$[^\s:]*): (.*)$/.exec(message);
    if (!m) return { message };
    let path = m[1]!.replace(/\.(field|operator|value)$/, '');
    while (!paths.has(path) && /\.rules\[\d+\]$/.test(path)) {
        path = path.replace(/\.rules\[\d+\]$/, '');
    }
    return { id: paths.get(path), message: m[2]! };
}

/** Replaces the node with this id (or removes it when next is null). */
export function updateNode(
    root: GroupNode,
    id: string,
    next: (n: TreeNode) => TreeNode | null,
): GroupNode {
    const walk = (n: TreeNode): TreeNode | null => {
        if (n.id === id) return next(n);
        if (n.type === 'rule') return n;
        const rules = n.rules
            .map(walk)
            .filter((r): r is TreeNode => r !== null)
            // A nested group that lost its last rule goes away too.
            .filter((r) => r.type === 'rule' || r.rules.length > 0);
        return { ...n, rules };
    };
    const out = walk(root);
    return out && out.type === 'group' ? out : { ...root, rules: [] };
}
