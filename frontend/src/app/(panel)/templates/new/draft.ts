import type { Translate } from '@/i18n/translate';
import { variablesOf } from '../../../../lib/templates.ts';

/**
 * The template builder's draft and the Meta components JSON built from it.
 * The limits mirror internal/modules/channels/components.go (Meta's
 * documented limits); the server checks again before submitting.
 */

export type Kind = 'standard' | 'carousel' | 'lto' | 'authentication';
export type HeaderKind =
    'NONE' | 'TEXT' | 'IMAGE' | 'VIDEO' | 'DOCUMENT' | 'LOCATION';
export type ButtonType =
    | 'QUICK_REPLY'
    | 'URL'
    | 'PHONE_NUMBER'
    | 'COPY_CODE'
    | 'VOICE_CALL'
    | 'FLOW';
export type CardButtonType = 'QUICK_REPLY' | 'URL' | 'PHONE_NUMBER';
export type OTPType = 'COPY_CODE' | 'ONE_TAP' | 'ZERO_TAP';

export type ButtonDraft = {
    key: number;
    type: ButtonType;
    text: string;
    /** URL, phone number, sample code or Flow ID by type. */
    value: string;
    /** Sample of a URL variable. */
    example: string;
    flowAction: 'navigate' | 'data_exchange';
};

export type CardButtonDraft = { text: string; value: string; example: string };

export type CardDraft = {
    key: number;
    handle: string;
    file: string;
    preview?: string;
    body: string;
    examples: Record<string, string>;
    buttons: CardButtonDraft[];
};

export type Draft = {
    account_id: string;
    name: string;
    language: string;
    kind: Kind;
    category: 'MARKETING' | 'UTILITY' | 'AUTHENTICATION';
    format: 'POSITIONAL' | 'NAMED';
    headerKind: HeaderKind;
    header: string;
    headerHandle: string;
    headerFile: string;
    headerPreview?: string;
    body: string;
    footer: string;
    /** "h:<var>" and "b:<var>" sample values. */
    examples: Record<string, string>;
    buttons: ButtonDraft[];
    ltoText: string;
    ltoExpiration: boolean;
    ltoCoupon: string;
    ltoUrl: { text: string; value: string; example: string };
    cardHeader: 'IMAGE' | 'VIDEO';
    cardBody: boolean;
    cardButtons: CardButtonType[];
    cards: CardDraft[];
    auth: {
        otpType: OTPType;
        security: boolean;
        expiry: string;
        text: string;
        autofill: string;
        packageName: string;
        signatureHash: string;
        zeroTapTerms: boolean;
    };
};

export const LIMITS = {
    header: 60,
    body: 1024,
    ltoBody: 600,
    cardBody: 160,
    footer: 60,
    buttons: 10,
    buttonText: 25,
    voiceText: 20,
    url: 2000,
    phone: 20,
    coupon: 20,
    ltoCoupon: 15,
    ltoText: 16,
    minCards: 2,
    cards: 10,
    cardButtons: 2,
} as const;

/** Per-type maxima (VOICE_CALL and FLOW: one each, not documented). */
export const BUTTON_LIMITS: Record<ButtonType, number> = {
    QUICK_REPLY: 10,
    URL: 2,
    PHONE_NUMBER: 1,
    COPY_CODE: 1,
    VOICE_CALL: 1,
    FLOW: 1,
};

/** Button types the type picker offers for a standard template. */
export function allowedButtons(d: Pick<Draft, 'kind'>): ButtonType[] {
    if (d.kind !== 'standard') return [];
    return [
        'QUICK_REPLY',
        'URL',
        'PHONE_NUMBER',
        'COPY_CODE',
        'VOICE_CALL',
        'FLOW',
    ];
}

/** Header formats a kind allows (location: utility and marketing only). */
export function allowedHeaders(kind: Kind): HeaderKind[] {
    switch (kind) {
        case 'standard':
            return ['NONE', 'TEXT', 'IMAGE', 'VIDEO', 'DOCUMENT', 'LOCATION'];
        case 'lto':
            return ['NONE', 'IMAGE', 'VIDEO'];
        default:
            return ['NONE'];
    }
}

export function emptyCard(key: number, types: CardButtonType[]): CardDraft {
    return {
        key,
        handle: '',
        file: '',
        body: '',
        examples: {},
        buttons: types.map(() => ({ text: '', value: '', example: '' })),
    };
}

