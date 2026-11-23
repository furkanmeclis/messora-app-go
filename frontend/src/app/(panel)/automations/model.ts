/**
 * Pure helpers for the automation screens: the editor's step tree, the
 * conversion to and from the API shape, limits, plain-language summaries
 * and durations. Mirrors internal/modules/automation/program.go.
 */
import type {
    AutomationStep,
    AutomationTrigger,
    AutomationTriggerType,
} from '@/lib/types';

// ── Limits (program.go) ────────────────────────────────────────────────────

export const MAX_STEPS = 50;
export const MAX_DEPTH = 4;
export const MAX_DELAY_SECONDS = 30 * 24 * 3600;
export const MIN_MUTE_SECONDS = 60;
export const MAX_MUTE_SECONDS = 90 * 24 * 3600;
export const MAX_KEYWORDS = 50;

// ── Triggers ───────────────────────────────────────────────────────────────

export const TRIGGER_TYPES: {
    value: AutomationTriggerType;
    label: string;
    description: string;
}[] = [
    {
        value: 'message_received',
        label: 'Message received',
        description: 'A customer message arrives.',
    },
    {
        value: 'new_contact',
        label: 'New contact',
        description: 'The first message ever arrives from a contact.',
    },
    {
        value: 'tag_added',
        label: 'Tag added',
        description: 'A person or integration adds a tag to a contact.',
    },
    {
        value: 'message_status',
        label: 'Message status',
        description: 'An outbound message is delivered, read or fails.',
    },
    {
        value: 'conversation_closed',
        label: 'Conversation closed',
        description: 'A person or integration closes a conversation.',
    },
];

export const MATCH_MODES = [
    { value: 'any', label: 'Any message' },
    { value: 'contains', label: 'Contains a keyword' },
    { value: 'exact', label: 'Is exactly a keyword' },
    { value: 'starts_with', label: 'Starts with a keyword' },
] as const;

export const MESSAGE_STATUSES = ['delivered', 'read', 'failed'] as const;

/** Trigger options kept by the editor (all types at once). */
export type TriggerForm = {
    match: string;
    keywords: string[];
    phone_number_id: string;
    tag: string;
    status: string;
    campaign_id: string;
    cooldown_seconds: number;
};

export function triggerFromApi(t: AutomationTrigger | undefined): TriggerForm {
    return {
        match: t?.match ?? 'any',
        keywords: t?.keywords ?? [],
        phone_number_id: t?.phone_number_id ?? '',
        tag: t?.tag ?? '',
        status: t?.status ?? 'delivered',
        campaign_id: t?.campaign_id ?? '',
        cooldown_seconds: t?.cooldown_seconds ?? 0,
    };
}

/** Only the options that apply to the trigger type. */
export function triggerToApi(
    type: AutomationTriggerType,
    f: TriggerForm,
): AutomationTrigger {
    const t: AutomationTrigger = {};
    switch (type) {
        case 'message_received':
            t.match = f.match as AutomationTrigger['match'];
            if (f.match !== 'any') t.keywords = f.keywords;
            if (f.phone_number_id) t.phone_number_id = f.phone_number_id;
            break;
        case 'new_contact':
            if (f.phone_number_id) t.phone_number_id = f.phone_number_id;
            break;
        case 'tag_added':
            t.tag = f.tag.trim();
            break;
        case 'message_status':
            t.status = f.status as AutomationTrigger['status'];
            if (f.campaign_id) t.campaign_id = f.campaign_id;
            break;
    }
    if (f.cooldown_seconds > 0) t.cooldown_seconds = f.cooldown_seconds;
    return t;
}

function quoteList(words: string[], joiner: string): string {
    const q = words.map((w) => `'${w}'`);
    if (q.length <= 3) {
        return q.length <= 1
            ? (q[0] ?? '')
            : `${q.slice(0, -1).join(', ')} ${joiner} ${q.at(-1)}`;
    }
    return `${q.slice(0, 3).join(', ')} ${joiner} ${q.length - 3} more`;
}

