import type { CampaignParams, Template } from './types';

/**
 * Helpers for Meta's template `components` JSON, shared by the templates
 * page (preview, create form) and the campaign wizard (variable mapping).
 * Templates come from Meta, so every field is treated as untrusted input.
 */

export type TemplateButton = {
    type: string;
    text: string;
    url?: string;
    phone_number?: string;
};

export type ParsedTemplate = {
    headerFormat?: string;
    header?: string;
    body: string;
    footer?: string;
    buttons: TemplateButton[];
};

const VARIABLE = /\{\{\s*([A-Za-z0-9_]+)\s*\}\}/g;

function str(v: unknown): string | undefined {
    return typeof v === 'string' ? v : undefined;
}

export function parseComponents(components: unknown): ParsedTemplate {
    const out: ParsedTemplate = { body: '', buttons: [] };
    if (!Array.isArray(components)) return out;
    for (const raw of components) {
        if (!raw || typeof raw !== 'object') continue;
        const c = raw as Record<string, unknown>;
        switch (String(c.type ?? '').toUpperCase()) {
            case 'HEADER':
                out.headerFormat = String(c.format ?? 'TEXT').toUpperCase();
                out.header = str(c.text);
                break;
            case 'BODY':
                out.body = str(c.text) ?? '';
                break;
            case 'FOOTER':
                out.footer = str(c.text);
                break;
            case 'BUTTONS':
                if (Array.isArray(c.buttons)) {
                    for (const b of c.buttons) {
                        if (!b || typeof b !== 'object') continue;
                        const x = b as Record<string, unknown>;
                        out.buttons.push({
                            type: String(x.type ?? '').toUpperCase(),
                            text: str(x.text) ?? '',
                            url: str(x.url),
                            phone_number: str(x.phone_number),
                        });
                    }
                }
                break;
        }
    }
    return out;
}

/** Variable names in order of first appearance ("1", "2" or "first_name"). */
export function variablesOf(text: string | undefined): string[] {
    if (!text) return [];
    const seen: string[] = [];
    for (const m of text.matchAll(VARIABLE)) {
        const name = m[1]!;
        if (!seen.includes(name)) seen.push(name);
    }
    return seen;
}

/** What a campaign has to fill in for a template. */
export type TemplateSlots = {
    named: boolean;
    header: string[];
    body: string[];
    /** Indexes (button positions) of URL buttons with a variable. */
    urlButtons: number[];
    /** A media header needs a file at send time, which campaigns lack. */
    mediaHeader: boolean;
};

export function slotsOf(t: Template): TemplateSlots {
    const p = parseComponents(t.components);
    const named = t.parameter_format?.toUpperCase() === 'NAMED';
    const order = (vars: string[]) =>
        named ? vars : [...vars].sort((a, b) => Number(a) - Number(b) || 0);
    return {
        named,
        header:
            p.headerFormat === 'TEXT' || p.headerFormat === undefined
                ? order(variablesOf(p.header))
                : [],
        body: order(variablesOf(p.body)),
        urlButtons: p.buttons
            .map((b, i) => (b.type === 'URL' && hasVariable(b.url) ? i : -1))
            .filter((i) => i >= 0),
        mediaHeader:
            p.headerFormat !== undefined &&
            p.headerFormat !== 'TEXT' &&
            p.headerFormat !== 'NONE',
    };
}

function hasVariable(s: string | undefined): boolean {
    return !!s && /\{\{\s*[A-Za-z0-9_]+\s*\}\}/.test(s);
}

/** Replaces each {{var}} in text with values[var] when given. */
export function fillVariables(
    text: string,
    values: Record<string, string>,
): string {
    return text.replace(VARIABLE, (m, name: string) => {
        const v = values[name];
        return v ? v : m;
    });
}

/** Templates are sent by name and language; this key identifies one. */
export function templateKey(t: { name: string; language: string }): string {
    return `${t.name}|${t.language}`;
}

export const TEMPLATE_STATUSES = [
    'APPROVED',
    'PENDING',
    'REJECTED',
    'PAUSED',
    'DISABLED',
] as const;

// ── Shape, values and sample rendering ────────────────────────────────────

export type HeaderKind =
    'none' | 'text' | 'image' | 'video' | 'document' | 'location';

/** Everything a template needs at send time (mirrors campaigns.ShapeOf). */
export type TemplateShape = {
    named: boolean;
    header: HeaderKind;
    headerVars: string[];
    bodyVars: string[];
    /** Button positions of URL buttons with a variable. */
    urlButtons: number[];
    /** Meta's examples, shown as hints. */
    examples: Record<string, string>;
    /** Example media of a media header (Meta-hosted, may expire). */
    exampleMedia?: string;
};

function exampleValues(c: Record<string, unknown>, key: string): string[] {
    const ex = c.example as Record<string, unknown> | undefined;
    const v = ex?.[key];
    if (Array.isArray(v)) {
        const first = v[0];
        if (Array.isArray(first)) return first.map(String);
        return v.map(String);
    }
    return [];
}

