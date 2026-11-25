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
    /** Copy code buttons: the sample code. */
    example?: string;
    /** OTP buttons: COPY_CODE, ONE_TAP or ZERO_TAP. */
    otp_type?: string;
    autofill_text?: string;
};

/** One card of a media card carousel. */
export type ParsedCard = {
    headerFormat?: string;
    body?: string;
    buttons: TemplateButton[];
};

export type ParsedTemplate = {
    headerFormat?: string;
    header?: string;
    body: string;
    footer?: string;
    buttons: TemplateButton[];
    /** Limited-time offer component. */
    lto?: { text: string; hasExpiration: boolean };
    cards: ParsedCard[];
    /** Authentication templates: Meta's security line and code expiry. */
    securityRecommendation?: boolean;
    codeExpirationMinutes?: number;
};

const VARIABLE = /\{\{\s*([A-Za-z0-9_]+)\s*\}\}/g;

function str(v: unknown): string | undefined {
    return typeof v === 'string' ? v : undefined;
}

function parseButtons(v: unknown): TemplateButton[] {
    if (!Array.isArray(v)) return [];
    const out: TemplateButton[] = [];
    for (const b of v) {
        if (!b || typeof b !== 'object') continue;
        const x = b as Record<string, unknown>;
        out.push({
            type: String(x.type ?? '').toUpperCase(),
            text: str(x.text) ?? '',
            url: str(x.url),
            phone_number: str(x.phone_number),
            example: str(x.example),
            otp_type: str(x.otp_type)?.toUpperCase(),
            autofill_text: str(x.autofill_text),
        });
    }
    return out;
}