/**
 * Plain-language trigger, e.g. "When a message contains 'price' or
 * 'fiyat'". Lookups name phone numbers and campaigns when known.
 */
export function describeTrigger(
    type: AutomationTriggerType,
    t: AutomationTrigger | undefined,
    names: {
        phones?: Map<string, string>;
        campaigns?: Map<string, string>;
    } = {},
): string {
    const opts = t ?? {};
    let s: string;
    const onNumber = opts.phone_number_id
        ? ` on ${names.phones?.get(opts.phone_number_id) ?? 'one number'}`
        : '';
    switch (type) {
        case 'message_received': {
            const kw = opts.keywords ?? [];
            const verbs: Record<string, string> = {
                contains: 'contains',
                exact: 'is exactly',
                starts_with: 'starts with',
            };
            const verb = verbs[opts.match ?? 'any'];
            s =
                verb && kw.length
                    ? `When a message ${verb} ${quoteList(kw, 'or')}`
                    : 'When any message arrives';
            s += onNumber;
            break;
        }
        case 'new_contact':
            s = `When a new contact writes for the first time${onNumber}`;
            break;
        case 'tag_added':
            s = `When the tag '${opts.tag ?? ''}' is added to a contact`;
            break;
        case 'message_status': {
            const st = opts.status ?? 'delivered';
            s =
                st === 'failed'
                    ? 'When an outbound message fails'
                    : `When an outbound message is ${st}`;
            if (opts.campaign_id) {
                const c = names.campaigns?.get(opts.campaign_id);
                s += c ? ` in campaign '${c}'` : ' in one campaign';
            }
            break;
        }
        case 'conversation_closed':
            s = 'When a conversation is closed';
            break;
        default:
            s = `When ${String(type)} happens`;
    }
    if (opts.cooldown_seconds) {
        s += ` (at most once per contact every ${formatDuration(opts.cooldown_seconds)})`;
    }
    return s;
}

// ── Durations ──────────────────────────────────────────────────────────────

export const DURATION_UNITS = [
    { value: 'seconds', label: 'seconds', seconds: 1 },
    { value: 'minutes', label: 'minutes', seconds: 60 },
    { value: 'hours', label: 'hours', seconds: 3600 },
    { value: 'days', label: 'days', seconds: 86400 },
] as const;

export type DurationUnit = (typeof DURATION_UNITS)[number]['value'];

/** Largest unit that expresses the seconds exactly. */
export function splitDuration(seconds: number): {
    amount: number;
    unit: DurationUnit;
} {
    for (const u of [...DURATION_UNITS].reverse()) {
        if (seconds > 0 && seconds % u.seconds === 0) {
            return { amount: seconds / u.seconds, unit: u.value };
        }
    }
    return { amount: seconds, unit: 'seconds' };
}

export function unitSeconds(unit: DurationUnit): number {
    return DURATION_UNITS.find((u) => u.value === unit)?.seconds ?? 1;
}

/** "1 hour", "90 minutes", "2 days". */
export function formatDuration(seconds: number): string {
    const { amount, unit } = splitDuration(seconds);
    const singular = unit.slice(0, -1);
    return `${amount} ${amount === 1 ? singular : unit}`;
}

// ── Steps ──────────────────────────────────────────────────────────────────

export type StepType =
    | 'reply'
    | 'send_template'
    | 'add_tag'
    | 'remove_tag'
    | 'add_to_group'
    | 'set_field'
    | 'condition'
    | 'http_request'
    | 'send_email'
    | 'delay'
    | 'assign'
    | 'close_conversation'
    | 'mute';