export function shapeOf(t: Template): TemplateShape {
    const named = t.parameter_format?.toUpperCase() === 'NAMED';
    const order = (vars: string[]) =>
        named ? vars : [...vars].sort((a, b) => Number(a) - Number(b) || 0);
    const shape: TemplateShape = {
        named,
        header: 'none',
        headerVars: [],
        bodyVars: [],
        urlButtons: [],
        examples: {},
    };
    if (!Array.isArray(t.components)) return shape;
    for (const raw of t.components) {
        if (!raw || typeof raw !== 'object') continue;
        const c = raw as Record<string, unknown>;
        const type = String(c.type ?? '').toUpperCase();
        if (type === 'HEADER') {
            const f = String(c.format ?? 'TEXT').toLowerCase();
            shape.header = (
                ['text', 'image', 'video', 'document', 'location'].includes(f)
                    ? f
                    : 'text'
            ) as HeaderKind;
            if (shape.header === 'text') {
                shape.headerVars = order(variablesOf(str(c.text)));
                exampleValues(c, 'header_text').forEach((v, i) => {
                    const k = shape.headerVars[i];
                    if (k) shape.examples[`header:${k}`] = v;
                });
            } else {
                shape.exampleMedia = exampleValues(c, 'header_handle')[0];
            }
        } else if (type === 'BODY') {
            shape.bodyVars = order(variablesOf(str(c.text)));
            const ex = c.example as Record<string, unknown> | undefined;
            const namedEx = ex?.body_text_named_params;
            if (Array.isArray(namedEx)) {
                for (const p of namedEx) {
                    const x = p as Record<string, unknown>;
                    if (x?.param_name)
                        shape.examples[`body:${String(x.param_name)}`] = String(
                            x.example ?? '',
                        );
                }
            } else {
                exampleValues(c, 'body_text').forEach((v, i) => {
                    const k = shape.bodyVars[i];
                    if (k) shape.examples[`body:${k}`] = v;
                });
            }
        } else if (type === 'BUTTONS' && Array.isArray(c.buttons)) {
            c.buttons.forEach((b, i) => {
                const x = (b ?? {}) as Record<string, unknown>;
                if (
                    String(x.type ?? '').toUpperCase() === 'URL' &&
                    hasVariable(str(x.url))
                ) {
                    shape.urlButtons.push(i);
                    const ex = Array.isArray(x.example)
                        ? x.example[0]
                        : undefined;
                    if (ex) shape.examples[`button:${i}`] = String(ex);
                }
            });
        }
    }
    return shape;
}

export const MEDIA_HEADERS = ['image', 'video', 'document'] as const;
export type MediaHeaderKind = (typeof MEDIA_HEADERS)[number];

export function isMediaHeader(k: HeaderKind): k is MediaHeaderKind {
    return (MEDIA_HEADERS as readonly string[]).includes(k);
}

/** Contact placeholders the server resolves (campaigns/render.go). */
export const CONTACT_FIELDS = [
    'name',
    'first_name',
    'phone',
    'email',
    'var1',
    'var2',
    'var3',
    'var4',
    'var5',
] as const;

const FIELD_TOKEN = /^\{\{\s*([a-z0-9_]+(?::[A-Za-z0-9_ .-]+)?)\s*\}\}$/;

/** The contact field a value maps to, when it is exactly one placeholder. */
export function fieldOf(value: string | undefined): string | undefined {
    return value ? FIELD_TOKEN.exec(value.trim())?.[1] : undefined;
}

export function fieldToken(field: string): string {
    return `{{${field}}}`;
}

const PLACEHOLDER = /\{\{\s*([a-z0-9_]+(?::[A-Za-z0-9_ .-]+)?)\s*\}\}/g;

/** Placeholders used in a value ("first_name", "attr:city"). */
export function placeholdersIn(value: string | undefined): string[] {
    if (!value) return [];
    return [...new Set([...value.matchAll(PLACEHOLDER)].map((m) => m[1]!))];
}

/** What a sample contact looks like for previews. */
export type SampleContact = {
    name?: string;
    phone?: string;
    email?: string | null;
    var1?: string | null;
    var2?: string | null;
    var3?: string | null;
    var4?: string | null;
    var5?: string | null;
    attributes?: Record<string, unknown>;
};

export function sampleValue(
    key: string,
    c: SampleContact | undefined,
): string | undefined {
    if (!c) return undefined;
    if (key.startsWith('attr:')) {
        const v = c.attributes?.[key.slice(5)];
        return v == null || v === '' ? undefined : String(v);
    }
    if (key === 'first_name')
        return c.name?.trim().split(/\s+/)[0] || undefined;
    const v = (c as Record<string, unknown>)[key];
    return v == null || v === '' ? undefined : String(v);
}

