'use client';

import Link from 'next/link';
import {
    startTransition,
    useActionState,
    useEffect,
    useRef,
    useState,
} from 'react';
import {
    AlertCircle,
    ArrowLeft,
    CheckCircle2,
    KeyRound,
    MailCheck,
    ShieldCheck,
} from 'lucide-react';
import { Checkbox } from '@/components/ui/checkbox';
import { Label } from '@/components/ui/label';
import { Button } from '@/components/ui/button';
import { Spinner } from '@/components/ui/spinner';
import { useT } from '@/i18n/client';
import { AUTH_ERROR_CODES, LEGAL_LINKS } from '@/lib/auth/contracts';
import { PASSWORD_MIN } from '@/lib/auth/password-strength';
import { sanitizeSlugInput, slugify, validateSlug } from '@/lib/auth/slug';
import { cn } from '@/lib/utils';
import {
    AuthField,
    AuthSubmit,
    ErrorSummary,
    FormAlert,
    OtpInput,
    PasswordField,
} from './_components/fields';
import { rules, useAuthForm } from './_components/use-auth-form';
import {
    acceptInvitationAction,
    checkSlugAction,
    forgotPasswordAction,
    loginAction,
    resendVerificationAction,
    resetPasswordAction,
    setupAction,
    signupAction,
    verifyEmailAction,
    verifyMfaAction,
    type ResendState,
} from './actions';

/** The server's overall message, unless it is only about fields. */
function ServerAlert({
    state,
}: {
    state: {
        ok: boolean;
        code?: string;
        message?: string;
        fieldErrors?: Record<string, string>;
    } | null;
}) {
    if (!state || state.ok || !state.message) return null;
    if (
        state.code === 'VALIDATION_ERROR' &&
        state.fieldErrors &&
        Object.keys(state.fieldErrors).length > 0
    ) {
        return null;
    }
    return (
        <FormAlert>
            <AlertCircle className="mt-0.5 size-4 shrink-0" aria-hidden />
            <span>{state.message}</span>
        </FormAlert>
    );
}

function useCommonRules() {
    const t = useT();
    return {
        email: rules.email(
            t('auth.validation.required'),
            t('auth.validation.email'),
        ),
        required: rules.required(t('auth.validation.required')),
        newPassword: rules.minLength(
            PASSWORD_MIN,
            t('auth.validation.required'),
            t('auth.validation.passwordMin'),
        ),
    };
}

export function LoginForm({
    next,
    canReset,
}: {
    next: string;
    canReset: boolean;
}) {
    const t = useT();
    const r = useCommonRules();
    const f = useAuthForm(loginAction, {
        email: r.email,
        password: r.required,
    });
    return (
        <form {...f.formProps} className="flex flex-col gap-5">
            <input type="hidden" name="next" value={next} />
            <ErrorSummary
                title={t('auth.validation.summary', {
                    count: Object.values(f.summaryErrors).filter(Boolean)
                        .length,
                })}
                errors={f.summaryErrors}
                labels={{
                    email: t('auth.email'),
                    password: t('auth.password'),
                }}
                summaryRef={f.summaryRef}
            />
            <ServerAlert state={f.state} />
            <AuthField
                label={t('auth.email')}
                name="email"
                defaultValue={f.values.email}
                type="email"
                inputMode="email"
                autoComplete="username email"
                autoCapitalize="none"
                spellCheck={false}
                required
                autoFocus
                error={f.errorFor('email')}
            />
            <div className="flex flex-col gap-1">
                <PasswordField
                    label={t('auth.password')}
                    name="password"
                    autoComplete="current-password"
                    required
                    error={f.errorFor('password')}
                />
            </div>
            <div className="-my-1 flex flex-wrap items-center justify-between gap-x-4">
                <Label className="flex min-h-11 cursor-pointer items-center gap-2.5 text-sm font-normal">
                    <Checkbox
                        name="remember"
                        defaultChecked={
                            f.values.remember !== undefined || !f.state
                        }
                        className="size-[18px] rounded-[5px]"
                    />
                    {t('auth.login.remember')}
                </Label>
                {canReset && (
                    <Link
                        href="/forgot-password"
                        className="inline-flex min-h-11 items-center rounded-md text-sm font-medium text-primary underline-offset-4 hover:underline focus-visible:ring-3 focus-visible:ring-ring/50 focus-visible:outline-none"
                    >
                        {t('auth.login.forgot')}
                    </Link>
                )}
            </div>
            <AuthSubmit>{t('auth.login.submit')}</AuthSubmit>
        </form>
    );
}

