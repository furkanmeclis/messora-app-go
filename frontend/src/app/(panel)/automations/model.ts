/**
 * Pure helpers for the automation screens: step and trigger shapes, the
 * conversion of single steps to and from the API, limits, plain-language
 * summaries and durations. Mirrors internal/modules/automation/program.go.
 * The step tree itself lives in graph.ts (the canvas).
 */
import type { MessageKey, Translate } from '@/i18n/translate';
import type {
    AutomationStep,
    AutomationTrigger,
    AutomationTriggerType,
    CampaignParams,
} from '@/lib/types';

/** A message key built at run time (the dictionary has every variant). */
export function mk(key: string): MessageKey {
    return key as MessageKey;
}

// ── Limits (program.go) ────────────────────────────────────────────────────

export const MAX_STEPS = 50;
export const MAX_DEPTH = 4;
export const MAX_DELAY_SECONDS = 30 * 24 * 3600;
export const MIN_MUTE_SECONDS = 60;
export const MAX_MUTE_SECONDS = 90 * 24 * 3600;
export const MAX_KEYWORDS = 50;

// ── Triggers ───────────────────────────────────────────────────────────────

export const TRIGGER_TYPES: AutomationTriggerType[] = [
    'message_received',
    'new_contact',
    'tag_added',
    'message_status',
    'conversation_closed',
    'subscription_request',
];

export function triggerLabel(type: string, t: Translate): string {
    return TRIGGER_TYPES.includes(type as AutomationTriggerType)
        ? t(mk(`automations.triggers.${type}.label`))
        : type;
}

export const MATCH_MODES = ['any', 'contains', 'exact', 'starts_with'] as const;

export const MESSAGE_STATUSES = ['delivered', 'read', 'failed'] as const;

/** Kinds of subscription_request ('' = any of them). */
export const SUBSCRIPTION_KINDS = ['stop', 'request', 'resume'] as const;

/** Trigger options kept by the editor (all types at once). */
export type TriggerForm = {
    match: string;
    keywords: string[];
    phone_number_id: string;
    tag: string;
    status: string;
    campaign_id: string;
    /** subscription_request: '' (any), stop, request or resume. */
    subscription: string;
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
        subscription: t?.subscription ?? '',
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
        case 'subscription_request':
            if (f.subscription)
                t.subscription =
                    f.subscription as AutomationTrigger['subscription'];
            break;
    }
    if (f.cooldown_seconds > 0) t.cooldown_seconds = f.cooldown_seconds;
    return t;
}

function quoteList(words: string[], t: Translate): string {
    const q = words.map((w) => `'${w}'`);
    const or = t('automations.summary.or');
    if (q.length <= 3) {
        return q.length <= 1
            ? (q[0] ?? '')
            : `${q.slice(0, -1).join(', ')} ${or} ${q.at(-1)}`;
    }
    return `${q.slice(0, 3).join(', ')} ${t('automations.summary.orMore', {
        count: q.length - 3,
    })}`;
}

/**
 * Plain-language trigger, e.g. "When a message contains 'price' or
 * 'fiyat'". Lookups name phone numbers and campaigns when known.
 */
