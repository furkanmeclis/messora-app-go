import type { Schemas } from '@/lib/types';

/**
 * Requests and responses of the sign-up, verification and workspace
 * endpoints (phase 17 WP2), as generated from api/openapi.yaml.
 *
 * Every endpoint uses the usual envelope: `{ data, meta }` on success and
 * `{ error: { code, message, details[] }, meta }` on failure.
 */

/** POST /api/v1/auth/signup (anonymous, any host). */
export type SignupRequest = Schemas['SignupRequest'];

/**
 * 201. No session yet: the address has to be verified first.
 * Errors: 422 VALIDATION_ERROR (details per field), 409 EMAIL_TAKEN,
 * 409 SLUG_TAKEN, 422 SLUG_RESERVED, 403 SIGNUP_DISABLED, 429 RATE_LIMITED.
 */
export type SignupResponse = Schemas['Signup'];

/** GET /api/v1/auth/signup/slug-availability?slug=acme. */
export type SlugAvailability = Schemas['SlugAvailability'];

/** POST /api/v1/auth/verify-email. */
export type VerifyEmailRequest = Schemas['VerifyEmailRequest'];

/**
 * 200. Errors: 410 VERIFICATION_TOKEN_EXPIRED, 400 VERIFICATION_TOKEN_INVALID,
 * 409 EMAIL_ALREADY_VERIFIED.
 */
export type VerifyEmailResponse = Schemas['VerifyEmailResult'];

/** POST /api/v1/auth/resend-verification: 202, or 429 with Retry-After. */
export type ResendVerificationRequest = Schemas['ResendVerificationRequest'];

/** GET /api/v1/me/memberships (session). */
export type Membership = Schemas['Membership'];

/** POST /api/v1/auth/switch-workspace (session). */
export type SwitchWorkspaceRequest = Schemas['SwitchWorkspaceRequest'];
export type SwitchWorkspaceResponse = Schemas['SwitchWorkspace'];

/** Error codes the auth screens treat specially. */
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
    emailNotVerified: 'EMAIL_NOT_VERIFIED',
    passwordResetRequired: 'PASSWORD_RESET_REQUIRED',
    handoffInvalid: 'HANDOFF_INVALID',
} as const;

/** Legal pages linked from every sign-in screen. */
export const LEGAL_LINKS = {
    privacy: 'https://messora.tech/gizlilik',
    terms: 'https://messora.tech/kosullar',
} as const;