export function VerifyForm({ next }: { next: string }) {
    const t = useT();
    const [recovery, setRecovery] = useState(false);
    const f = useAuthForm(
        verifyMfaAction,
        recovery
            ? { recovery_code: rules.required(t('auth.validation.required')) }
            : {
                  code: (v) =>
                      /^\d{6}$/.test(v) ? undefined : t('auth.validation.code'),
              },
    );
    return (
        <form {...f.formProps} className="flex flex-col gap-5">
            <input type="hidden" name="next" value={next} />
            <ServerAlert state={f.state} />
            {recovery ? (
                <AuthField
                    key="recovery"
                    label={t('auth.verify.recoveryCode')}
                    name="recovery_code"
                    placeholder="xxxxx-xxxxx"
                    autoComplete="one-time-code"
                    autoCapitalize="none"
                    spellCheck={false}
                    className="font-mono tracking-wider"
                    required
                    autoFocus
                    hint={t('auth.verify.recoveryDescription')}
                    error={f.errorFor('recovery_code')}
                />
            ) : (
                <OtpInput
                    key="code"
                    name="code"
                    label={t('auth.verify.authenticatorCode')}
                    autoFocus
                    error={f.errorFor('code')}
                />
            )}
            <AuthSubmit>{t('auth.verify.submit')}</AuthSubmit>
            <div className="flex flex-col items-center gap-1">
                <Button
                    type="button"
                    variant="link"
                    className="h-11 text-sm"
                    onClick={() => setRecovery((v) => !v)}
                >
                    <KeyRound aria-hidden />
                    {recovery
                        ? t('auth.verify.useApp')
                        : t('auth.verify.useRecovery')}
                </Button>
            </div>
        </form>
    );
}

export function SetupForm({ token }: { token: string }) {
    const t = useT();
    const r = useCommonRules();
    const f = useAuthForm(setupAction, {
        name: r.required,
        email: r.email,
        password: r.newPassword,
    });
    const [name, setName] = useState('');
    const [email, setEmail] = useState('');
    return (
        <form {...f.formProps} className="flex flex-col gap-5">
            <input type="hidden" name="token" value={token} />
            <ErrorSummary
                title={t('auth.validation.summary', {
                    count: Object.values(f.summaryErrors).filter(Boolean)
                        .length,
                })}
                errors={f.summaryErrors}
                labels={{
                    name: t('auth.yourName'),
                    email: t('auth.email'),
                    password: t('auth.password'),
                }}
                summaryRef={f.summaryRef}
            />
            <ServerAlert state={f.state} />
            <AuthField
                label={t('auth.setup.organizationName')}
                name="organization_name"
                defaultValue={f.values.organization_name}
                placeholder={t('auth.setup.organizationPlaceholder')}
                autoComplete="organization"
                maxLength={120}
                autoFocus
                error={f.errorFor('organization_name')}
            />
            <AuthField
                label={t('auth.yourName')}
                name="name"
                defaultValue={f.values.name}
                autoComplete="name"
                required
                maxLength={120}
                onChange={(e) => setName(e.target.value)}
                error={f.errorFor('name')}
            />
            <AuthField
                label={t('auth.email')}
                name="email"
                defaultValue={f.values.email}
                type="email"
                autoComplete="username email"
                required
                onChange={(e) => setEmail(e.target.value)}
                error={f.errorFor('email')}
            />
            <PasswordField
                label={t('auth.password')}
                name="password"
                autoComplete="new-password"
                required
                meter
                personal={[name, email]}
                hint={t('auth.passwordHint')}
                error={f.errorFor('password')}
            />
            <AuthSubmit>{t('auth.setup.submit')}</AuthSubmit>
        </form>
    );
}