export const STEP_TYPES: { type: StepType; label: string; hint: string }[] = [
    {
        type: 'reply',
        label: 'Reply',
        hint: 'Free-form message (only inside the 24-hour window).',
    },
    {
        type: 'send_template',
        label: 'Send template',
        hint: 'An approved template message.',
    },
    { type: 'add_tag', label: 'Add tag', hint: 'Tag the contact.' },
    { type: 'remove_tag', label: 'Remove tag', hint: 'Untag the contact.' },
    {
        type: 'add_to_group',
        label: 'Add to group',
        hint: 'Add the contact to a group.',
    },
    {
        type: 'set_field',
        label: 'Set field',
        hint: 'Change name, e-mail or a custom variable.',
    },
    {
        type: 'http_request',
        label: 'HTTP request',
        hint: 'Call an https:// endpoint (15 s timeout).',
    },
    {
        type: 'send_email',
        label: 'Send e-mail',
        hint: 'Needs SMTP to be configured.',
    },
    { type: 'delay', label: 'Wait', hint: 'Pause the run (1 s – 30 days).' },
    {
        type: 'assign',
        label: 'Assign conversation',
        hint: 'Assign or unassign the conversation.',
    },
    {
        type: 'close_conversation',
        label: 'Close conversation',
        hint: 'Close the conversation.',
    },
    {
        type: 'mute',
        label: 'Mute automations',
        hint: 'No automation starts for the contact (1 min – 90 days).',
    },
    {
        type: 'condition',
        label: 'Condition',
        hint: 'Branch into then / else steps.',
    },
];

export function stepLabel(type: string): string {
    return STEP_TYPES.find((s) => s.type === type)?.label ?? type;
}

export const CONTACT_FIELDS = [
    { value: 'name', label: 'Name' },
    { value: 'email', label: 'E-mail' },
    { value: 'var1', label: 'var1' },
    { value: 'var2', label: 'var2' },
    { value: 'var3', label: 'var3' },
    { value: 'var4', label: 'var4' },
    { value: 'var5', label: 'var5' },
] as const;

export const HTTP_METHODS = ['GET', 'POST', 'PUT', 'PATCH', 'DELETE'] as const;

export type TemplateVars = string[] | Record<string, string>;

export type HeaderRow = { key: string; name: string; value: string };

/** One step as edited: a stable key plus the API fields. */
export type EditorStep = {
    key: string;
    type: string;
    text?: string;
    template?: {
        name: string;
        language: string;
        body?: TemplateVars;
        header?: TemplateVars;
    };
    tag?: string;
    group_id?: string;
    field?: string;
    value?: string;
    if?: { field: string; operator: string; value?: string };
    then?: EditorStep[];
    else?: EditorStep[];
    url?: string;
    method?: string;
    headers?: HeaderRow[];
    body?: string;
    to?: string;
    subject?: string;
    seconds?: number;
    assignee_id?: string;
    /** A step type this editor does not know, kept as loaded. */
    raw?: AutomationStep;
};

let keySeq = 0;
/** Key for a step added in the browser (loaded steps use their path). */
export function newKey(): string {
    keySeq += 1;
    return `n${keySeq}`;
}

const KNOWN = new Set(STEP_TYPES.map((s) => s.type as string));

function str(v: unknown): string | undefined {
    return typeof v === 'string' ? v : undefined;
}

function vars(v: unknown): TemplateVars | undefined {
    if (Array.isArray(v)) return v.map((x) => String(x ?? ''));
    if (v && typeof v === 'object') {
        return Object.fromEntries(
            Object.entries(v).map(([k, x]) => [k, String(x ?? '')]),
        );
    }
    return undefined;
}