export function describeTrigger(
    type: AutomationTriggerType,
    trig: AutomationTrigger | undefined,
    t: Translate,
    names: {
        phones?: Map<string, string>;
        campaigns?: Map<string, string>;
    } = {},
): string {
    const opts = trig ?? {};
    let s: string;
    const onNumber = opts.phone_number_id
        ? t('automations.summary.onNumber', {
              number:
                  names.phones?.get(opts.phone_number_id) ??
                  t('automations.summary.oneNumber'),
          })
        : '';
    switch (type) {
        case 'message_received': {
            const kw = opts.keywords ?? [];
            const match = opts.match ?? 'any';
            s =
                match !== 'any' && kw.length
                    ? t(mk(`automations.summary.message.${match}`), {
                          words: quoteList(kw, t),
                      })
                    : t('automations.summary.anyMessage');
            s += onNumber;
            break;
        }
        case 'new_contact':
            s = t('automations.summary.newContact') + onNumber;
            break;
        case 'tag_added':
            s = t('automations.summary.tagAdded', { tag: opts.tag ?? '' });
            break;
        case 'message_status': {
            const st = opts.status ?? 'delivered';
            s = t(mk(`automations.summary.status.${st}`));
            if (opts.campaign_id) {
                const c = names.campaigns?.get(opts.campaign_id);
                s += c
                    ? t('automations.summary.inCampaign', { campaign: c })
                    : t('automations.summary.inOneCampaign');
            }
            break;
        }
        case 'conversation_closed':
            s = t('automations.summary.conversationClosed');
            break;
        case 'subscription_request':
            s = t(
                mk(
                    `automations.summary.subscription.${opts.subscription ?? 'any'}`,
                ),
            );
            break;
        default:
            s = t('automations.summary.unknown', { type: String(type) });
    }
    if (opts.cooldown_seconds) {
        s += t('automations.summary.cooldown', {
            duration: formatDuration(opts.cooldown_seconds, t),
        });
    }
    return s;
}

// ── Durations ──────────────────────────────────────────────────────────────