export function AcceptInvitationForm({
    token,
    email,
}: {
    token: string;
    email: string;
}) {
    const t = useT();
    const r = useCommonRules();
    const f = useAuthForm(acceptInvitationAction, {
        name: r.required,
        password: r.newPassword,
    });
    const [name, setName] = useState('');
    return (
        <form {...f.formProps} className="flex flex-col gap-5">
            <input type="hidden" name="token" value={token} />
            <ErrorSummary
                title={t('auth.validation.summary', {
                    count: Object.values(f.summaryErrors).filter(Boolean)
                        .length,
                })}
                errors={f.summaryErrors}
                labels={{
                    name: t('auth.yourName'),
                    password: t('auth.password'),
                }}
                summaryRef={f.summaryRef}
            />
            <ServerAlert state={f.state} />
            <AuthField
                label={t('auth.email')}
                name="email_display"
                value={email}
                // Lets password managers save the new password for it.
                autoComplete="username"
                readOnly
                className="bg-muted text-muted-foreground dark:bg-input/20"
            />
            <AuthField
                label={t('auth.yourName')}
                name="name"
                defaultValue={f.values.name}
                autoComplete="name"
                required
                maxLength={120}
                autoFocus
                onChange={(e) => setName(e.target.value)}
                error={f.errorFor('name')}
            />
            <PasswordField
                label={t('auth.password')}
                name="password"
                autoComplete="new-password"
                required
                meter
                personal={[name, email]}
                hint={t('auth.passwordHint')}
                error={f.errorFor('password')}
            />
            <AuthSubmit>{t('auth.invite.submit')}</AuthSubmit>
        </form>
    );
}

export function ForgotPasswordForm() {
    const t = useT();
    const r = useCommonRules();
    const f = useAuthForm(forgotPasswordAction, { email: r.email });
    const [retry, setRetry] = useState<unknown>(null);
    if (f.state?.ok && retry !== f.state) {
        return (
            <div className="flex flex-col items-start gap-5">
                <FormAlert tone="success">
                    <MailCheck className="mt-0.5 size-4 shrink-0" aria-hidden />
                    <span>
                        <strong className="block font-semibold">
                            {t('auth.forgot.sentTitle')}
                        </strong>
                        {t('auth.forgot.sentBody', {
                            email: f.state.data.email,
                        })}
                    </span>
                </FormAlert>
                <Button
                    type="button"
                    variant="outline"
                    className="h-11 w-full rounded-xl text-[15px]"
                    onClick={() => setRetry(f.state)}
                >
                    {t('auth.forgot.tryAgain')}
                </Button>
            </div>
        );
    }
    return (
        <form {...f.formProps} className="flex flex-col gap-5">
            <ServerAlert state={f.state} />
            <AuthField
                label={t('auth.email')}
                name="email"
                defaultValue={f.values.email}
                type="email"
                inputMode="email"
                autoComplete="username email"
                autoCapitalize="none"
                spellCheck={false}
                required
                autoFocus
                error={f.errorFor('email')}
            />
            <AuthSubmit>{t('auth.forgot.submit')}</AuthSubmit>
        </form>
    );
}

export function ResetPasswordForm({ token }: { token: string }) {
    const t = useT();
    const r = useCommonRules();
    const f = useAuthForm(resetPasswordAction, { password: r.newPassword });
    if (
        f.state &&
        !f.state.ok &&
        f.state.code === AUTH_ERROR_CODES.resetTokenInvalid
    ) {
        return <ResetExpired />;
    }
    return (
        <form {...f.formProps} className="flex flex-col gap-5">
            <input type="hidden" name="token" value={token} />
            <ServerAlert state={f.state} />
            <PasswordField
                label={t('auth.newPassword')}
                name="password"
                autoComplete="new-password"
                required
                autoFocus
                meter
                hint={t('auth.passwordHint')}
                error={f.errorFor('password')}
            />
            <AuthSubmit>{t('auth.reset.submit')}</AuthSubmit>
        </form>
    );
}