/** Steps as stored to editor steps; keys come from the path (stable SSR). */
export function stepsFromApi(
    steps: AutomationStep[] | undefined,
    prefix = 's',
): EditorStep[] {
    return (steps ?? []).map((raw, i) => {
        const s = raw as Record<string, unknown>;
        const key = `${prefix}${i}`;
        const type = String(s.type ?? '');
        if (!KNOWN.has(type)) return { key, type, raw };
        const tpl = s.template as Record<string, unknown> | undefined;
        const cond = s.if as Record<string, unknown> | undefined;
        const headers = s.headers as Record<string, string> | undefined;
        return {
            key,
            type,
            text: str(s.text),
            template: tpl
                ? {
                      name: str(tpl.name) ?? '',
                      language: str(tpl.language) ?? '',
                      body: vars(tpl.body),
                      header: vars(tpl.header),
                  }
                : undefined,
            tag: str(s.tag),
            group_id: str(s.group_id),
            field: str(s.field),
            value: str(s.value),
            if: cond
                ? {
                      field: str(cond.field) ?? '',
                      operator: str(cond.operator) ?? '',
                      value: str(cond.value) ?? '',
                  }
                : undefined,
            then:
                type === 'condition'
                    ? stepsFromApi(s.then as AutomationStep[], `${key}t`)
                    : undefined,
            else:
                type === 'condition'
                    ? stepsFromApi(s.else as AutomationStep[], `${key}e`)
                    : undefined,
            url: str(s.url),
            method: str(s.method) || undefined,
            headers: headers
                ? Object.entries(headers).map(([name, value], j) => ({
                      key: `${key}h${j}`,
                      name,
                      value: String(value),
                  }))
                : undefined,
            body: str(s.body),
            to: str(s.to),
            subject: str(s.subject),
            seconds: typeof s.seconds === 'number' ? s.seconds : undefined,
            assignee_id: str(s.assignee_id),
        };
    });
}

function cleanVars(v: TemplateVars | undefined): TemplateVars | undefined {
    if (!v) return undefined;
    if (Array.isArray(v)) return v.length ? v : undefined;
    return Object.keys(v).length ? v : undefined;
}

export function operatorNeedsValue(op: string): boolean {
    return !['is_set', 'is_not_set', 'is_true', 'is_false'].includes(op);
}

/** Editor steps to the API shape, with only the fields of each type. */
export function stepsToApi(steps: EditorStep[]): AutomationStep[] {
    return steps.map((s): AutomationStep => {
        if (s.raw) return s.raw;
        const type = s.type;
        switch (type) {
            case 'reply':
                return { type, text: s.text ?? '' };
            case 'send_template': {
                const t = s.template ?? { name: '', language: '' };
                const out: Record<string, unknown> = {
                    name: t.name,
                    language: t.language,
                };
                const body = cleanVars(t.body);
                const header = cleanVars(t.header);
                if (body) out.body = body;
                if (header) out.header = header;
                return { type, template: out };
            }
            case 'add_tag':
            case 'remove_tag':
                return { type, tag: (s.tag ?? '').trim() };
            case 'add_to_group':
                return s.group_id ? { type, group_id: s.group_id } : { type };
            case 'set_field':
                return { type, field: s.field ?? '', value: s.value ?? '' };
            case 'condition': {
                const c = s.if ?? { field: '', operator: '' };
                const out: AutomationStep = {
                    type,
                    if: operatorNeedsValue(c.operator)
                        ? {
                              field: c.field,
                              operator: c.operator,
                              value: c.value ?? '',
                          }
                        : { field: c.field, operator: c.operator },
                };
                if (s.then?.length) out.then = stepsToApi(s.then);
                if (s.else?.length) out.else = stepsToApi(s.else);
                return out;
            }
            case 'http_request': {
                const out: AutomationStep = {
                    type,
                    url: (s.url ?? '').trim(),
                    method: s.method ?? 'POST',
                };
                const headers = (s.headers ?? []).filter((h) => h.name.trim());
                if (headers.length) {
                    out.headers = Object.fromEntries(
                        headers.map((h) => [h.name.trim(), h.value]),
                    );
                }
                if (s.body) out.body = s.body;
                return out;
            }
            case 'send_email':
                return {
                    type,
                    to: (s.to ?? '').trim(),
                    subject: s.subject ?? '',
                    body: s.body ?? '',
                };
            case 'delay':
            case 'mute':
                return { type, seconds: s.seconds ?? 0 };
            case 'assign':
                return s.assignee_id
                    ? { type, assignee_id: s.assignee_id }
                    : { type };
            default:
                return { type };
        }
    });
}

