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
import type { ActionResult, User } from '@/lib/types';
import { updateProfileAction } from '../../actions';

export function ProfileForm({ user }: { user: User }) {
    const [state, action] = useActionState<ActionResult | null, FormData>(
        updateProfileAction,
        null,
    );
    return (
        <form action={action} className="space-y-4">
            <FormStatus state={state} />
            <FieldGroup>
                <TextField
                    label="Name"
                    name="name"
                    defaultValue={user.name}
                    required
                    maxLength={120}
                    autoComplete="name"
                    error={fieldError(state, 'name')}
                />
                <TimezoneSelect
                    name="timezone"
                    label="Time zone"
                    defaultValue={user.timezone}
                />
                <Field>
                    <FieldLabel htmlFor="field-locale">Language</FieldLabel>
                    <Select name="locale" defaultValue={user.locale}>
                        <SelectTrigger id="field-locale" className="w-full">
                            <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                            <SelectItem value="en">English</SelectItem>
                            <SelectItem value="tr">Türkçe</SelectItem>
                        </SelectContent>
                    </Select>
                </Field>
            </FieldGroup>
            <SubmitButton>Save</SubmitButton>
        </form>
    );
}