function ResetExpired() {
    const t = useT();
    return (
        <div className="flex flex-col gap-5" role="alert">
            <FormAlert tone="error">
                <AlertCircle className="mt-0.5 size-4 shrink-0" aria-hidden />
                <span>
                    <strong className="block font-semibold">
                        {t('auth.reset.expiredTitle')}
                    </strong>
                    {t('auth.reset.expiredBody')}
                </span>
            </FormAlert>
            <Button
                asChild
                className="h-11 w-full rounded-xl text-[15px] font-semibold"
            >
                <Link href="/forgot-password">
                    {t('auth.reset.requestNew')}
                </Link>
            </Button>
        </div>
    );
}

/** Live check of a workspace address against the API (debounced). */
function useSlugAvailability(slug: string) {
    const [result, setResult] = useState<{
        slug: string;
        available: boolean | null;
        reason?: 'taken' | 'reserved';
    } | null>(null);
    useEffect(() => {
        if (validateSlug(slug)) return;
        let live = true;
        const id = setTimeout(() => {
            checkSlugAction(slug).then((r) => {
                if (live) setResult({ slug, ...r });
            });
        }, 400);
        return () => {
            live = false;
            clearTimeout(id);
        };
    }, [slug]);
    if (!result || result.slug !== slug || validateSlug(slug)) return null;
    return result;
}