export function parseComponents(components: unknown): ParsedTemplate {
    const out: ParsedTemplate = { body: '', buttons: [], cards: [] };
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
                if (c.add_security_recommendation === true)
                    out.securityRecommendation = true;
                break;
            case 'FOOTER':
                out.footer = str(c.text);
                if (typeof c.code_expiration_minutes === 'number')
                    out.codeExpirationMinutes = c.code_expiration_minutes;
                break;
            case 'BUTTONS':
                out.buttons = parseButtons(c.buttons);
                break;
            case 'LIMITED_TIME_OFFER': {
                const o = (c.limited_time_offer ?? {}) as Record<
                    string,
                    unknown
                >;
                out.lto = {
                    text: str(o.text) ?? '',
                    hasExpiration: o.has_expiration === true,
                };
                break;
            }
            case 'CAROUSEL':
                if (Array.isArray(c.cards)) {
                    for (const card of c.cards) {
                        const parsed = parseComponents(
                            (card as Record<string, unknown> | null)
                                ?.components,
                        );
                        out.cards.push({
                            headerFormat: parsed.headerFormat,
                            body: parsed.body || undefined,
                            buttons: parsed.buttons,
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

/** One carousel card's send-time needs. */
export type CardShape = {
    header: 'image' | 'video';
    bodyVars: string[];
    /** Button positions of URL buttons with a variable. */
    urlButtons: number[];
    /** Button positions of quick replies (their value is the payload). */
    quickReplies: number[];
};

/** Everything a template needs at send time (mirrors campaigns.ShapeOf). */
export type TemplateShape = {
    named: boolean;
    header: HeaderKind;
    headerVars: string[];
    bodyVars: string[];
    /** Button positions of URL buttons with a variable. */
    urlButtons: number[];
    /** Button position of the copy code button (at most one). */
    copyCode: number[];
    /** Button position of the Flow button (at most one). */
    flow: number[];
    /** Button position of an authentication template's OTP button. */
    otp: number[];
    /** A limited-time offer: sending needs the offer's end. */
    lto: boolean;
    cards: CardShape[];
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
    if (typeof v === 'string') return [v];
    return [];
}

function bodyExamples(
    c: Record<string, unknown>,
    vars: string[],
    prefix: string,
    into: Record<string, string>,
) {
    const ex = c.example as Record<string, unknown> | undefined;
    const namedEx = ex?.body_text_named_params;
    if (Array.isArray(namedEx)) {
        for (const p of namedEx) {
            const x = p as Record<string, unknown>;
            if (x?.param_name)
                into[`${prefix}${String(x.param_name)}`] = String(
                    x.example ?? '',
                );
        }
        return;
    }
    exampleValues(c, 'body_text').forEach((v, i) => {
        const k = vars[i];
        if (k) into[`${prefix}${k}`] = v;
    });
}

export function shapeOf(t: Template): TemplateShape {
    const named = t.parameter_format?.toUpperCase() === 'NAMED';
    // Meta stores an authentication template's OTP button as a URL button.
    const auth = t.category?.toUpperCase() === 'AUTHENTICATION';
    const order = (vars: string[]) =>
        named ? vars : [...vars].sort((a, b) => Number(a) - Number(b) || 0);
    const shape: TemplateShape = {
        named,
        header: 'none',
        headerVars: [],
        bodyVars: [],
        urlButtons: [],
        copyCode: [],
        flow: [],
        otp: [],
        lto: false,
        cards: [],
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
            bodyExamples(c, shape.bodyVars, 'body:', shape.examples);
        } else if (type === 'LIMITED_TIME_OFFER') {
            shape.lto = true;
        } else if (type === 'BUTTONS' && Array.isArray(c.buttons)) {
            c.buttons.forEach((b, i) => {
                const x = (b ?? {}) as Record<string, unknown>;
                const bt = String(x.type ?? '').toUpperCase();
                if (bt === 'OTP' || (auth && bt === 'URL')) {
                    shape.otp.push(i);
                } else if (bt === 'URL' && hasVariable(str(x.url))) {
                    shape.urlButtons.push(i);
                    const ex = Array.isArray(x.example)
                        ? x.example[0]
                        : undefined;
                    if (ex) shape.examples[`button:${i}`] = String(ex);
                } else if (bt === 'COPY_CODE') {
                    shape.copyCode.push(i);
                    if (typeof x.example === 'string')
                        shape.examples.coupon = x.example;
                } else if (bt === 'FLOW') {
                    shape.flow.push(i);
                }
            });
        } else if (type === 'CAROUSEL' && Array.isArray(c.cards)) {
            c.cards.forEach((card, k) => {
                const cs: CardShape = {
                    header: 'image',
                    bodyVars: [],
                    urlButtons: [],
                    quickReplies: [],
                };
                const comps = (card as Record<string, unknown> | null)
                    ?.components;
                if (Array.isArray(comps)) {
                    for (const cr of comps) {
                        const cc = (cr ?? {}) as Record<string, unknown>;
                        const ct = String(cc.type ?? '').toUpperCase();
                        if (ct === 'HEADER') {
                            cs.header =
                                String(cc.format ?? '').toLowerCase() ===
                                'video'
                                    ? 'video'
                                    : 'image';
                            const m = exampleValues(cc, 'header_handle')[0];
                            if (m) shape.examples[`card:${k}:media`] = m;
                        } else if (ct === 'BODY') {
                            cs.bodyVars = order(variablesOf(str(cc.text)));
                            bodyExamples(
                                cc,
                                cs.bodyVars,
                                `card:${k}:body:`,
                                shape.examples,
                            );
                        } else if (
                            ct === 'BUTTONS' &&
                            Array.isArray(cc.buttons)
                        ) {
                            cc.buttons.forEach((b, i) => {
                                const x = (b ?? {}) as Record<string, unknown>;
                                const bt = String(x.type ?? '').toUpperCase();
                                if (bt === 'QUICK_REPLY')
                                    cs.quickReplies.push(i);
                                if (bt === 'URL' && hasVariable(str(x.url))) {
                                    cs.urlButtons.push(i);
                                    const ex = Array.isArray(x.example)
                                        ? x.example[0]
                                        : undefined;
                                    if (ex)
                                        shape.examples[
                                            `card:${k}:button:${i}`
                                        ] = String(ex);
                                }
                            });
                        }
                    }
                }
                shape.cards.push(cs);
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

/** Media of a header or carousel card (campaign format). */
export type HeaderMediaValue = NonNullable<CampaignParams['header_media']>;

/** One carousel card's values. */
export type CardValues = {
    body: Record<string, string>;
    /** By button position: URL suffix, or a quick reply's payload. */
    buttons: Record<number, string>;
    media?: HeaderMediaValue;
    mediaPreview?: string;
};

export type LocationValue = {
    latitude: string;
    longitude: string;
    name: string;
    address: string;
};

/** Template params (campaign format) keyed for editing. */
export type ParamValues = {
    header: Record<string, string>;
    body: Record<string, string>;
    buttons: Record<number, string>;
    fallback: Record<string, string>;
    media?: CampaignParams['header_media'];
    /** Client-only: what to show for the media (object URL of an upload). */
    mediaPreview?: string;
    location?: LocationValue;
    /** Copy code button. */
    coupon: string;
    /** Limited-time offer end (Unix ms). */
    expirationMs?: number;
    cards: CardValues[];
    /** Authentication templates: the one-time code. */
    otp: string;
    flowToken: string;
    /** JSON object text for the Flow's first screen. */
    flowData: string;
};

export const emptyValues = (): ParamValues => ({
    header: {},
    body: {},
    buttons: {},
    fallback: {},
    coupon: '',
    cards: [],
    otp: '',
    flowToken: '',
    flowData: '',
});

/**
 * What a linked offer fills in (campaigns): its end, coupon and image, so
 * the editor does not ask for them.
 */
export type OfferFills = {
    expiration?: boolean;
    coupon?: boolean;
    media?: boolean;
};

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

function sparse(list: string[] | undefined): Record<number, string> {
    const out: Record<number, string> = {};
    (list ?? []).forEach((b, i) => {
        if (b) out[i] = b;
    });
    return out;
}

export function valuesFromParams(
    p: CampaignParams | null | undefined,
    shape: TemplateShape,
): ParamValues {
    const out = emptyValues();
    if (!p) return out;
    out.header = toRecord(p.header, shape.headerVars, shape.named);
    out.body = toRecord(p.body, shape.bodyVars, shape.named);
    out.buttons = sparse(p.buttons);
    out.fallback = { ...(p.fallback ?? {}) };
    out.media = p.header_media;
    if (p.location)
        out.location = {
            latitude: p.location.latitude,
            longitude: p.location.longitude,
            name: p.location.name ?? '',
            address: p.location.address ?? '',
        };
    out.coupon = p.coupon_code ?? '';
    out.expirationMs = p.expiration_time_ms;
    out.cards = (p.cards ?? []).map((c, k) => ({
        body: toRecord(c.body, shape.cards[k]?.bodyVars ?? [], shape.named),
        buttons: sparse(c.buttons),
        media: c.header_media,
    }));
    out.otp = p.otp_code ?? '';
    out.flowToken = p.flow?.flow_token ?? '';
    out.flowData = p.flow?.flow_action_data
        ? JSON.stringify(p.flow.flow_action_data)
        : '';
    return out;
}

/** The Flow data object, or undefined when the text is not a JSON object. */
export function flowData(text: string): Record<string, unknown> | undefined {
    if (!text.trim()) return undefined;
    try {
        const v: unknown = JSON.parse(text);
        return v && typeof v === 'object' && !Array.isArray(v)
            ? (v as Record<string, unknown>)
            : undefined;
    } catch {
        return undefined;
    }
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
    // Sparse by button position: buttons[i] is the template's button i.
    const positions = (at: number[], rec: Record<number, string>) => {
        if (at.length === 0) return undefined;
        const max = Math.max(...at);
        return Array.from({ length: max + 1 }, (_, i) =>
            at.includes(i) ? (rec[i] ?? '') : '',
        );
    };
    const out: CampaignParams = {};
    const used: string[] = [];
    if (shape.otp.length > 0) {
        // Meta writes the body and button; both take the code.
        if (v.otp.trim()) out.otp_code = v.otp.trim();
        used.push(v.otp);
    } else {
        const header = pick(shape.headerVars, v.header);
        const body = pick(shape.bodyVars, v.body);
        if (header) out.header = header;
        if (body) out.body = body;
        const buttons = positions(shape.urlButtons, v.buttons);
        if (buttons) out.buttons = buttons;
        used.push(
            ...Object.values(v.header),
            ...Object.values(v.body),
            ...Object.values(v.buttons),
        );
    }
    if (isMediaHeader(shape.header) && v.media) out.header_media = v.media;
    if (shape.header === 'location' && v.location) {
        const l = v.location;
        if (l.latitude.trim() || l.longitude.trim()) {
            out.location = {
                latitude: l.latitude.trim(),
                longitude: l.longitude.trim(),
                ...(l.name.trim() ? { name: l.name.trim() } : {}),
                ...(l.address.trim() ? { address: l.address.trim() } : {}),
            };
            used.push(l.name, l.address);
        }
    }
    if (shape.copyCode.length > 0 && v.coupon.trim()) {
        out.coupon_code = v.coupon.trim();
        used.push(v.coupon);
    }
    if (shape.lto && v.expirationMs) out.expiration_time_ms = v.expirationMs;
    if (shape.cards.length > 0) {
        out.cards = shape.cards.map((cs, k) => {
            const cv = v.cards[k] ?? { body: {}, buttons: {} };
            const card: NonNullable<CampaignParams['cards']>[number] = {};
            if (cv.media) card.header_media = cv.media;
            const body = pick(cs.bodyVars, cv.body);
            if (body) card.body = body;
            const buttons = positions(
                [...cs.urlButtons, ...cs.quickReplies],
                cv.buttons,
            );
            if (buttons?.some((b) => b.trim())) card.buttons = buttons;
            used.push(...Object.values(cv.body), ...Object.values(cv.buttons));
            return card;
        });
    }
    if (shape.flow.length > 0) {
        const data = flowData(v.flowData);
        if (v.flowToken.trim() || data)
            out.flow = {
                ...(v.flowToken.trim()
                    ? { flow_token: v.flowToken.trim() }
                    : {}),
                ...(data ? { flow_action_data: data } : {}),
            };
    }
    // Only fallbacks for placeholders that are still used.
    const inUse = new Set(used.flatMap(placeholdersIn));
    const fb: Record<string, string> = {};
    for (const [k, x] of Object.entries(v.fallback)) {
        if (inUse.has(k) && x.trim()) fb[k] = x;
    }
    if (Object.keys(fb).length > 0) out.fallback = fb;
    return out;
}

/** Problems that would stop sending (mirrors campaigns.CheckParams). */
export type ParamIssue =
    | { kind: 'media' }
    | { kind: 'location' }
    | { kind: 'cardMedia'; card: number }
    | { kind: 'flowData' }
    | { kind: 'value'; slot: string };

export function issuesOf(
    v: ParamValues,
    shape: TemplateShape,
    fills: OfferFills = {},
): ParamIssue[] {
    const out: ParamIssue[] = [];
    if (shape.header === 'location') {
        const l = v.location;
        const lat = Number(l?.latitude);
        const lng = Number(l?.longitude);
        if (
            !l?.latitude.trim() ||
            !l.longitude.trim() ||
            !Number.isFinite(lat) ||
            !Number.isFinite(lng) ||
            Math.abs(lat) > 90 ||
            Math.abs(lng) > 180
        )
            out.push({ kind: 'location' });
    }
    if (isMediaHeader(shape.header)) {
        const m = v.media;
        const offerImage = fills.media && shape.header === 'image';
        if (
            (!m || m.type !== shape.header || (!m.id && !m.link)) &&
            !offerImage
        )
            out.push({ kind: 'media' });
    }
    if (shape.otp.length > 0) {
        if (!v.otp.trim()) out.push({ kind: 'value', slot: 'otp' });
        return out;
    }
    for (const k of shape.headerVars)
        if (!v.header[k]?.trim())
            out.push({ kind: 'value', slot: `header:${k}` });
    for (const k of shape.bodyVars)
        if (!v.body[k]?.trim()) out.push({ kind: 'value', slot: `body:${k}` });
    for (const i of shape.urlButtons)
        if (!v.buttons[i]?.trim())
            out.push({ kind: 'value', slot: `button:${i}` });
    if (shape.copyCode.length > 0 && !v.coupon.trim() && !fills.coupon)
        out.push({ kind: 'value', slot: 'coupon' });
    if (shape.lto && !v.expirationMs && !fills.expiration)
        out.push({ kind: 'value', slot: 'expiration' });
    shape.cards.forEach((cs, k) => {
        const cv = v.cards[k];
        const m = cv?.media;
        if (!m || m.type !== cs.header || (!m.id && !m.link))
            out.push({ kind: 'cardMedia', card: k });
        for (const name of cs.bodyVars)
            if (!cv?.body[name]?.trim())
                out.push({ kind: 'value', slot: `card:${k}:body:${name}` });
        for (const i of cs.urlButtons)
            if (!cv?.buttons[i]?.trim())
                out.push({ kind: 'value', slot: `card:${k}:button:${i}` });
    });
    if (shape.flow.length > 0 && v.flowData.trim() && !flowData(v.flowData))
        out.push({ kind: 'flowData' });
    return out;
}

/** Counts down to ends (Unix ms) from now: "2 g 3 sa" style parts. */
export function remaining(
    endsMs: number,
    nowMs: number,
): { days: number; hours: number; minutes: number; ended: boolean } {
    const left = Math.max(0, endsMs - nowMs);
    const minutes = Math.floor(left / 60_000);
    return {
        days: Math.floor(minutes / 1440),
        hours: Math.floor((minutes % 1440) / 60),
        minutes: minutes % 60,
        ended: left === 0,
    };
}
