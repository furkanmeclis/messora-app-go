/** Messages for API error codes. */
const errors = {
    ACCOUNT_DISABLED: 'This account is disabled.',
    ACCOUNT_EXISTS: 'This WhatsApp account is already connected.',
    ACCOUNT_HAS_CAMPAIGNS:
        'This account still has campaigns. Delete or move them first.',
    ACCOUNT_HAS_MESSAGES:
        'This account has message history and cannot be removed.',
    ACCOUNT_LOCKED: 'Too many failed attempts. Try again later.',
    API_KEY_INVALID: 'The API key is invalid.',
    CAMPAIGN_ACTIVE: 'The campaign is running. Pause it first.',
    CAMPAIGN_STARTED: 'The campaign has already started.',
    CHANNEL_FORBIDDEN: 'You cannot use this channel.',
    CONFLICT: 'This conflicts with the current state. Reload and try again.',
    CONTACT_EXISTS: 'A contact with this phone number already exists.',
    DELIVERY_IN_PROGRESS: 'A delivery is in progress. Try again shortly.',
    EMAIL_NOT_CONFIGURED: 'E-mail is not configured.',
    FORBIDDEN: 'You do not have permission to do this.',
    GROUP_EXISTS: 'A group with this name already exists.',
    IDEMPOTENCY_IN_PROGRESS: 'The same request is still being processed.',
    IDEMPOTENCY_KEY_REUSED:
        'This request was already sent with different content.',
    IMPORT_FINISHED: 'This import has already finished.',
    IMPORT_STARTED: 'This import has already started.',
    INSUFFICIENT_ROLE: 'Your role does not allow this.',
    INTERNAL_ERROR: 'Something went wrong on the server. Try again.',
    INVALID_CREDENTIALS: 'The e-mail address or password is wrong.',
    INVALID_TRANSITION: 'This action is not possible in the current state.',
    INVITATION_EXISTS: 'This person already has a pending invitation.',
    INVITATION_INVALID: 'This invitation is invalid or has expired.',
    LAST_OWNER: 'The workspace needs at least one owner.',
    MAIL_UNAVAILABLE: 'E-mail settings are not available.',
    MEDIA_NOT_STORED: 'This file is no longer stored.',
    META_DRY_RUN: 'Requests to Meta are disabled on this server (dry run).',
    META_UNREACHABLE: 'Meta could not be reached. Try again.',
    METHOD_NOT_ALLOWED: 'This action is not supported.',
    MFA_ALREADY_ENABLED: 'Two-factor authentication is already on.',
    MFA_EXPIRED: 'The sign-in step expired. Sign in again.',
    MFA_INVALID: 'The code is wrong or has expired.',
    MFA_NOT_ENABLED: 'Two-factor authentication is not on.',
    MFA_NOT_STARTED: 'Start the two-factor set-up first.',
    MFA_REQUIRED_BY_POLICY: 'Your role requires two-factor authentication.',
    MFA_SETUP_REQUIRED: 'Set up two-factor authentication to continue.',
    NOT_FOUND: 'Not found. It may have been deleted.',
    NO_PHONE_NUMBER: 'Connect a WhatsApp number first.',
    NUMBER_OWNED_ELSEWHERE: 'This number belongs to another account.',
    OWNER_ONLY: 'Only the owner can do this.',
    PAYLOAD_TOO_LARGE: 'The file or request is too large.',
    RATE_LIMITED: 'Too many requests. Wait a moment and try again.',
    REAUTH_REQUIRED: 'Confirm your password to continue.',
    RESET_TOKEN_INVALID: 'This reset link is invalid or has expired.',
    SCOPE_NOT_ALLOWED: 'This API key scope is not allowed.',
    SEGMENT_EXISTS: 'A segment with this name already exists.',
    SELF_MODIFICATION: 'You cannot change your own role or status.',
    SERVICE_UNAVAILABLE: 'The service is temporarily unavailable.',
    SESSION_INVALID: 'Your session has ended. Sign in again.',
    SETTING_MANAGED_BY_ENV:
        'This setting is managed by an environment variable.',
    SETUP_COMPLETE: 'Setup is already complete.',
    SETUP_TOKEN_INVALID: 'The setup token is invalid or has expired.',
    SMTP_MANAGED_BY_ENV: 'E-mail is configured by environment variables.',
    TAG_EXISTS: 'A tag with this name already exists.',
    UNAUTHORIZED: 'Sign in to continue.',
    USER_EXISTS: 'A user with this e-mail address already exists.',
    EMAIL_TAKEN: 'An account with this e-mail already exists. Sign in instead.',
    SLUG_TAKEN: 'This panel address is already in use.',
    SLUG_RESERVED: 'This panel address is reserved. Try another.',
    SIGNUP_DISABLED: 'Sign-ups are closed right now.',
    VERIFICATION_TOKEN_EXPIRED: 'This verification link has expired.',
    VERIFICATION_TOKEN_INVALID:
        'This verification link is invalid or was already used.',
    EMAIL_ALREADY_VERIFIED: 'This e-mail address is already verified.',
    VALIDATION_ERROR: 'Check the highlighted fields: {detail}',
    WEBHOOK_LIMIT: 'The webhook limit has been reached.',
    PLAN_LIMIT_REACHED: 'You have reached the limit of your plan.',
    FEATURE_NOT_IN_PLAN: 'Your plan does not include this feature.',
    PLAN_REQUEST_OPEN: 'A plan change request is already open.',
    AI_CREDITS_EXHAUSTED: 'This month’s AI credits are used up.',
    SUBSCRIPTION_READ_ONLY:
        'The workspace is read-only until its subscription is paid: reading and exports work, sending and creating do not.',
    MESSAGE_BALANCE_EXHAUSTED:
        'The message balance is used up; top it up in Billing to send templates again.',
    BILLING_PROFILE_REQUIRED: 'Enter the invoice details first.',
    BILLING_NOT_CONFIGURED: 'Payments are not set up yet.',
    PLAN_NOT_SELF_SERVE:
        'This plan is priced per offer; ask Messora for an offer.',
    RENEWAL_DUE: 'Pay the open renewal first; then change the plan.',
    PROFORMA_OPEN:
        'A proforma with a payment on it is open; pay it in full first.',
    PROFORMA_NOT_OPEN:
        'Only an open proforma with nothing paid can be cancelled.',
    INVOICE_PDF_PENDING: 'The official invoice PDF is not attached yet.',
    TRANSFER_NOT_UNMATCHED: 'This transfer is matched or ignored already.',
    PERIOD_ENDING:
        'The period ends today; choose the plan for the next period.',
    QUOTA_EXCEEDED: 'The monthly API quota of this workspace is used up.',
    AI_UNAVAILABLE: 'The AI is not available here: {detail}',
    DRAFT_DECIDED: 'Someone already decided on this draft.',
    AI_DISABLED:
        'AI is turned off for the company. Turn it on in Settings → AI.',
    AI_NOT_CONFIGURED: 'AI is not configured: {detail}',
    AI_RATE_LIMITED:
        'The AI provider is rate limiting requests. Try again shortly.',
    AI_QUOTA_EXCEEDED:
        'The AI provider account has no credit or reached its spend limit.',
    AI_AUTH_FAILED: 'The AI provider rejected the API key.',
    AI_PROVIDER_UNAVAILABLE:
        'The AI provider is unavailable or overloaded. Try again shortly.',
    AI_BAD_REQUEST: 'The AI provider rejected the request: {detail}',
    AI_REFUSED: 'The model declined to answer.',
    AI_INVALID_OUTPUT: 'The model’s answer did not match the expected format.',
    AI_PROVIDER_ERROR: 'The AI provider request failed: {detail}',
    NETWORK: 'The server could not be reached. Check your connection.',
    TECH_PROVIDER_DISABLED:
        "Connecting through Messora's Meta app is not available.",
    ONBOARDING_RUNNING:
        'This account is being connected already; wait a moment.',
    ONBOARDING_FINISHED: 'This connection has finished; start a new one.',
    MANAGED_BY_MESSORA:
        "This account is connected through Messora's Meta app; Messora manages its credentials.",
    TECH_PROVIDER_DISCONNECTED:
        'This account was disconnected from Messora; connect it again with Meta.',
    WABA_CONNECTED_TECH_PROVIDER:
        "This WhatsApp Business Account is connected through Messora's Meta app; disconnect it there first.",
    NOT_TECH_PROVIDER: 'This account is connected with your own Meta app.',
    PLATFORM_WRITE_DENIED:
        "Only the platform admin can change Messora's Tech Provider settings.",
};
export default errors;