export function SignupForm({
    baseDomain,
    contactUrl,
}: {
    baseDomain: string;
    contactUrl: string;
}) {
    const t = useT();
    const r = useCommonRules();
    const f = useAuthForm(signupAction, {
        name: r.required,
        email: r.email,
        password: r.newPassword,
        company_name: r.required,
        workspace_slug: (v) => {
            const e = validateSlug(v);
            return e ? t(`auth.slug.${e}`) : undefined;
        },
        accept_terms: (_v, form) =>
            form.get('accept_terms') === 'on'
                ? undefined
                : t('auth.validation.terms'),
    });
    const [name, setName] = useState('');
    const [email, setEmail] = useState('');
    const [company, setCompany] = useState(f.values.company_name ?? '');
    const [slugEdited, setSlugEdited] = useState(!!f.values.workspace_slug);
    const [slugInput, setSlugInput] = useState(f.values.workspace_slug ?? '');
    const slug = slugEdited ? slugInput : slugify(company);
    const slugError = slug ? validateSlug(slug) : null;
    const availability = useSlugAvailability(slug);

    if (
        f.state &&
        !f.state.ok &&
        f.state.code === AUTH_ERROR_CODES.signupUnavailable
    ) {
        return (
            <div className="flex flex-col gap-5">
                <FormAlert tone="info">
                    <ShieldCheck
                        className="mt-0.5 size-4 shrink-0 text-primary"
                        aria-hidden
                    />
                    <span>
                        <strong className="block font-semibold">
                            {t('auth.signup.soonTitle')}
                        </strong>
                        {t('auth.signup.soonBody')}
                    </span>
                </FormAlert>
                <Button
                    asChild
                    className="h-11 w-full rounded-xl text-[15px] font-semibold"
                >
                    <a href={contactUrl}>{t('auth.signup.soonContact')}</a>
                </Button>
                <Button
                    asChild
                    variant="outline"
                    className="h-11 w-full rounded-xl text-[15px]"
                >
                    <Link href="/login">
                        <ArrowLeft aria-hidden />
                        {t('auth.signup.soonBack')}
                    </Link>
                </Button>
            </div>
        );
    }

    const slugMessage = f.errorFor('workspace_slug');
    const termsError = f.errorFor('accept_terms');
    const labels = {
        name: t('auth.signup.name'),
        email: t('auth.signup.email'),
        password: t('auth.signup.password'),
        company_name: t('auth.signup.company'),
        workspace_slug: t('auth.signup.slug'),
        accept_terms: t('auth.signup.termsLink'),
    };
    const [termsBefore, termsMid, termsAfter] = splitTerms(
        t('auth.signup.terms'),
    );

    return (
        <form {...f.formProps} className="flex flex-col gap-5">
            <ErrorSummary
                title={t('auth.validation.summary', {
                    count: Object.values(f.summaryErrors).filter(Boolean)
                        .length,
                })}
                errors={f.summaryErrors}
                labels={labels}
                summaryRef={f.summaryRef}
            />
            <ServerAlert state={f.state} />
            <AuthField
                label={labels.name}
                name="name"
                defaultValue={f.values.name}
                autoComplete="name"
                required
                maxLength={120}
                autoFocus
                onChange={(e) => setName(e.target.value)}
                error={f.errorFor('name')}
            />
            <AuthField
                label={labels.email}
                name="email"
                defaultValue={f.values.email}
                type="email"
                inputMode="email"
                autoComplete="username email"
                autoCapitalize="none"
                spellCheck={false}
                required
                hint={t('auth.signup.emailHint')}
                onChange={(e) => setEmail(e.target.value)}
                error={f.errorFor('email')}
            />
            <PasswordField
                label={labels.password}
                name="password"
                autoComplete="new-password"
                required
                meter
                personal={[name, email, company]}
                hint={t('auth.passwordHint')}
                error={f.errorFor('password')}
            />
            <AuthField
                label={labels.company_name}
                name="company_name"
                value={company}
                onChange={(e) => setCompany(e.target.value)}
                placeholder={t('auth.signup.companyPlaceholder')}
                autoComplete="organization"
                required
                maxLength={120}
                error={f.errorFor('company_name')}
            />
            <AuthField
                label={labels.workspace_slug}
                name="workspace_slug"
                value={slug}
                onChange={(e) => {
                    setSlugEdited(true);
                    setSlugInput(sanitizeSlugInput(e.target.value));
                }}
                autoComplete="off"
                autoCapitalize="none"
                spellCheck={false}
                inputMode="url"
                maxLength={30}
                required
                className="pr-32 font-medium"
                trailing={
                    <span
                        aria-hidden
                        className="pointer-events-none absolute inset-y-0 right-3.5 flex items-center text-[15px] text-muted-foreground"
                    >
                        .{baseDomain}
                    </span>
                }
                error={slugMessage}
                hint={
                    <span className="flex flex-col gap-1">
                        <span
                            aria-live="polite"
                            className="flex flex-wrap items-center gap-x-1.5"
                        >
                            <span>{t('auth.signup.preview')}</span>
                            <span
                                className={cn(
                                    'rounded-md bg-muted px-1.5 py-0.5 font-mono text-[13px] text-foreground',
                                    !slug && 'text-muted-foreground',
                                )}
                            >
                                {(slug || 'firma') + '.' + baseDomain}
                            </span>
                            {!slugMessage &&
                                slug &&
                                !slugError &&
                                availability && (
                                    <SlugBadge
                                        available={availability.available}
                                        reason={availability.reason}
                                    />
                                )}
                            {!slugMessage && slugError && slug.length >= 3 && (
                                <span className="text-destructive">
                                    {t(`auth.slug.${slugError}`)}
                                </span>
                            )}
                        </span>
                        <span>{t('auth.signup.slugHint')}</span>
                    </span>
                }
            />
            <div className="flex flex-col gap-1.5">
                <div className="flex items-start gap-3">
                    <Checkbox
                        id="field-accept_terms"
                        name="accept_terms"
                        aria-invalid={termsError ? true : undefined}
                        aria-describedby={
                            termsError ? 'field-accept_terms-error' : undefined
                        }
                        defaultChecked={f.values.accept_terms === 'on'}
                        className="mt-0.5 size-[18px] rounded-[5px]"
                    />
                    <Label
                        htmlFor="field-accept_terms"
                        className="block text-sm leading-relaxed font-normal text-muted-foreground"
                    >
                        {termsBefore}
                        <a
                            href={LEGAL_LINKS.terms}
                            target="_blank"
                            rel="noreferrer"
                            className="font-medium text-primary underline underline-offset-2"
                        >
                            {t('auth.signup.termsLink')}
                        </a>
                        {termsMid}
                        <a
                            href={LEGAL_LINKS.privacy}
                            target="_blank"
                            rel="noreferrer"
                            className="font-medium text-primary underline underline-offset-2"
                        >
                            {t('auth.signup.privacyLink')}
                        </a>
                        {termsAfter}
                    </Label>
                </div>
                {termsError && (
                    <p
                        id="field-accept_terms-error"
                        className="flex items-center gap-1.5 pl-[30px] text-sm text-destructive"
                    >
                        {termsError}
                    </p>
                )}
            </div>
            <AuthSubmit>{t('auth.signup.submit')}</AuthSubmit>
        </form>
    );
}

