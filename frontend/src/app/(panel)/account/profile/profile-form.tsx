'use client';

import { useActionState } from 'react';
import {
    FormStatus,
    SubmitButton,
    TextField,
    fieldError,
} from '@/components/forms';
import { TimezoneSelect } from '@/components/timezone-select';
import { Field, FieldGroup, FieldLabel } from '@/components/ui/field';
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from '@/components/ui/select';
import { useT } from '@/i18n/client';
import { localeNames, locales } from '@/i18n/config';
import type { ActionResult, User } from '@/lib/types';
import { updateProfileAction } from '../../actions';

export function ProfileForm({ user }: { user: User }) {
    const [state, action] = useActionState<ActionResult | null, FormData>(
        updateProfileAction,
        null,
    );
    const t = useT();
    return (
        <form action={action} className="space-y-4">
            <FormStatus state={state} />
            <FieldGroup>
                <TextField
                    label={t('account.profile.name')}
                    name="name"
                    defaultValue={user.name}
                    required
                    maxLength={120}
                    autoComplete="name"
                    error={fieldError(state, 'name')}
                />
                <TimezoneSelect
                    name="timezone"
                    label={t('account.profile.timezone')}
                    defaultValue={user.timezone}
                />
                <Field>
                    <FieldLabel htmlFor="field-locale">
                        {t('account.profile.language')}
                    </FieldLabel>
                    <Select name="locale" defaultValue={user.locale}>
                        <SelectTrigger id="field-locale" className="w-full">
                            <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                            {locales.map((l) => (
                                <SelectItem key={l} value={l}>
                                    {localeNames[l]}
                                </SelectItem>
                            ))}
                        </SelectContent>
                    </Select>
                </Field>
            </FieldGroup>
            <SubmitButton>{t('common.save')}</SubmitButton>
        </form>
    );
}