export function emptyDraft(account: string): Draft {
    return {
        account_id: account,
        name: '',
        language: 'tr',
        kind: 'standard',
        category: 'MARKETING',
        format: 'POSITIONAL',
        headerKind: 'NONE',
        header: '',
        headerHandle: '',
        headerFile: '',
        body: '',
        footer: '',
        examples: {},
        buttons: [],
        ltoText: '',
        ltoExpiration: true,
        ltoCoupon: '',
        ltoUrl: { text: '', value: '', example: '' },
        cardHeader: 'IMAGE',
        cardBody: false,
        cardButtons: ['URL'],
        cards: [emptyCard(1, ['URL']), emptyCard(2, ['URL'])],
        auth: {
            otpType: 'COPY_CODE',
            security: true,
            expiry: '10',
            text: '',
            autofill: '',
            packageName: '',
            signatureHash: '',
            zeroTapTerms: false,
        },
    };
}

/** The variable "Add variable" inserts next. */
export function nextVariable(body: string, named: boolean): string {
    const vars = variablesOf(body);
    if (!named) {
        const max = Math.max(0, ...vars.map(Number).filter(Number.isFinite));
        return String(max + 1);
    }
    for (let i = 1; ; i++) {
        const v = i === 1 ? 'name' : `value_${i}`;
        if (!vars.includes(v)) return v;
    }
}

function sortVars(vars: string[], named: boolean): string[] {
    return named ? vars : [...vars].sort((a, b) => Number(a) - Number(b));
}

/** A BODY component with Meta's example block. */
function bodyComponent(
    text: string,
    sample: (v: string) => string,
    named: boolean,
): Record<string, unknown> {
    const c: Record<string, unknown> = { type: 'BODY', text };
    const vars = sortVars(variablesOf(text), named);
    if (vars.length > 0) {
        c.example = named
            ? {
                  body_text_named_params: vars.map((v) => ({
                      param_name: v,
                      example: sample(v),
                  })),
              }
            : { body_text: [vars.map(sample)] };
    }
    return c;
}

function urlButton(text: string, url: string, example: string) {
    const dyn = variablesOf(url).length > 0;
    return {
        type: 'URL',
        text: text.trim(),
        url: url.trim(),
        ...(dyn ? { example: [example.trim()] } : {}),
    };
}

