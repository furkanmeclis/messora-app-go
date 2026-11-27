/**
 * Requests and responses of the identity endpoints the new sign-up,
 * verification and workspace screens call. They are not in the API yet
 * (phase 17 WP2); once they are, these types should be replaced by
 * `Schemas[...]` from the generated OpenAPI schema with the same shapes.
 *
 * Every endpoint uses the usual envelope: `{ data, meta }` on success and
 * `{ error: { code, message, details[] }, meta }` on failure. Until an
 * endpoint exists the API answers 404 and the screens show a friendly
 * "coming soon" state instead of an error.
 */

/** POST /api/v1/auth/signup (anonymous). */
export type SignupRequest = {
    name: string;
    email: string;
    password: string;
    company_name: string;
    /** a–z0–9-, 3–30 characters, not reserved (see lib/auth/slug). */
    workspace_slug: string;
    /** The language the person used; e-mails go out in it. */
    locale: 'tr' | 'en';
    /** Always true; the form does not submit otherwise. */
    accept_terms: true;
};

/**
 * 201. No session yet: the address has to be verified first.
 * Errors: 422 VALIDATION_ERROR (details per field: name, email, password,
 * company_name, workspace_slug), 409 EMAIL_TAKEN, 409 SLUG_TAKEN,
 * 422 SLUG_RESERVED, 403 SIGNUP_DISABLED, 429 RATE_LIMITED.
 */
export type SignupResponse = {
    email: string;
    tenant: { id: string; slug: string; name: string };
    verification_required: boolean;
};

/**
 * GET /api/v1/auth/signup/slug-availability?slug=acme (anonymous).
 * 200 always for a well-formed slug; 422 VALIDATION_ERROR otherwise.
 */
export type SlugAvailability = {
    slug: string;
    available: boolean;
    /** Why not: "taken" or "reserved". */
    reason?: 'taken' | 'reserved';
};

/** POST /api/v1/auth/verify-email (anonymous). */
export type VerifyEmailRequest = { token: string };

/**
 * 200. Errors: 410 VERIFICATION_TOKEN_EXPIRED (the link is old; a new one
 * can be requested), 400 VERIFICATION_TOKEN_INVALID (unknown or used),
 * 409 EMAIL_ALREADY_VERIFIED.
 */
export type VerifyEmailResponse = {
    email: string;
    tenant: { slug: string; name: string };
    /** Where to sign in: https://<slug>.messora.tech/login. */
    login_url: string;
};

/**
 * POST /api/v1/auth/resend-verification (anonymous). Answers 202 with an
 * empty body whether or not the address is known (no account probing).
 * Errors: 429 RATE_LIMITED with a Retry-After header in seconds.
 */
export type ResendVerificationRequest = { email: string };

/** GET /api/v1/me/memberships (session). */
export type Membership = {
    tenant: {
        id: string;
        slug: string;
        name: string;
        /** Absolute or same-origin URL of the workspace logo. */
        logo_url: string | null;
    };
    role: { key: string; name: string };
    /** The workspace this session belongs to. */
    current: boolean;
    /** https://<slug>.messora.tech (or the custom domain). */
    url: string;
};

/**
 * POST /api/v1/auth/switch-workspace (session). Sessions belong to one
 * workspace; switching hands over to the other address with a one-time
 * token. Errors: 403 FORBIDDEN (no membership), 403 ACCOUNT_DISABLED.
 */
export type SwitchWorkspaceRequest = { tenant_id: string };
export type SwitchWorkspaceResponse = {
    /** https://<slug>.messora.tech/auth/handoff?token=… */
    redirect_url: string;
};

/**
 * Extra fields WP2 may add to `InvitationPreview` (POST
 * /invitations/lookup). The invitation screen shows them when present.
 */
export type InvitationPreviewExtras = {
    invited_by?: { name: string } | null;
    tenant?: { name: string; logo_url: string | null } | null;
};

/** Error codes the new screens treat specially. */
export const AUTH_ERROR_CODES = {
    emailTaken: 'EMAIL_TAKEN',
    slugTaken: 'SLUG_TAKEN',
    slugReserved: 'SLUG_RESERVED',
    signupDisabled: 'SIGNUP_DISABLED',
    verificationExpired: 'VERIFICATION_TOKEN_EXPIRED',
    verificationInvalid: 'VERIFICATION_TOKEN_INVALID',
    alreadyVerified: 'EMAIL_ALREADY_VERIFIED',
    resetTokenInvalid: 'RESET_TOKEN_INVALID',
    invitationInvalid: 'INVITATION_INVALID',
    /** Not an API code: the sign-up endpoint is not there yet (404). */
    signupUnavailable: 'SIGNUP_UNAVAILABLE',
} as const;

/** Legal pages linked from every sign-in screen. */
export const LEGAL_LINKS = {
    privacy: 'https://messora.tech/gizlilik',
    terms: 'https://messora.tech/kosullar',
} as const;
