'use client';

import { useActionState, useState } from 'react';
import Link from 'next/link';
import { AlertCircle, CheckCircle2, KeyRound } from 'lucide-react';
import { Button } from '@/components/ui/button';
import {
    AuthField,
    AuthSubmit,
    FormAlert,
    OtpInput,
    PasswordField,
} from '@/app/(auth)/_components/fields';
import { rules, useAuthForm } from '@/app/(auth)/_components/use-auth-form';
import { useT } from '@/i18n/client';
import { PASSWORD_MIN } from '@/lib/auth/password-strength';
import type { ActionResult } from '@/lib/types';
import {
    bootstrapAction,
    confirmEnrollAction,
    platformLoginAction,
    platformVerifyAction,
    startEnrollAction,
} from '../actions';

function ServerError({ state }: { state: ActionResult<unknown> | null }) {
    if (!state || state.ok || !state.message) return null;
    return (
        <FormAlert>
            <AlertCircle className="mt-0.5 size-4 shrink-0" aria-hidden />
            <span>{state.message}</span>
        </FormAlert>
    );
}

export function PlatformLoginForm() {
    const t = useT();
    const f = useAuthForm(platformLoginAction, {
        email: rules.email(
            t('auth.validation.required'),
            t('auth.validation.email'),
        ),
        password: rules.required(t('auth.validation.required')),
    });
    return (
        <form {...f.formProps} className="flex flex-col gap-5">
            <ServerError state={f.state} />
            <AuthField
                label={t('auth.email')}
                name="email"
                type="email"
                defaultValue={f.values.email}
                autoComplete="username email"
                autoCapitalize="none"
                spellCheck={false}
                required
                autoFocus
                error={f.errorFor('email')}
            />
            <PasswordField
                label={t('auth.password')}
                name="password"
                autoComplete="current-password"
                required
                error={f.errorFor('password')}
            />
            <AuthSubmit>{t('platform.login.submit')}</AuthSubmit>
        </form>
    );
}

export function PlatformVerifyForm() {
    const t = useT();
    const [recovery, setRecovery] = useState(false);
    const f = useAuthForm(
        platformVerifyAction,
        recovery
            ? { recovery_code: rules.required(t('auth.validation.required')) }
            : {
                  code: (v) =>
                      /^\d{6}$/.test(v) ? undefined : t('auth.validation.code'),
              },
    );
    return (
        <form {...f.formProps} className="flex flex-col gap-5">
            <ServerError state={f.state} />
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
            <AuthSubmit>{t('platform.login.verifySubmit')}</AuthSubmit>
            <Button
                type="button"
                variant="link"
                className="h-11 text-sm"
                onClick={() => setRecovery((v) => !v)}
            >
                <KeyRound aria-hidden />
                {recovery
                    ? t('platform.login.useApp')
                    : t('platform.login.useRecovery')}
            </Button>
        </form>
    );
}

export function BootstrapForm({ token }: { token: string }) {
    const t = useT();
    const f = useAuthForm(bootstrapAction, {
        name: rules.required(t('auth.validation.required')),
        email: rules.email(
            t('auth.validation.required'),
            t('auth.validation.email'),
        ),
        password: rules.minLength(
            PASSWORD_MIN,
            t('auth.validation.required'),
            t('auth.validation.passwordMin'),
        ),
    });
    return (
        <form {...f.formProps} className="flex flex-col gap-5">
            <input type="hidden" name="token" value={token} />
            <ServerError state={f.state} />
            <AuthField
                label={t('auth.yourName')}
                name="name"
                defaultValue={f.values.name}
                autoComplete="name"
                required
                autoFocus
                error={f.errorFor('name')}
            />
            <AuthField
                label={t('auth.email')}
                name="email"
                type="email"
                defaultValue={f.values.email}
                autoComplete="username email"
                required
                error={f.errorFor('email')}
            />
            <PasswordField
                label={t('auth.password')}
                name="password"
                autoComplete="new-password"
                required
                meter
                hint={t('auth.passwordHint')}
                error={f.errorFor('password')}
            />
            <AuthSubmit>{t('platform.bootstrap.submit')}</AuthSubmit>
        </form>
    );
}

/** Mandatory TOTP enrolment of a platform admin. */
export function EnrollForm() {
    const t = useT();
    const [setup, setSetup] = useState<{ secret: string; qr: string } | null>(
        null,
    );
    const [startError, setStartError] = useState<string | null>(null);
    const [starting, setStarting] = useState(false);
    const [state, action] = useActionState(confirmEnrollAction, null);

    if (state?.ok) {
        return (
            <div className="flex flex-col gap-5">
                <FormAlert tone="success">
                    <CheckCircle2
                        className="mt-0.5 size-4 shrink-0"
                        aria-hidden
                    />
                    <span>
                        <strong className="block font-semibold">
                            {t('platform.enroll.codesTitle')}
                        </strong>
                        {t('platform.enroll.codesBody')}
                    </span>
                </FormAlert>
                <ul
                    data-testid="recovery-codes"
                    className="grid grid-cols-2 gap-2 rounded-xl border bg-card p-4 font-mono text-sm"
                >
                    {state.data.map((c) => (
                        <li key={c}>{c}</li>
                    ))}
                </ul>
                <Button asChild className="h-11 w-full rounded-xl">
                    <Link href="/">{t('platform.enroll.continue')}</Link>
                </Button>
            </div>
        );
    }
    if (!setup) {
        return (
            <div className="flex flex-col gap-4">
                {startError && (
                    <FormAlert>
                        <AlertCircle
                            className="mt-0.5 size-4 shrink-0"
                            aria-hidden
                        />
                        <span>{startError}</span>
                    </FormAlert>
                )}
                <Button
                    className="h-11 w-full rounded-xl"
                    disabled={starting}
                    onClick={async () => {
                        setStarting(true);
                        const r = await startEnrollAction();
                        setStarting(false);
                        if (r.ok) setSetup(r.data);
                        else setStartError(r.message);
                    }}
                >
                    {t('platform.enroll.start')}
                </Button>
            </div>
        );
    }
    return (
        <form action={action} className="flex flex-col gap-5">
            <div className="flex flex-col items-center gap-3">
                {/* A data: URL; next/image adds nothing here. */}
                {/* eslint-disable-next-line @next/next/no-img-element */}
                <img
                    src={setup.qr}
                    alt={t('account.twoFactor.qrAlt')}
                    width={220}
                    height={220}
                    className="rounded-lg border bg-white p-2"
                />
                <p
                    className="text-center text-sm break-all text-muted-foreground"
                    data-testid="totp-secret"
                    data-secret={setup.secret}
                >
                    {t('platform.enroll.secret', { secret: setup.secret })}
                </p>
            </div>
            <ServerError state={state} />
            <OtpInput name="code" label={t('platform.enroll.code')} autoFocus />
            <AuthSubmit>{t('platform.enroll.confirm')}</AuthSubmit>
        </form>
    );
}