export const DURATION_UNITS = [
    { value: 'seconds', seconds: 1 },
    { value: 'minutes', seconds: 60 },
    { value: 'hours', seconds: 3600 },
    { value: 'days', seconds: 86400 },
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

/** "1 hour", "90 minutes", "2 days" (in the user's language). */
export function formatDuration(seconds: number, t: Translate): string {
    const { amount, unit } = splitDuration(seconds);
    return t(mk(`automations.duration.${unit}`), { count: amount });
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

/** Colour family of a node on the canvas and in the palette. */
export type Category = 'trigger' | 'message' | 'data' | 'flow' | 'integration';

export const STEP_TYPES: { type: StepType; category: Category }[] = [
    { type: 'reply', category: 'message' },
    { type: 'send_template', category: 'message' },
    { type: 'add_tag', category: 'data' },
    { type: 'remove_tag', category: 'data' },
    { type: 'add_to_group', category: 'data' },
    { type: 'set_field', category: 'data' },
    { type: 'assign', category: 'data' },
    { type: 'close_conversation', category: 'data' },
    { type: 'condition', category: 'flow' },
    { type: 'delay', category: 'flow' },
    { type: 'mute', category: 'flow' },
    { type: 'http_request', category: 'integration' },
    { type: 'send_email', category: 'integration' },
];

export const CATEGORIES: Exclude<Category, 'trigger'>[] = [
    'message',
    'data',
    'flow',
    'integration',
];

const KNOWN = new Set<string>(STEP_TYPES.map((s) => s.type));

export function isKnownStep(type: string): type is StepType {
    return KNOWN.has(type);
}

export function stepCategory(type: string): Category {
    return STEP_TYPES.find((s) => s.type === type)?.category ?? 'integration';
}

export function stepLabel(type: string, t: Translate): string {
    if (type === 'branch') return t('automations.steps.condition.label');
    return isKnownStep(type) ? t(mk(`automations.steps.${type}.label`)) : type;
}

export const CONTACT_FIELDS = [
    'name',
    'email',
    'var1',
    'var2',
    'var3',
    'var4',
    'var5',
] as const;

export function contactFieldLabel(f: string, t: Translate): string {
    if (f === 'name') return t('automations.contactFields.name');
    if (f === 'email') return t('automations.contactFields.email');
    return f;
}

export const HTTP_METHODS = ['GET', 'POST', 'PUT', 'PATCH', 'DELETE'] as const;

export type TemplateVars = string[] | Record<string, string>;

export type HeaderRow = { key: string; name: string; value: string };

/**
 * One step as edited on the canvas: its id plus the API fields of its type.
 * Branches are edges of the canvas, not fields (see graph.ts).
 */
export type EditorStep = {
    id: string;
    type: string;
    text?: string;
    template?: {
        name: string;
        language: string;
        body?: TemplateVars;
        header?: TemplateVars;
        buttons?: string[];
        fallback?: Record<string, string>;
        header_media?: {
            type: 'image' | 'video' | 'document';
            id?: string;
            link?: string;
            filename?: string;
        };
    } & Pick<
        CampaignParams,
        | 'location'
        | 'coupon_code'
        | 'expiration_time_ms'
        | 'cards'
        | 'otp_code'
        | 'flow'
    >;
    tag?: string;
    group_id?: string;
    field?: string;
    value?: string;
    if?: { field: string; operator: string; value?: string };
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

/**
 * A new step id, the shape the server generates ("n" + 12 base-36
 * characters); unique enough that two editors never collide.
 */
export function newStepId(): string {
    const digits = '0123456789abcdefghijklmnopqrstuvwxyz';
    const bytes = new Uint8Array(12);
    crypto.getRandomValues(bytes);
    let id = 'n';
    for (const b of bytes) id += digits[b % digits.length];
    return id;
}

let keySeq = 0;
/** Key for a row added in the browser (e.g. an HTTP header). */
export function newKey(): string {
    keySeq += 1;
    return `k${keySeq}`;
}

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

/** One step as stored (its own fields; branches are read by graph.ts). */
export function stepFromApi(raw: AutomationStep, id: string): EditorStep {
    const s = raw as Record<string, unknown>;
    const type = String(s.type ?? '');
    if (!isKnownStep(type)) {
        const { then: _t, else: _e, ...rest } = s;
        void _t;
        void _e;
        return { id, type, raw: rest as AutomationStep };
    }
    const tpl = s.template as Record<string, unknown> | undefined;
    const cond = s.if as Record<string, unknown> | undefined;
    const headers = s.headers as Record<string, string> | undefined;
    return {
        id,
        type,
        text: str(s.text),
        template: tpl
            ? {
                  name: str(tpl.name) ?? '',
                  language: str(tpl.language) ?? '',
                  body: vars(tpl.body),
                  header: vars(tpl.header),
                  buttons: Array.isArray(tpl.buttons)
                      ? tpl.buttons.map((x) => String(x ?? ''))
                      : undefined,
                  fallback:
                      tpl.fallback && typeof tpl.fallback === 'object'
                          ? (tpl.fallback as Record<string, string>)
                          : undefined,
                  header_media:
                      tpl.header_media && typeof tpl.header_media === 'object'
                          ? (tpl.header_media as NonNullable<
                                EditorStep['template']
                            >['header_media'])
                          : undefined,
                  ...pickExtras(tpl),
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
        url: str(s.url),
        method: str(s.method) || undefined,
        headers: headers
            ? Object.entries(headers).map(([name, value], j) => ({
                  key: `${id}h${j}`,
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
}

function cleanVars(v: TemplateVars | undefined): TemplateVars | undefined {
    if (!v) return undefined;
    if (Array.isArray(v)) return v.length ? v : undefined;
    return Object.keys(v).length ? v : undefined;
}

export function operatorNeedsValue(op: string): boolean {
    return !['is_set', 'is_not_set', 'is_true', 'is_false'].includes(op);
}

/**
 * One editor step to the API shape, with only the fields of its type.
 * Branches (then / else) are added by graph.ts.
 */
export function stepToApi(s: EditorStep): AutomationStep {
    const id = s.id;
    if (s.raw) return { ...s.raw, id };
    const type = s.type;
    switch (type) {
        case 'reply':
            return { id, type, text: s.text ?? '' };
        case 'send_template': {
            const tpl = s.template ?? { name: '', language: '' };
            const out: Record<string, unknown> = {
                name: tpl.name,
                language: tpl.language,
            };
            const body = cleanVars(tpl.body);
            const header = cleanVars(tpl.header);
            if (body) out.body = body;
            if (header) out.header = header;
            if (tpl.buttons?.some((b) => b.trim())) out.buttons = tpl.buttons;
            if (tpl.fallback && Object.keys(tpl.fallback).length)
                out.fallback = tpl.fallback;
            if (tpl.header_media) out.header_media = tpl.header_media;
            for (const k of EXTRA_KEYS)
                if (tpl[k] !== undefined) out[k] = tpl[k];
            return { id, type, template: out };
        }
        case 'add_tag':
        case 'remove_tag':
            return { id, type, tag: (s.tag ?? '').trim() };
        case 'add_to_group':
            return s.group_id
                ? { id, type, group_id: s.group_id }
                : { id, type };
        case 'set_field':
            return { id, type, field: s.field ?? '', value: s.value ?? '' };
        case 'condition': {
            const c = s.if ?? { field: '', operator: '' };
            return {
                id,
                type,
                if: operatorNeedsValue(c.operator)
                    ? {
                          field: c.field,
                          operator: c.operator,
                          value: c.value ?? '',
                      }
                    : { field: c.field, operator: c.operator },
            };
        }
        case 'http_request': {
            const out: AutomationStep = {
                id,
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
                id,
                type,
                to: (s.to ?? '').trim(),
                subject: s.subject ?? '',
                body: s.body ?? '',
            };
        case 'delay':
        case 'mute':
            return { id, type, seconds: s.seconds ?? 0 };
        case 'assign':
            return s.assignee_id
                ? { id, type, assignee_id: s.assignee_id }
                : { id, type };
        default:
            return { id, type };
    }
}

/** A new step of the given type with sensible defaults. */
export function newStep(type: StepType, id = newStepId()): EditorStep {
    switch (type) {
        case 'reply':
            return { id, type, text: '' };
        case 'send_template':
            return { id, type, template: { name: '', language: '' } };
        case 'add_tag':
        case 'remove_tag':
            return { id, type, tag: '' };
        case 'add_to_group':
            return { id, type, group_id: '' };
        case 'set_field':
            return { id, type, field: 'name', value: '' };
        case 'condition':
            return {
                id,
                type,
                if: { field: 'message.body', operator: 'contains', value: '' },
            };
        case 'http_request':
            return {
                id,
                type,
                url: 'https://',
                method: 'POST',
                headers: [],
                body: '',
            };
        case 'send_email':
            return { id, type, to: '', subject: '', body: '' };
        case 'delay':
            return { id, type, seconds: 3600 };
        case 'mute':
            return { id, type, seconds: 86400 };
        case 'assign':
            return { id, type, assignee_id: '' };
        default:
            return { id, type };
    }
}

/** Copy of a step under a new id (for "Duplicate"). */
export function cloneStep(s: EditorStep): EditorStep {
    return {
        ...structuredClone(s),
        id: newStepId(),
        headers: s.headers?.map((h) => ({ ...h, key: newKey() })),
    };
}

/**
 * Problems the server would reject, checked in the browser so they show on
 * the canvas while editing. The server stays authoritative.
 */
export function stepProblems(
    s: EditorStep,
    t: Translate,
): { field: string; message: string }[] {
    const out: { field: string; message: string }[] = [];
    const add = (field: string, key: MessageKey) =>
        out.push({ field, message: t(key) });
    switch (s.type) {
        case 'reply':
            if (!s.text?.trim()) add('text', 'automations.problems.text');
            break;
        case 'send_template':
            if (!s.template?.name || !s.template.language) {
                add('template', 'automations.problems.template');
            }
            break;
        case 'add_tag':
        case 'remove_tag':
            if (!s.tag?.trim()) add('tag', 'automations.problems.tag');
            break;
        case 'add_to_group':
            if (!s.group_id) add('group_id', 'automations.problems.group');
            break;
        case 'http_request':
            if (!/^https:\/\/[^/\s]+/i.test((s.url ?? '').trim())) {
                add('url', 'automations.problems.url');
            }
            break;
        case 'send_email':
            if (
                !(s.to ?? '').includes('@') ||
                !s.subject?.trim() ||
                !s.body?.trim()
            ) {
                add('', 'automations.problems.email');
            }
            break;
        case 'delay': {
            const v = s.seconds ?? 0;
            if (!(v >= 1 && v <= MAX_DELAY_SECONDS)) {
                add('seconds', 'automations.problems.delay');
            }
            break;
        }
        case 'mute': {
            const v = s.seconds ?? 0;
            if (!(v >= MIN_MUTE_SECONDS && v <= MAX_MUTE_SECONDS)) {
                add('seconds', 'automations.problems.mute');
            }
            break;
        }
        case 'condition':
            if (
                s.if &&
                operatorNeedsValue(s.if.operator) &&
                !(s.if.value ?? '').trim()
            ) {
                add('if.value', 'automations.problems.conditionValue');
            }
            break;
    }
    return out;
}

/** One line describing what a step does, for its node on the canvas. */
export function summarizeStep(
    s: EditorStep,
    t: Translate,
    lookup: {
        groups?: Map<string, string>;
        users?: Map<string, string>;
    } = {},
): string {
    const q = (v: string | undefined) => (v ? `'${v}'` : '…');
    switch (s.type) {
        case 'reply':
            return s.text?.trim() || t('automations.node.empty');
        case 'send_template':
            return s.template?.name
                ? `${s.template.name} · ${s.template.language}`
                : t('automations.node.empty');
        case 'add_tag':
        case 'remove_tag':
            return s.tag?.trim()
                ? `#${s.tag.trim()}`
                : t('automations.node.empty');
        case 'add_to_group':
            return s.group_id
                ? (lookup.groups?.get(s.group_id) ??
                      t('automations.fields.unknownGroup'))
                : t('automations.node.empty');
        case 'set_field':
            return `${contactFieldLabel(s.field ?? '', t)} = ${s.value ? q(s.value) : t('automations.node.cleared')}`;
        case 'condition': {
            const c = s.if;
            if (!c) return t('automations.node.empty');
            const field = conditionFieldLabel(c.field, t);
            const op = operatorLabel(c.field, c.operator, t);
            return operatorNeedsValue(c.operator)
                ? `${field} ${op} ${q(c.value)}`
                : `${field} ${op}`;
        }
        case 'http_request':
            return `${s.method ?? 'POST'} ${s.url ?? ''}`.trim();
        case 'send_email':
            return s.to?.trim()
                ? t('automations.node.emailTo', { to: s.to.trim() })
                : t('automations.node.empty');
        case 'delay':
            return t('automations.node.waitFor', {
                duration: formatDuration(s.seconds ?? 0, t),
            });
        case 'mute':
            return t('automations.node.muteFor', {
                duration: formatDuration(s.seconds ?? 0, t),
            });
        case 'assign':
            return s.assignee_id
                ? (lookup.users?.get(s.assignee_id) ??
                      t('automations.fields.anotherUser'))
                : t('automations.fields.unassign');
        case 'close_conversation':
            return t('automations.steps.close_conversation.hint');
        default:
            return t('automations.node.unknownType');
    }
}

// ── API validation errors ──────────────────────────────────────────────────

export type StepError = { field: string; message: string };

/**
 * Finds the step an API error path points at, e.g. `steps[1].then[0].text`
 * (or the schema guard's `steps.1.text`), in the steps as sent. Returns the
 * step's id and the remaining field path ("" for the whole step).
 */
export function locateStep(
    steps: AutomationStep[],
    path: string,
): { id: string; field: string } | null {
    const p = path.replace(/\.(\d+)(?=\.|$)/g, '[$1]');
    if (!p.startsWith('steps[')) return null;
    let rest = p.slice('steps'.length);
    let list: AutomationStep[] | undefined = steps;
    let found: AutomationStep | undefined;
    for (;;) {
        const m = /^\[(\d+)\]/.exec(rest);
        if (!m) break;
        const next: AutomationStep | undefined = list?.[Number(m[1])];
        if (!next) break;
        found = next;
        rest = rest.slice(m[0].length);
        const b = /^\.(then|else)(?=\[)/.exec(rest);
        if (!b) break;
        list = (found as Record<string, unknown>)[b[1]!] as
            AutomationStep[] | undefined;
        rest = rest.slice(b[0].length);
    }
    if (!found?.id) return null;
    const field = rest.replace(/^\./, '');
    // "then" / "else" errors concern the condition's branches.
    return { id: found.id, field };
}

/** Splits API field errors into per-step errors and the rest. */
export function mapErrors(
    steps: AutomationStep[],
    fieldErrors: Record<string, string> | undefined,
): { steps: Record<string, StepError>; other: Record<string, string> } {
    const out = {
        steps: {} as Record<string, StepError>,
        other: {} as Record<string, string>,
    };
    for (const [path, message] of Object.entries(fieldErrors ?? {})) {
        const at = locateStep(steps, path);
        if (at) out.steps[at.id] = { field: at.field, message };
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

const TEXT_OPS = [
    'contains',
    'not_contains',
    'equals',
    'not_equals',
    'starts_with',
    'is_set',
    'is_not_set',
];
const BOOL_OPS = ['is_true', 'is_false'];
const TAG_OPS = ['equals', 'not_equals'];

export const CONDITION_FIELDS: { value: string; ops: string[] }[] = [
    { value: 'message.body', ops: TEXT_OPS },
    { value: 'message.type', ops: TEXT_OPS },
    { value: 'contact.name', ops: TEXT_OPS },
    { value: 'contact.email', ops: TEXT_OPS },
    { value: 'contact.phone', ops: TEXT_OPS },
    ...[1, 2, 3, 4, 5].map((n) => ({
        value: `contact.var${n}`,
        ops: TEXT_OPS,
    })),
    { value: 'contact.tag', ops: TAG_OPS },
    { value: 'contact.opted_out', ops: BOOL_OPS },
    { value: 'conversation.window_open', ops: BOOL_OPS },
    { value: 'event.tag', ops: TEXT_OPS },
    { value: 'event.status', ops: TEXT_OPS },
];

export function conditionFieldLabel(field: string, t: Translate): string {
    const m = /^contact\.var([1-5])$/.exec(field);
    if (m) return t('automations.conditionFields.var', { n: m[1]! });
    return CONDITION_FIELDS.some((f) => f.value === field)
        ? t(mk(`automations.conditionFields.${field.replace('.', '_')}`))
        : field;
}

export function operatorsFor(field: string): string[] {
    return CONDITION_FIELDS.find((f) => f.value === field)?.ops ?? TEXT_OPS;
}

export function operatorLabel(field: string, op: string, t: Translate): string {
    if (field === 'contact.tag') {
        return op === 'not_equals' || op === 'not_contains'
            ? t('automations.operators.not_has_tag')
            : t('automations.operators.has_tag');
    }
    return [...TEXT_OPS, ...BOOL_OPS].includes(op)
        ? t(mk(`automations.operators.${op}`))
        : op;
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

/** Template values beyond variables and header media (campaigns.Extras). */
const EXTRA_KEYS = [
    'location',
    'coupon_code',
    'expiration_time_ms',
    'cards',
    'otp_code',
    'flow',
] as const;

type Extras = Pick<
    NonNullable<EditorStep['template']>,
    (typeof EXTRA_KEYS)[number]
>;

function pickExtras(tpl: Record<string, unknown>): Extras {
    const out: Record<string, unknown> = {};
    for (const k of EXTRA_KEYS) if (tpl[k] !== undefined) out[k] = tpl[k];
    return out as Extras;
}
