'use client';

import { useActionState } from 'react';
import {
    FormStatus,
    SubmitButton,
    TextField,
    fieldError,
} from '@/components/forms';
import { TimezoneSelect } from '@/components/timezone-select';
import { Checkbox } from '@/components/ui/checkbox';
import { FieldGroup } from '@/components/ui/field';
import { Label } from '@/components/ui/label';
import type { ActionResult, Settings } from '@/lib/types';
import { updateSettingsAction } from '../actions';

const ROLES = [
    { key: 'owner', name: 'Owner' },
    { key: 'admin', name: 'Admin' },
    { key: 'manager', name: 'Manager' },
    { key: 'agent', name: 'Agent' },
] as const;

export function SettingsForm({
    settings,
    readOnly,
}: {
    settings: Settings;
    readOnly: boolean;
}) {
    const [state, action] = useActionState<ActionResult | null, FormData>(
        updateSettingsAction,
        null,
    );
    return (
        <form action={action} className="space-y-6">
            <FormStatus state={state} />
            <fieldset disabled={readOnly} className="space-y-6">
                <FieldGroup>
                    <TextField
                        label="Organization name"
                        name="organization_name"
                        defaultValue={settings.organization_name}
                        required
                        maxLength={120}
                        error={fieldError(state, 'organization_name')}
                    />
                    <TimezoneSelect
                        name="default_timezone"
                        label="Default time zone (for new users and scheduling)"
                        defaultValue={settings.default_timezone}
                    />
                </FieldGroup>
                <div className="space-y-2">
                    <div className="text-sm font-medium">
                        Require two-factor authentication for
                    </div>
                    <div className="flex flex-wrap gap-4">
                        {ROLES.map((r) => (
                            <div
                                key={r.key}
                                className="flex items-center gap-2"
                            >
                                <Checkbox
                                    id={`mfa-${r.key}`}
                                    name="mfa_required_roles"
                                    value={r.key}
                                    defaultChecked={settings.mfa_required_roles.includes(
                                        r.key,
                                    )}
                                />
                                <Label
                                    htmlFor={`mfa-${r.key}`}
                                    className="font-normal"
                                >
                                    {r.name}
                                </Label>
                            </div>
                        ))}
                    </div>
                </div>
                <FieldGroup className="grid gap-4 sm:grid-cols-2">
                    <TextField
                        label="Sign out after inactivity (hours)"
                        name="session_idle_hours"
                        type="number"
                        min={1}
                        max={720}
                        defaultValue={settings.session_idle_hours}
                        error={fieldError(state, 'session_idle_hours')}
                    />
                    <TextField
                        label="Maximum session length (days)"
                        name="session_lifetime_days"
                        type="number"
                        min={1}
                        max={90}
                        defaultValue={settings.session_lifetime_days}
                        error={fieldError(state, 'session_lifetime_days')}
                    />
                </FieldGroup>
                {!readOnly && <SubmitButton>Save settings</SubmitButton>}
            </fieldset>
        </form>
    );
}