/** A new step of the given type with sensible defaults. */
export function newStep(type: StepType): EditorStep {
    const key = newKey();
    switch (type) {
        case 'reply':
            return { key, type, text: '' };
        case 'send_template':
            return { key, type, template: { name: '', language: '' } };
        case 'add_tag':
        case 'remove_tag':
            return { key, type, tag: '' };
        case 'add_to_group':
            return { key, type, group_id: '' };
        case 'set_field':
            return { key, type, field: 'name', value: '' };
        case 'condition':
            return {
                key,
                type,
                if: { field: 'message.body', operator: 'contains', value: '' },
                then: [],
                else: [],
            };
        case 'http_request':
            return {
                key,
                type,
                url: 'https://',
                method: 'POST',
                headers: [],
                body: '',
            };
        case 'send_email':
            return { key, type, to: '', subject: '', body: '' };
        case 'delay':
            return { key, type, seconds: 3600 };
        case 'mute':
            return { key, type, seconds: 86400 };
        case 'assign':
            return { key, type, assignee_id: '' };
        default:
            return { key, type };
    }
}

/** Deep copy with fresh keys (for "Duplicate"). */
export function cloneStep(s: EditorStep): EditorStep {
    return {
        ...s,
        key: newKey(),
        template: s.template ? structuredClone(s.template) : undefined,
        if: s.if ? { ...s.if } : undefined,
        headers: s.headers?.map((h) => ({ ...h, key: newKey() })),
        then: s.then?.map(cloneStep),
        else: s.else?.map(cloneStep),
    };
}

/** Steps counted like the API does (conditions and nested steps too). */
export function countSteps(steps: EditorStep[] | undefined): number {
    return (steps ?? []).reduce(
        (n, s) => n + 1 + countSteps(s.then) + countSteps(s.else),
        0,
    );
}

// ── Tree editing (immutable) ───────────────────────────────────────────────

/** "root" or "<condition key>:then" / "<condition key>:else". */
export type ListId = string;

export function editList(
    steps: EditorStep[],
    list: ListId,
    fn: (l: EditorStep[]) => EditorStep[],
): EditorStep[] {
    if (list === 'root') return fn(steps);
    const [owner, branch] = list.split(':') as [string, 'then' | 'else'];
    return steps.map((s) => {
        if (s.key === owner) return { ...s, [branch]: fn(s[branch] ?? []) };
        if (s.type !== 'condition') return s;
        return {
            ...s,
            then: s.then && editList(s.then, list, fn),
            else: s.else && editList(s.else, list, fn),
        };
    });
}

export function editStep(
    steps: EditorStep[],
    key: string,
    fn: (s: EditorStep) => EditorStep,
): EditorStep[] {
    return steps.map((s) => {
        if (s.key === key) return fn(s);
        if (s.type !== 'condition') return s;
        return {
            ...s,
            then: s.then && editStep(s.then, key, fn),
            else: s.else && editStep(s.else, key, fn),
        };
    });
}

// ── API validation errors ──────────────────────────────────────────────────

export type StepError = { field: string; message: string };

/**
 * Finds the step an API error path points at, e.g. `steps[1].then[0].text`
 * (or the schema guard's `steps.1.text`). Returns the step's key and the
 * remaining field path ("" for the whole step).
 */
