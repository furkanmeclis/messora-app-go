'use client';

import { useFormStatus } from 'react-dom';
import { AlertCircle, CheckCircle2 } from 'lucide-react';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import { Field, FieldDescription, FieldLabel } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { Spinner } from '@/components/ui/spinner';
import type { ActionResult } from '@/lib/types';

type TextFieldProps = React.ComponentProps<typeof Input> & {
    label: string;
    name: string;
    error?: string;
    hint?: string;
};

/** Labelled input with an inline error from the API's field details. */
export function TextField({
    label,
    name,
    error,
    hint,
    id,
    ...props
}: TextFieldProps) {
    const inputId = id ?? `field-${name}`;
    return (
        <Field data-invalid={error ? true : undefined}>
            <FieldLabel htmlFor={inputId}>{label}</FieldLabel>
            <Input
                id={inputId}
                name={name}
                aria-invalid={error ? true : undefined}
                aria-describedby={error ? `${inputId}-error` : undefined}
                {...props}
            />
            {hint && !error && <FieldDescription>{hint}</FieldDescription>}
            {error && (
                <p
                    id={`${inputId}-error`}
                    role="alert"
                    className="text-sm text-destructive"
                >
                    {error}
                </p>
            )}
        </Field>
    );
}

/** Submit button that shows progress while its form's action runs. */
export function SubmitButton({
    children,
    className,
    variant,
    size,
}: {
    children: React.ReactNode;
    className?: string;
    variant?: React.ComponentProps<typeof Button>['variant'];
    size?: React.ComponentProps<typeof Button>['size'];
}) {
    const { pending } = useFormStatus();
    return (
        <Button
            type="submit"
            disabled={pending}
            className={className}
            variant={variant}
            size={size}
        >
            {pending && <Spinner />}
            {children}
        </Button>
    );
}

/** Banner for an action's overall result. */
export function FormStatus({ state }: { state: ActionResult<unknown> | null }) {
    if (!state) return null;
    if (state.ok) {
        if (!state.message) return null;
        return (
            <Alert>
                <CheckCircle2 />
                <AlertDescription>{state.message}</AlertDescription>
            </Alert>
        );
    }
    const onlyFieldErrors =
        state.code === 'VALIDATION_ERROR' &&
        state.fieldErrors &&
        Object.keys(state.fieldErrors).length > 0;
    if (onlyFieldErrors) return null;
    return (
        <Alert variant="destructive">
            <AlertCircle />
            <AlertDescription>{state.message}</AlertDescription>
        </Alert>
    );
}

export function fieldError(
    state: ActionResult<unknown> | null,
    name: string,
): string | undefined {
    return state && !state.ok ? state.fieldErrors?.[name] : undefined;
}