/** Meta's components JSON for the draft. */
export function buildComponents(d: Draft): Record<string, unknown>[] {
    const named = d.format === 'NAMED';
    const out: Record<string, unknown>[] = [];
    if (d.kind === 'authentication') {
        const a = d.auth;
        out.push({ type: 'BODY', add_security_recommendation: a.security });
        if (a.expiry.trim())
            out.push({
                type: 'FOOTER',
                code_expiration_minutes: Number(a.expiry),
            });
        const btn: Record<string, unknown> = {
            type: 'OTP',
            otp_type: a.otpType,
        };
        if (a.text.trim()) btn.text = a.text.trim();
        if (a.otpType !== 'COPY_CODE') {
            if (a.autofill.trim()) btn.autofill_text = a.autofill.trim();
            btn.supported_apps = [
                {
                    package_name: a.packageName.trim(),
                    signature_hash: a.signatureHash.trim(),
                },
            ];
        }
        if (a.otpType === 'ZERO_TAP')
            btn.zero_tap_terms_accepted = a.zeroTapTerms;
        out.push({ type: 'BUTTONS', buttons: [btn] });
        return out;
    }
    if (d.kind !== 'carousel') {
        if (d.headerKind === 'TEXT' && d.header.trim()) {
            const text = d.header.trim();
            const vars = variablesOf(text);
            const c: Record<string, unknown> = {
                type: 'HEADER',
                format: 'TEXT',
                text,
            };
            const ex = (v: string) => d.examples[`h:${v}`] ?? '';
            if (vars.length > 0)
                c.example = named
                    ? {
                          header_text_named_params: vars.map((v) => ({
                              param_name: v,
                              example: ex(v),
                          })),
                      }
                    : { header_text: vars.map(ex) };
            out.push(c);
        } else if (d.headerKind === 'LOCATION') {
            out.push({ type: 'HEADER', format: 'LOCATION' });
        } else if (d.headerKind !== 'NONE' && d.headerKind !== 'TEXT') {
            out.push({
                type: 'HEADER',
                format: d.headerKind,
                ...(d.headerHandle
                    ? { example: { header_handle: [d.headerHandle] } }
                    : {}),
            });
        }
    }
    if (d.kind === 'lto')
        out.push({
            type: 'LIMITED_TIME_OFFER',
            limited_time_offer: {
                text: d.ltoText.trim(),
                has_expiration: d.ltoExpiration,
            },
        });
    out.push(
        bodyComponent(
            d.body.trim(),
            (v) => (d.examples[`b:${v}`] ?? '').trim(),
            named,
        ),
    );
    if (d.kind === 'standard') {
        if (d.footer.trim())
            out.push({ type: 'FOOTER', text: d.footer.trim() });
        if (d.buttons.length > 0)
            out.push({
                type: 'BUTTONS',
                buttons: d.buttons.map((b) => {
                    switch (b.type) {
                        case 'URL':
                            return urlButton(b.text, b.value, b.example);
                        case 'PHONE_NUMBER':
                            return {
                                type: 'PHONE_NUMBER',
                                text: b.text.trim(),
                                phone_number: b.value.trim(),
                            };
                        case 'COPY_CODE':
                            return {
                                type: 'COPY_CODE',
                                example: b.value.trim(),
                            };
                        case 'VOICE_CALL':
                            return {
                                type: 'VOICE_CALL',
                                ...(b.text.trim()
                                    ? { text: b.text.trim() }
                                    : {}),
                            };
                        case 'FLOW': {
                            const id = b.value.trim();
                            return {
                                type: 'FLOW',
                                text: b.text.trim(),
                                ...(/^[0-9]+$/.test(id)
                                    ? { flow_id: id }
                                    : { flow_name: id }),
                                flow_action: b.flowAction,
                            };
                        }
                        default:
                            return { type: 'QUICK_REPLY', text: b.text.trim() };
                    }
                }),
            });
    }
    if (d.kind === 'lto') {
        const buttons: Record<string, unknown>[] = [];
        if (d.ltoCoupon.trim())
            buttons.push({ type: 'COPY_CODE', example: d.ltoCoupon.trim() });
        buttons.push(
            urlButton(d.ltoUrl.text, d.ltoUrl.value, d.ltoUrl.example),
        );
        out.push({ type: 'BUTTONS', buttons });
    }
    if (d.kind === 'carousel') {
        out.push({
            type: 'CAROUSEL',
            cards: d.cards.map((c) => {
                const comps: Record<string, unknown>[] = [
                    {
                        type: 'HEADER',
                        format: d.cardHeader,
                        ...(c.handle
                            ? { example: { header_handle: [c.handle] } }
                            : {}),
                    },
                ];
                if (d.cardBody)
                    comps.push(
                        bodyComponent(
                            c.body.trim(),
                            (v) => (c.examples[v] ?? '').trim(),
                            named,
                        ),
                    );
                if (d.cardButtons.length > 0)
                    comps.push({
                        type: 'BUTTONS',
                        buttons: d.cardButtons.map((type, j) => {
                            const b = c.buttons[j] ?? {
                                text: '',
                                value: '',
                                example: '',
                            };
                            if (type === 'URL')
                                return urlButton(b.text, b.value, b.example);
                            if (type === 'PHONE_NUMBER')
                                return {
                                    type,
                                    text: b.text.trim(),
                                    phone_number: b.value.trim(),
                                };
                            return { type, text: b.text.trim() };
                        }),
                    });
                return { components: comps };
            }),
        });
    }
    return out;
}

const len = (s: string) => [...s].length;

function checkVariables(
    t: Translate,
    vars: string[],
    named: boolean,
): string | null {
    if (!named) {
        const bad = vars.find((v) => !/^[1-9][0-9]*$/.test(v));
        if (bad)
            return t('templates.validation.positionalNumbers', {
                variable: `{{${bad}}}`,
            });
        const nums = vars.map(Number).sort((a, b) => a - b);
        if (nums.some((n, i) => n !== i + 1))
            return t('templates.validation.positionalOrder');
        return null;
    }
    const bad = vars.find((v) => !/^[a-z_][a-z0-9_]*$/.test(v));
    return bad
        ? t('templates.validation.namedChars', { variable: `{{${bad}}}` })
        : null;
}