/** "I accept the {terms} and the {privacy}." → text around the two links. */
function splitTerms(s: string): [string, string, string] {
    const a = s.indexOf('{terms}');
    const b = s.indexOf('{privacy}');
    if (a < 0 || b < 0 || b < a) return [s + ' ', ' ', ''];
    return [s.slice(0, a), s.slice(a + 7, b), s.slice(b + 9)];
}

function SlugBadge({
    available,
    reason,
}: {
    available: boolean | null;
    reason?: 'taken' | 'reserved';
}) {
    const t = useT();
    if (available === null) return null;
    return available ? (
        <span className="inline-flex items-center gap-1 font-medium text-emerald-700 dark:text-mint">
            <CheckCircle2 className="size-3.5" aria-hidden />
            {t('auth.slug.available')}
        </span>
    ) : (
        <span className="inline-flex items-center gap-1 font-medium text-destructive">
            <AlertCircle className="size-3.5" aria-hidden />
            {reason === 'reserved'
                ? t('auth.slug.reserved')
                : t('auth.slug.taken')}
        </span>
    );
}

/** Seconds left until `state` allows another send. */
function useCooldown(state: ResendState | null, initial: number) {
    const [now, setNow] = useState(() => Date.now());
    const [start] = useState(() => Date.now());
    const until = state
        ? state.at + state.cooldown * 1000
        : start + initial * 1000;
    const left = Math.max(0, Math.ceil((until - now) / 1000));
    useEffect(() => {
        if (left <= 0) return;
        const id = setInterval(() => setNow(Date.now()), 1000);
        return () => clearInterval(id);
    }, [left]);
    return left;
}

export function ResendVerificationForm({
    email,
    initialCooldown = 30,
}: {
    email: string | null;
    initialCooldown?: number;
}) {
    const t = useT();
    const [state, action] = useActionState(resendVerificationAction, null);
    const left = useCooldown(state, email ? initialCooldown : 0);
    if (state?.status === 'unavailable') {
        return (
            <FormAlert tone="info">
                <ShieldCheck
                    className="mt-0.5 size-4 shrink-0 text-primary"
                    aria-hidden
                />
                <span>{t('auth.verifyEmail.soonBody')}</span>
            </FormAlert>
        );
    }
    return (
        <form action={action} className="flex flex-col gap-4">
            {state?.status === 'sent' && (
                <FormAlert tone="success">
                    <CheckCircle2
                        className="mt-0.5 size-4 shrink-0"
                        aria-hidden
                    />
                    <span>{t('auth.verifyEmail.resent')}</span>
                </FormAlert>
            )}
            {state?.status === 'error' && state.message && (
                <FormAlert>
                    <AlertCircle
                        className="mt-0.5 size-4 shrink-0"
                        aria-hidden
                    />
                    <span>{state.message}</span>
                </FormAlert>
            )}
            {email ? (
                <input type="hidden" name="email" value={email} />
            ) : (
                <AuthField
                    label={t('auth.verifyEmail.emailLabel')}
                    name="email"
                    type="email"
                    inputMode="email"
                    autoComplete="email"
                    required
                />
            )}
            <ResendButton left={left} />
        </form>
    );
}

function ResendButton({ left }: { left: number }) {
    const t = useT();
    return (
        <div className="flex flex-col gap-1">
            <AuthSubmitWhen disabled={left > 0} variant="outline">
                {left > 0
                    ? t('auth.verifyEmail.resendIn', { seconds: left })
                    : t('auth.verifyEmail.resend')}
            </AuthSubmitWhen>
        </div>
    );
}

/** AuthSubmit that can also be held back (a cooldown). */
function AuthSubmitWhen({
    disabled,
    variant,
    children,
}: {
    disabled: boolean;
    variant?: React.ComponentProps<typeof Button>['variant'];
    children: React.ReactNode;
}) {
    if (!disabled) return <AuthSubmit variant={variant}>{children}</AuthSubmit>;
    return (
        <Button
            type="submit"
            variant={variant}
            disabled
            className="h-11 w-full rounded-xl text-[15px] font-semibold tabular-nums"
        >
            {children}
        </Button>
    );
}