export function locateStep(
    steps: EditorStep[],
    path: string,
): { key: string; field: string } | null {
    const p = path.replace(/\.(\d+)(?=\.|$)/g, '[$1]');
    if (!p.startsWith('steps[')) return null;
    let rest = p.slice('steps'.length);
    let list: EditorStep[] | undefined = steps;
    let found: EditorStep | undefined;
    for (;;) {
        const m = /^\[(\d+)\]/.exec(rest);
        if (!m) break;
        const next: EditorStep | undefined = list?.[Number(m[1])];
        if (!next) break;
        found = next;
        rest = rest.slice(m[0].length);
        const b = /^\.(then|else)(?=\[)/.exec(rest);
        if (!b) break;
        list = found[b[1] as 'then' | 'else'];
        rest = rest.slice(b[0].length);
    }
    if (!found) return null;
    return { key: found.key, field: rest.replace(/^\./, '') };
}

/** Splits API field errors into per-step errors and the rest. */
export function mapErrors(
    steps: EditorStep[],
    fieldErrors: Record<string, string> | undefined,
): { steps: Record<string, StepError>; other: Record<string, string> } {
    const out = {
        steps: {} as Record<string, StepError>,
        other: {} as Record<string, string>,
    };
    for (const [path, message] of Object.entries(fieldErrors ?? {})) {
        const at = locateStep(steps, path);
        if (at) out.steps[at.key] = { field: at.field, message };
        else out.other[path] = message;
    }
    return out;
}

// ── Placeholders and conditions ────────────────────────────────────────────

export const PLACEHOLDERS = [
    '{{name}}',
    '{{first_name}}',
    '{{phone}}',
    '{{email}}',
    '{{var1}}',
    '{{var2}}',
    '{{var3}}',
    '{{var4}}',
    '{{var5}}',
    '{{attr:key}}',
    '{{message}}',
] as const;

type Op = { value: string; label: string };
const TEXT_OPS: Op[] = [
    { value: 'contains', label: 'contains' },
    { value: 'not_contains', label: 'does not contain' },
    { value: 'equals', label: 'equals' },
    { value: 'not_equals', label: 'does not equal' },
    { value: 'starts_with', label: 'starts with' },
    { value: 'is_set', label: 'is set' },
    { value: 'is_not_set', label: 'is not set' },
];
const BOOL_OPS: Op[] = [
    { value: 'is_true', label: 'is true' },
    { value: 'is_false', label: 'is false' },
];
const TAG_OPS: Op[] = [
    { value: 'equals', label: 'has the tag' },
    { value: 'not_equals', label: 'does not have the tag' },
];

export const CONDITION_FIELDS: { value: string; label: string; ops: Op[] }[] = [
    { value: 'message.body', label: 'Message text', ops: TEXT_OPS },
    { value: 'message.type', label: 'Message type', ops: TEXT_OPS },
    { value: 'contact.name', label: 'Contact name', ops: TEXT_OPS },
    { value: 'contact.email', label: 'Contact e-mail', ops: TEXT_OPS },
    { value: 'contact.phone', label: 'Contact phone', ops: TEXT_OPS },
    ...[1, 2, 3, 4, 5].map((n) => ({
        value: `contact.var${n}`,
        label: `Contact var${n}`,
        ops: TEXT_OPS,
    })),
    { value: 'contact.tag', label: 'Contact tags', ops: TAG_OPS },
    {
        value: 'contact.opted_out',
        label: 'Contact opted out',
        ops: BOOL_OPS,
    },
    {
        value: 'conversation.window_open',
        label: '24-hour window open',
        ops: BOOL_OPS,
    },
    { value: 'event.tag', label: 'Added tag (trigger)', ops: TEXT_OPS },
    {
        value: 'event.status',
        label: 'Message status (trigger)',
        ops: TEXT_OPS,
    },
];

export function operatorsFor(field: string): Op[] {
    return CONDITION_FIELDS.find((f) => f.value === field)?.ops ?? TEXT_OPS;
}

// ── Templates ──────────────────────────────────────────────────────────────

/**
 * Variables a template expects in its body and text header: positional
 * ({{1}}, {{2}} → a list) or named ({{order_id}} → an object).
 */
export function templateVariables(components: Record<string, unknown>[]): {
    body: { named: boolean; names: string[] };
    header: { named: boolean; names: string[] };
} {
    const scan = (type: string) => {
        const c = components.find(
            (x) => String(x.type ?? '').toUpperCase() === type,
        );
        const text = typeof c?.text === 'string' ? c.text : '';
        const names = [
            ...new Set(
                [...text.matchAll(/\{\{\s*([A-Za-z0-9_]+)\s*\}\}/g)].map(
                    (m) => m[1]!,
                ),
            ),
        ];
        const named = names.some((n) => !/^\d+$/.test(n));
        if (!named) names.sort((a, b) => Number(a) - Number(b));
        return { named, names };
    };
    return { body: scan('BODY'), header: scan('HEADER') };
}
