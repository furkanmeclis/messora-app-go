'use client';

import { useActionState, useRef } from 'react';
import {
    FormStatus,
    SubmitButton,
    TextField,
    fieldError,
} from '@/components/forms';
import { FieldGroup } from '@/components/ui/field';
import { useT } from '@/i18n/client';
import type { ActionResult } from '@/lib/types';
import { changePasswordAction } from '../../actions';

export function PasswordForm() {
    const t = useT();
    const ref = useRef<HTMLFormElement>(null);
    const [state, action] = useActionState<ActionResult | null, FormData>(
        async (prev, form) => {
            const r = await changePasswordAction(prev, form);
            if (r.ok) ref.current?.reset();
            return r;
        },
        null,
    );
    return (
        <form ref={ref} action={action} className="max-w-md space-y-4">
            <FormStatus state={state} />
            <FieldGroup>
                <TextField
                    label={t('account.password.current')}
                    name="current_password"
                    type="password"
                    autoComplete="current-password"
                    required
                    error={fieldError(state, 'current_password')}
                />
                <TextField
                    label={t('account.password.new')}
                    name="new_password"
                    type="password"
                    autoComplete="new-password"
                    required
                    minLength={12}
                    hint={t('account.password.newHint')}
                    error={fieldError(state, 'new_password')}
                />
            </FieldGroup>
            <SubmitButton>{t('account.password.submit')}</SubmitButton>
        </form>
    );
}