/** One rendered piece: literal text, or a placeholder with its value. */
export type Piece =
    { text: string } | { field: string; value?: string; fallback?: boolean };

/** Resolves the placeholders of a value against a sample contact. */
export function resolvePieces(
    value: string,
    contact: SampleContact | undefined,
    fallback: Record<string, string> | undefined,
): Piece[] {
    const out: Piece[] = [];
    let last = 0;
    for (const m of value.matchAll(PLACEHOLDER)) {
        if (m.index! > last) out.push({ text: value.slice(last, m.index) });
        const field = m[1]!;
        const v = sampleValue(field, contact);
        if (v) out.push({ field, value: v });
        else if (fallback?.[field])
            out.push({ field, value: fallback[field], fallback: true });
        else out.push({ field });
        last = m.index! + m[0].length;
    }
    if (last < value.length) out.push({ text: value.slice(last) });
    return out;
}

/** Template params (campaign format) keyed for editing. */
export type ParamValues = {
    header: Record<string, string>;
    body: Record<string, string>;
    buttons: Record<number, string>;
    fallback: Record<string, string>;
    media?: CampaignParams['header_media'];
    /** Client-only: what to show for the media (object URL of an upload). */
    mediaPreview?: string;
};

export const emptyValues = (): ParamValues => ({
    header: {},
    body: {},
    buttons: {},
    fallback: {},
});

function toRecord(
    v: string[] | Record<string, string> | undefined,
    vars: string[],
    named: boolean,
): Record<string, string> {
    if (!v) return {};
    if (Array.isArray(v)) {
        const out: Record<string, string> = {};
        v.forEach((x, i) => {
            out[named ? (vars[i] ?? String(i + 1)) : String(i + 1)] = x;
        });
        return out;
    }
    return { ...v };
}

export function valuesFromParams(
    p: CampaignParams | null | undefined,
    shape: TemplateShape,
): ParamValues {
    const out = emptyValues();
    if (!p) return out;
    out.header = toRecord(p.header, shape.headerVars, shape.named);
    out.body = toRecord(p.body, shape.bodyVars, shape.named);
    (p.buttons ?? []).forEach((b, i) => {
        if (b) out.buttons[i] = b;
    });
    out.fallback = { ...(p.fallback ?? {}) };
    out.media = p.header_media;
    return out;
}

/** Builds the API params for a template from the edited values. */
export function paramsFromValues(
    v: ParamValues,
    shape: TemplateShape,
): CampaignParams {
    const pick = (vars: string[], rec: Record<string, string>) => {
        if (vars.length === 0) return undefined;
        if (shape.named) {
            const o: Record<string, string> = {};
            for (const k of vars) o[k] = rec[k] ?? '';
            return o;
        }
        const max = Math.max(...vars.map(Number));
        return Array.from({ length: max }, (_, i) => rec[String(i + 1)] ?? '');
    };
    const out: CampaignParams = {};
    const header = pick(shape.headerVars, v.header);
    const body = pick(shape.bodyVars, v.body);
    if (header) out.header = header;
    if (body) out.body = body;
    if (shape.urlButtons.length > 0) {
        // Sparse by button position: buttons[i] is the template's button i.
        const max = Math.max(...shape.urlButtons);
        out.buttons = Array.from({ length: max + 1 }, (_, i) =>
            shape.urlButtons.includes(i) ? (v.buttons[i] ?? '') : '',
        );
    }
    if (isMediaHeader(shape.header) && v.media) out.header_media = v.media;
    // Only fallbacks for placeholders that are still used.
    const used = new Set(
        [
            ...Object.values(v.header),
            ...Object.values(v.body),
            ...Object.values(v.buttons),
        ].flatMap(placeholdersIn),
    );
    const fb: Record<string, string> = {};
    for (const [k, x] of Object.entries(v.fallback)) {
        if (used.has(k) && x.trim()) fb[k] = x;
    }
    if (Object.keys(fb).length > 0) out.fallback = fb;
    return out;
}

/** Problems that would stop sending (mirrors campaigns.CheckParams). */
export type ParamIssue =
    { kind: 'media' } | { kind: 'location' } | { kind: 'value'; slot: string };

export function issuesOf(v: ParamValues, shape: TemplateShape): ParamIssue[] {
    const out: ParamIssue[] = [];
    if (shape.header === 'location') out.push({ kind: 'location' });
    if (isMediaHeader(shape.header)) {
        const m = v.media;
        if (!m || m.type !== shape.header || (!m.id && !m.link))
            out.push({ kind: 'media' });
    }
    for (const k of shape.headerVars)
        if (!v.header[k]?.trim())
            out.push({ kind: 'value', slot: `header:${k}` });
    for (const k of shape.bodyVars)
        if (!v.body[k]?.trim()) out.push({ kind: 'value', slot: `body:${k}` });
    for (const i of shape.urlButtons)
        if (!v.buttons[i]?.trim())
            out.push({ kind: 'value', slot: `button:${i}` });
    return out;
}