function checkUrl(
    t: Translate,
    e: Record<string, string>,
    key: string,
    url: string,
    example: string,
) {
    const u = url.trim();
    if (!/^https?:\/\/\S+$/.test(u) || len(u) > LIMITS.url)
        e[`${key}-value`] = t('templates.validation.https');
    else if (variablesOf(u).length > 0) {
        if (
            variablesOf(u).length > 1 ||
            !/\{\{\s*[A-Za-z0-9_]+\s*\}\}$/.test(u)
        )
            e[`${key}-value`] = t('templates.validation.urlVariable');
        else if (!example.trim())
            e[`${key}-example`] = t('templates.validation.urlExample');
    }
}

/** Problems Meta would reject anyway, keyed by field id. */
export function validateDraft(t: Translate, d: Draft): Record<string, string> {
    const e: Record<string, string> = {};
    const named = d.format === 'NAMED';
    if (!d.account_id) e.account_id = t('templates.validation.chooseAccount');
    if (!/^[a-z0-9_]{1,512}$/.test(d.name))
        e.name = t('templates.validation.name');
    if (d.language.trim().length < 2)
        e.language = t('templates.validation.language');

    if (d.kind === 'authentication') {
        const a = d.auth;
        const m = Number(a.expiry);
        if (a.expiry.trim() && (!Number.isInteger(m) || m < 1 || m > 90))
            e['otp-expiry'] = t('templates.validation.otpExpiry');
        if (len(a.text) > LIMITS.buttonText)
            e['otp-text'] = t('templates.validation.max', {
                max: LIMITS.buttonText,
            });
        if (a.otpType !== 'COPY_CODE') {
            if (len(a.autofill) > LIMITS.buttonText)
                e['otp-autofill'] = t('templates.validation.max', {
                    max: LIMITS.buttonText,
                });
            if (
                !/^[a-zA-Z][a-zA-Z0-9_]*(\.[a-zA-Z][a-zA-Z0-9_]*)+$/.test(
                    a.packageName.trim(),
                )
            )
                e['otp-package'] = t('templates.validation.package');
            if (!/^[a-zA-Z0-9+/=]{11}$/.test(a.signatureHash.trim()))
                e['otp-hash'] = t('templates.validation.signature');
        }
        if (a.otpType === 'ZERO_TAP' && !a.zeroTapTerms)
            e['otp-terms'] = t('templates.validation.zeroTapTerms');
        return e;
    }

    // Header.
    if (d.kind !== 'carousel') {
        if (d.headerKind === 'TEXT') {
            const hv = variablesOf(d.header);
            if (!d.header.trim())
                e.header = t('templates.validation.headerText');
            else if (len(d.header) > LIMITS.header)
                e.header = t('templates.validation.max60');
            else if (hv.length > 1)
                e.header = t('templates.validation.headerOneVariable');
            else {
                const p = checkVariables(t, hv, named);
                if (p) e.header = p;
            }
            for (const v of hv)
                if (!d.examples[`h:${v}`]?.trim())
                    e[`example-h:${v}`] = t(
                        'templates.validation.exampleRequired',
                    );
        }
        if (
            (d.headerKind === 'IMAGE' ||
                d.headerKind === 'VIDEO' ||
                d.headerKind === 'DOCUMENT') &&
            !d.headerHandle
        )
            e.header = t('templates.validation.headerSample');
    }

    // Body.
    const max = d.kind === 'lto' ? LIMITS.ltoBody : LIMITS.body;
    if (!d.body.trim()) e.body = t('templates.validation.bodyRequired');
    else if (len(d.body) > max) e.body = t('templates.validation.max', { max });
    const bodyProblem = checkVariables(t, variablesOf(d.body), named);
    if (bodyProblem) e.body = bodyProblem;
    for (const v of variablesOf(d.body))
        if (!d.examples[`b:${v}`]?.trim())
            e[`example-b:${v}`] = t('templates.validation.exampleRequired');

    if (d.kind === 'standard') {
        if (len(d.footer) > LIMITS.footer)
            e.footer = t('templates.validation.max60');
        if (variablesOf(d.footer).length > 0)
            e.footer = t('templates.validation.footerNoVariables');
        if (d.buttons.length > LIMITS.buttons)
            e.buttons = t('templates.validation.tooManyButtons');
        // Quick replies form one block, before or after the others.
        const groups = d.buttons.reduce(
            (n, b, i) =>
                i === 0 ||
                (b.type === 'QUICK_REPLY') !==
                    (d.buttons[i - 1]!.type === 'QUICK_REPLY')
                    ? n + 1
                    : n,
            0,
        );
        if (groups > 2 && d.buttons.some((b) => b.type === 'QUICK_REPLY'))
            e.buttons = t('templates.validation.quickReplyGroup');
        d.buttons.forEach((b, i) => {
            const k = `button-${i}`;
            const textMax =
                b.type === 'VOICE_CALL' ? LIMITS.voiceText : LIMITS.buttonText;
            if (
                b.type !== 'COPY_CODE' &&
                b.type !== 'VOICE_CALL' &&
                !b.text.trim()
            )
                e[`${k}-text`] = t('templates.validation.buttonTextRequired');
            else if (len(b.text) > textMax)
                e[`${k}-text`] = t('templates.validation.max', {
                    max: textMax,
                });
            if (b.type === 'URL') checkUrl(t, e, k, b.value, b.example);
            if (
                b.type === 'PHONE_NUMBER' &&
                !/^\+?[0-9 ]{7,20}$/.test(b.value.trim())
            )
                e[`${k}-value`] = t('templates.validation.phone');
            if (b.type === 'COPY_CODE') {
                if (!b.value.trim())
                    e[`${k}-value`] = t('templates.validation.couponRequired');
                else if (len(b.value.trim()) > LIMITS.coupon)
                    e[`${k}-value`] = t('templates.validation.max', {
                        max: LIMITS.coupon,
                    });
            }
            if (b.type === 'FLOW' && !b.value.trim())
                e[`${k}-value`] = t('templates.validation.flowRequired');
        });
    }

    if (d.kind === 'lto') {
        if (!d.ltoText.trim())
            e['lto-text'] = t('templates.validation.ltoText');
        else if (len(d.ltoText) > LIMITS.ltoText)
            e['lto-text'] = t('templates.validation.max', {
                max: LIMITS.ltoText,
            });
        if (len(d.ltoCoupon.trim()) > LIMITS.ltoCoupon)
            e['lto-coupon'] = t('templates.validation.max', {
                max: LIMITS.ltoCoupon,
            });
        if (!d.ltoUrl.text.trim())
            e['lto-url-text'] = t('templates.validation.buttonTextRequired');
        else if (len(d.ltoUrl.text) > LIMITS.buttonText)
            e['lto-url-text'] = t('templates.validation.max', {
                max: LIMITS.buttonText,
            });
        checkUrl(t, e, 'lto-url', d.ltoUrl.value, d.ltoUrl.example);
        // checkUrl writes "<key>-value"; the LTO fields are named without it.
        if (e['lto-url-value']) {
            e['lto-url'] = e['lto-url-value'];
            delete e['lto-url-value'];
        }
    }

    if (d.kind === 'carousel') {
        if (d.cards.length < LIMITS.minCards || d.cards.length > LIMITS.cards)
            e.cards = t('templates.validation.cardCount');
        d.cards.forEach((c, k) => {
            const f = (x: string) => `card-${k}-${x}`;
            if (!c.handle) e[f('media')] = t('templates.validation.cardSample');
            if (d.cardBody) {
                if (!c.body.trim())
                    e[f('body')] = t('templates.validation.bodyRequired');
                else if (len(c.body) > LIMITS.cardBody)
                    e[f('body')] = t('templates.validation.max', {
                        max: LIMITS.cardBody,
                    });
                const p = checkVariables(t, variablesOf(c.body), named);
                if (p) e[f('body')] = p;
                for (const v of variablesOf(c.body))
                    if (!c.examples[v]?.trim())
                        e[`${f('example')}-${v}`] = t(
                            'templates.validation.exampleRequired',
                        );
            }
            d.cardButtons.forEach((type, j) => {
                const b = c.buttons[j] ?? { text: '', value: '', example: '' };
                const bk = `${f('button')}-${j}`;
                if (!b.text.trim())
                    e[`${bk}-text`] = t(
                        'templates.validation.buttonTextRequired',
                    );
                else if (len(b.text) > LIMITS.buttonText)
                    e[`${bk}-text`] = t('templates.validation.max', {
                        max: LIMITS.buttonText,
                    });
                if (type === 'URL') checkUrl(t, e, bk, b.value, b.example);
                if (
                    type === 'PHONE_NUMBER' &&
                    !/^\+?[0-9 ]{7,20}$/.test(b.value.trim())
                )
                    e[`${bk}-value`] = t('templates.validation.phone');
            });
        });
    }
    return e;
}