/**
 * Confirms the address from the e-mailed link. It runs on its own once
 * the page has loaded (link scanners that only fetch the page do not use
 * the token up); without JavaScript the button does the same.
 */
export function VerifyEmail({ token }: { token: string }) {
    const t = useT();
    const [state, action, pending] = useActionState(verifyEmailAction, null);
    const ran = useRef(false);
    useEffect(() => {
        if (ran.current || !token) return;
        ran.current = true;
        const data = new FormData();
        data.set('token', token);
        startTransition(() => action(data));
    }, [action, token]);

    if (!state) {
        return (
            <form
                action={action}
                className="flex flex-col items-center gap-4 py-2"
            >
                <input type="hidden" name="token" value={token} />
                <Spinner className="size-6 text-primary" />
                <noscript>
                    <Button type="submit" className="h-11 w-full rounded-xl">
                        {t('auth.verifyEmail.metaTitle')}
                    </Button>
                </noscript>
                {pending && <span className="sr-only">…</span>}
            </form>
        );
    }
    return <VerifyEmailResult state={state} />;
}

function VerifyEmailResult({
    state,
}: {
    state: NonNullable<Parameters<typeof verifyEmailAction>[0]>;
}) {
    const t = useT();
    if (state.status === 'verified') {
        return (
            <div className="flex flex-col gap-5">
                <FormAlert tone="success">
                    <CheckCircle2
                        className="mt-0.5 size-4 shrink-0"
                        aria-hidden
                    />
                    <span>
                        <strong className="block font-semibold">
                            {t('auth.verifyEmail.successTitle')}
                        </strong>
                        {state.workspace
                            ? t('auth.verifyEmail.successBody', {
                                  workspace: state.workspace,
                              })
                            : t('auth.verifyEmail.successBodyGeneric')}
                    </span>
                </FormAlert>
                <Button
                    asChild
                    className="h-11 w-full rounded-xl text-[15px] font-semibold"
                >
                    <a href={state.loginUrl}>
                        {t('auth.verifyEmail.continue')}
                    </a>
                </Button>
            </div>
        );
    }
    if (state.status === 'already') {
        return (
            <div className="flex flex-col gap-5">
                <FormAlert tone="info">
                    <CheckCircle2
                        className="mt-0.5 size-4 shrink-0 text-primary"
                        aria-hidden
                    />
                    <span>
                        <strong className="block font-semibold">
                            {t('auth.verifyEmail.alreadyTitle')}
                        </strong>
                        {t('auth.verifyEmail.alreadyBody')}
                    </span>
                </FormAlert>
                <Button
                    asChild
                    className="h-11 w-full rounded-xl text-[15px] font-semibold"
                >
                    <Link href="/login">{t('auth.verifyEmail.continue')}</Link>
                </Button>
            </div>
        );
    }
    if (state.status === 'unavailable') {
        return (
            <FormAlert tone="info">
                <ShieldCheck
                    className="mt-0.5 size-4 shrink-0 text-primary"
                    aria-hidden
                />
                <span>
                    <strong className="block font-semibold">
                        {t('auth.verifyEmail.soonTitle')}
                    </strong>
                    {t('auth.verifyEmail.soonBody')}
                </span>
            </FormAlert>
        );
    }
    const expired = state.status === 'expired';
    return (
        <div className="flex flex-col gap-5">
            <FormAlert>
                <AlertCircle className="mt-0.5 size-4 shrink-0" aria-hidden />
                <span>
                    <strong className="block font-semibold">
                        {expired
                            ? t('auth.verifyEmail.expiredTitle')
                            : t('auth.verifyEmail.invalidTitle')}
                    </strong>
                    {state.status === 'error' && state.message
                        ? state.message
                        : expired
                          ? t('auth.verifyEmail.expiredBody')
                          : t('auth.verifyEmail.invalidBody')}
                </span>
            </FormAlert>
            <ResendVerificationForm email={null} initialCooldown={0} />
        </div>
    );
}
