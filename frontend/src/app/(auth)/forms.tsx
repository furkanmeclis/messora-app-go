'use client';

import Link from 'next/link';
import { useActionState, useState } from 'react';
import {
    FormStatus,
    SubmitButton,
    TextField,
    fieldError,
} from '@/components/forms';
import { Button } from '@/components/ui/button';
import { FieldGroup } from '@/components/ui/field';
import type { ActionResult } from '@/lib/types';
import {
    acceptInvitationAction,
    forgotPasswordAction,
    loginAction,
    resetPasswordAction,
    setupAction,
    verifyMfaAction,
} from './actions';

type Action = (
    prev: ActionResult | null,
    form: FormData,
) => Promise<ActionResult>;

/** Fields never echoed back into the form after a failed submission. */
const SECRET_FIELDS = new Set(['password', 'code', 'token']);

type FormState = ActionResult & { values?: Record<string, string> };

/**
 * React resets a form after its action runs. Keeping the submitted
 * non-secret values lets fields show what the user typed when the server
 * rejects the submission (for example a weak password).
 */
function useAction(action: Action) {
    return useActionState<FormState | null, FormData>(async (prev, form) => {
        const result = await action(prev, form);
        const values: Record<string, string> = {};
        form.forEach((v, k) => {
            if (typeof v === 'string' && !SECRET_FIELDS.has(k)) values[k] = v;
        });
        return { ...result, values };
    }, null);
}

const passwordHint =
    'At least 12 characters. Avoid common passwords and your e-mail address.';

export function LoginForm({
    next,
    canReset,
}: {
    next: string;
    canReset: boolean;
}) {
    const [state, action] = useAction(loginAction);
    return (
        <form action={action} className="space-y-4">
            <input type="hidden" name="next" value={next} />
            <FormStatus state={state} />
            <FieldGroup>
                <TextField
                    label="E-mail"
                    name="email"

                    defaultValue={state?.values?.email}
                    type="email"
                    autoComplete="username"
                    required
                    autoFocus
                    error={fieldError(state, 'email')}
                />
                <TextField
                    label="Password"
                    name="password"
                    type="password"
                    autoComplete="current-password"
                    required
                    error={fieldError(state, 'password')}
                />
            </FieldGroup>
            <SubmitButton className="w-full">Sign in</SubmitButton>
            {canReset && (
                <p className="text-center text-sm">
                    <Link
                        href="/forgot-password"
                        className="text-muted-foreground underline-offset-4 hover:underline"
                    >
                        Forgot your password?
                    </Link>
                </p>
            )}
        </form>
    );
}

export function VerifyForm({ next }: { next: string }) {
    const [state, action] = useAction(verifyMfaAction);
    const [recovery, setRecovery] = useState(false);
    return (
        <form action={action} className="space-y-4">
            <input type="hidden" name="next" value={next} />
            <FormStatus state={state} />
            {recovery ? (
                <TextField
                    key="recovery"
                    label="Recovery code"
                    name="recovery_code"
                    placeholder="xxxxx-xxxxx"
                    autoComplete="one-time-code"
                    required
                    autoFocus
                    error={fieldError(state, 'recovery_code')}
                />
            ) : (
                <TextField
                    key="code"
                    label="Authenticator code"
                    name="code"
                    inputMode="numeric"
                    pattern="\d{6}"
                    maxLength={6}
                    placeholder="123456"
                    autoComplete="one-time-code"
                    required
                    autoFocus
                    error={fieldError(state, 'code')}
                />
            )}
            <SubmitButton className="w-full">Verify</SubmitButton>
            <Button
                type="button"
                variant="link"
                className="w-full"
                onClick={() => setRecovery((v) => !v)}
            >
                {recovery
                    ? 'Use the authenticator app instead'
                    : 'Use a recovery code instead'}
            </Button>
        </form>
    );
}

export function SetupForm({ token }: { token: string }) {
    const [state, action] = useAction(setupAction);
    return (
        <form action={action} className="space-y-4">
            <input type="hidden" name="token" value={token} />
            <FormStatus state={state} />
            <FieldGroup>
                <TextField
                    label="Organization name"
                    name="organization_name"

                    defaultValue={state?.values?.organization_name}
                    placeholder="Acme Inc."
                    maxLength={120}
                    error={fieldError(state, 'organization_name')}
                />
                <TextField
                    label="Your name"
                    name="name"

                    defaultValue={state?.values?.name}
                    autoComplete="name"
                    required
                    maxLength={120}
                    error={fieldError(state, 'name')}
                />
                <TextField
                    label="E-mail"
                    name="email"

                    defaultValue={state?.values?.email}
                    type="email"
                    autoComplete="username"
                    required
                    error={fieldError(state, 'email')}
                />
                <TextField
                    label="Password"
                    name="password"
                    type="password"
                    autoComplete="new-password"
                    required
                    minLength={12}
                    hint={passwordHint}
                    error={fieldError(state, 'password')}
                />
            </FieldGroup>
            <SubmitButton className="w-full">Create owner account</SubmitButton>
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
    const [state, action] = useAction(acceptInvitationAction);
    return (
        <form action={action} className="space-y-4">
            <input type="hidden" name="token" value={token} />
            <FormStatus state={state} />
            <FieldGroup>
                <TextField
                    label="E-mail"
                    name="email_display"
                    value={email}
                    readOnly
                    disabled
                />
                <TextField
                    label="Your name"
                    name="name"

                    defaultValue={state?.values?.name}
                    autoComplete="name"
                    required
                    maxLength={120}
                    error={fieldError(state, 'name')}
                />
                <TextField
                    label="Password"
                    name="password"
                    type="password"
                    autoComplete="new-password"
                    required
                    minLength={12}
                    hint={passwordHint}
                    error={fieldError(state, 'password')}
                />
            </FieldGroup>
            <SubmitButton className="w-full">Join</SubmitButton>
        </form>
    );
}

export function ForgotPasswordForm() {
    const [state, action] = useAction(forgotPasswordAction);
    return (
        <form action={action} className="space-y-4">
            <FormStatus state={state} />
            <TextField
                label="E-mail"
                name="email"

                defaultValue={state?.values?.email}
                type="email"
                autoComplete="username"
                required
                autoFocus
                error={fieldError(state, 'email')}
            />
            <SubmitButton className="w-full">Send reset link</SubmitButton>
        </form>
    );
}

export function ResetPasswordForm({ token }: { token: string }) {
    const [state, action] = useAction(resetPasswordAction);
    return (
        <form action={action} className="space-y-4">
            <input type="hidden" name="token" value={token} />
            <FormStatus state={state} />
            <TextField
                label="New password"
                name="password"
                type="password"
                autoComplete="new-password"
                required
                minLength={12}
                hint={passwordHint}
                error={fieldError(state, 'password')}
            />
            <SubmitButton className="w-full">Set new password</SubmitButton>
        </form>
    );
}
