/**
 * Meta's Embedded Signup (Messora as Tech Provider, phase 17 WP6): the parts
 * that do not need a browser, so they are unit-tested.
 *
 * The flow (developers.facebook.com/docs/whatsapp/embedded-signup):
 * the JavaScript SDK opens Meta's popup with FB.login({config_id,
 * response_type: 'code', override_default_response_type: true, extras});
 * the popup posts a WA_EMBEDDED_SIGNUP message (FINISH with the WABA and
 * phone number ids, CANCEL or ERROR) to the page, and FB.login's callback
 * gets the code the server trades for the business token.
 */

/** Where the SDK is loaded from (the only extra script origin). */
export const SDK_URL = 'https://connect.facebook.net/en_US/sdk.js';

/** The settings page that runs Embedded Signup; only it relaxes the CSP. */
export const SIGNUP_PAGE = '/settings/meta';

/**
 * The CSP additions the SDK needs on the signup page: its script, its
 * cross-domain frames on facebook.com, and the calls it makes there.
 */
export const SDK_CSP = {
    script: 'https://connect.facebook.net',
    frame: 'https://www.facebook.com https://web.facebook.com https://staticxx.facebook.com',
    connect:
        'https://www.facebook.com https://graph.facebook.com https://connect.facebook.net',
    img: 'https://www.facebook.com',
} as const;

/** Whether pathname is the signup page (the relaxed CSP applies). */
export function isSignupPage(pathname: string): boolean {
    return pathname === SIGNUP_PAGE || pathname === SIGNUP_PAGE + '/';
}

/** Whether a message comes from Meta's popup (an https facebook.com origin). */
export function isMetaOrigin(origin: string): boolean {
    let u: URL;
    try {
        u = new URL(origin);
    } catch {
        return false;
    }
    return (
        u.protocol === 'https:' &&
        (u.hostname === 'facebook.com' || u.hostname.endsWith('.facebook.com'))
    );
}

export type SignupEvent =
    | {
          kind: 'finish';
          wabaId: string;
          phoneNumberId: string;
          businessId?: string;
      }
    /** A WhatsApp Business app number (coexistence): not supported yet. */
    | { kind: 'finish_app'; wabaId?: string }
    | { kind: 'cancel'; step: string }
    | { kind: 'error'; message: string; code?: string };

const DIGITS = /^[0-9]{1,32}$/;

const str = (v: unknown): string | undefined =>
    typeof v === 'string' ? v : typeof v === 'number' ? String(v) : undefined;

/**
 * Parses a WA_EMBEDDED_SIGNUP message (a JSON string or an object); null
 * for anything else. Ids must be numeric: the server checks them against
 * the business token anyway.
 */
export function parseSignupMessage(data: unknown): SignupEvent | null {
    let msg: unknown = data;
    if (typeof data === 'string') {
        try {
            msg = JSON.parse(data);
        } catch {
            return null;
        }
    }
    if (!msg || typeof msg !== 'object') return null;
    const m = msg as Record<string, unknown>;
    if (m.type !== 'WA_EMBEDDED_SIGNUP') return null;
    const d =
        m.data && typeof m.data === 'object'
            ? (m.data as Record<string, unknown>)
            : {};
    switch (m.event) {
        case 'FINISH':
        case 'FINISH_ONLY_WABA': {
            const wabaId = str(d.waba_id);
            const phoneNumberId = str(d.phone_number_id);
            if (!wabaId || !DIGITS.test(wabaId)) return null;
            if (!phoneNumberId || !DIGITS.test(phoneNumberId)) {
                return {
                    kind: 'error',
                    message: 'No phone number was chosen in Meta’s window.',
                };
            }
            const businessId = str(d.business_id);
            return {
                kind: 'finish',
                wabaId,
                phoneNumberId,
                businessId:
                    businessId && DIGITS.test(businessId)
                        ? businessId
                        : undefined,
            };
        }
        case 'FINISH_WHATSAPP_BUSINESS_APP_ONBOARDING':
            return { kind: 'finish_app', wabaId: str(d.waba_id) };
        case 'CANCEL':
            return { kind: 'cancel', step: str(d.current_step) ?? '' };
        case 'ERROR':
            return {
                kind: 'error',
                message: str(d.error_message) ?? '',
                code: str(d.error_code),
            };
    }
    return null;
}

/** FB.login's options for Embedded Signup with configuration configId. */
export function loginOptions(configId: string) {
    return {
        config_id: configId,
        response_type: 'code',
        override_default_response_type: true,
        // Embedded Signup v4: the configuration decides the flow; v2/v3's
        // sessionInfoVersion and featureType are retired (Meta, 2026-10-15).
        extras: { setup: {} },
    };
}
