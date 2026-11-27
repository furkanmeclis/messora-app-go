'use client';

import { useActionState, useRef, useState } from 'react';
import type { ActionResult } from '@/lib/types';

/** Fields never echoed back into the form after a failed submission. */
const SECRET_FIELDS = new Set(['password', 'code', 'recovery_code', 'token']);

export type FormState<T = undefined> = ActionResult<T> & {
    values?: Record<string, string>;
};

type Action<T> = (
    prev: ActionResult<T> | null,
    form: FormData,
) => Promise<ActionResult<T>>;

/** A check for one field: an error message, or nothing when it is fine. */
export type Rule = (value: string, form: FormData) => string | undefined;

/**
 * Wires a sign-in form to its server action with inline validation.
 *
 * - Fields are checked when they lose focus, and again on every change once
 *   they show an error, so the message goes away as soon as it is fixed.
 * - Submitting checks everything first; with problems the action does not
 *   run and the error summary takes focus.
 * - React resets a form after its action runs; the submitted non-secret
 *   values come back in `state.values` for `defaultValue`.
 * - Server field errors hide once the person edits that field.
 */
export function useAuthForm<T = undefined>(
    action: Action<T>,
    rules: Record<string, Rule> = {},
) {
    const [state, formAction] = useActionState<FormState<T> | null, FormData>(
        async (prev, form) => {
            const result = await action(prev, form);
            const values: Record<string, string> = {};
            form.forEach((v, k) => {
                if (typeof v === 'string' && !SECRET_FIELDS.has(k)) {
                    values[k] = v;
                }
            });
            return { ...result, values };
        },
        null,
    );
    const [client, setClient] = useState<Record<string, string | undefined>>(
        {},
    );
    // Fields edited since the server answered (keyed by that answer).
    const [edited, setEdited] = useState<{
        state: unknown;
        names: Set<string>;
    }>({ state: null, names: new Set() });
    const summaryRef = useRef<HTMLDivElement>(null);
    const [showSummary, setShowSummary] = useState(false);

    const server = state && !state.ok ? (state.fieldErrors ?? {}) : {};
    const editedNames =
        edited.state === state ? edited.names : new Set<string>();

    const check = (form: HTMLFormElement, name: string) => {
        const rule = rules[name];
        if (!rule) return undefined;
        const data = new FormData(form);
        const v = data.get(name);
        return rule(typeof v === 'string' ? v.trim() : '', data);
    };

    const errorFor = (name: string): string | undefined =>
        client[name] ?? (editedNames.has(name) ? undefined : server[name]);

    const formProps = {
        action: formAction,
        noValidate: true,
        onBlur: (e: React.FocusEvent<HTMLFormElement>) => {
            const name = (e.target as unknown as HTMLInputElement).name;
            if (!name || !rules[name]) return;
            // Leaving an untouched empty field is not an error yet.
            const v = (e.target as unknown as HTMLInputElement).value;
            if (!v && !client[name]) return;
            // Heading for the submit button: the submit checks everything,
            // and a message appearing now would move the button away from
            // the pointer mid-click.
            const next = e.relatedTarget as HTMLElement | null;
            if (next?.getAttribute('type') === 'submit') return;
            // Checked now: React clears currentTarget after the handler.
            const error = check(e.currentTarget, name);
            setClient((c) => ({ ...c, [name]: error }));
        },
        onChange: (e: React.FormEvent<HTMLFormElement>) => {
            const name = (e.target as unknown as HTMLInputElement).name;
            if (!name) return;
            if (server[name] && !editedNames.has(name)) {
                setEdited({
                    state,
                    names: new Set([...editedNames, name]),
                });
            }
            if (client[name]) {
                const error = check(e.currentTarget, name);
                setClient((c) => ({ ...c, [name]: error }));
            }
        },
        onSubmit: (e: React.FormEvent<HTMLFormElement>) => {
            const all: Record<string, string | undefined> = {};
            for (const name of Object.keys(rules)) {
                all[name] = check(e.currentTarget, name);
            }
            setClient(all);
            const failed = Object.keys(all).filter((k) => all[k]);
            if (failed.length > 0) {
                e.preventDefault();
                setShowSummary(true);
                requestAnimationFrame(() => summaryRef.current?.focus());
            } else {
                setShowSummary(false);
            }
        },
    };

    /** Errors for the summary: after a blocked submit, or from the server. */
    const summaryErrors: Record<string, string | undefined> = {};
    if (showSummary) {
        for (const k of Object.keys(rules)) summaryErrors[k] = client[k];
    } else if (state && !state.ok) {
        for (const k of Object.keys(server)) summaryErrors[k] = errorFor(k);
    }

    return {
        state,
        formProps,
        errorFor,
        summaryRef,
        summaryErrors,
        values: state?.values ?? {},
    };
}

/** Common rules. */
export const rules = {
    required:
        (message: string): Rule =>
        (v) =>
            v ? undefined : message,
    email:
        (required: string, invalid: string): Rule =>
        (v) =>
            !v
                ? required
                : /^[^\s@]+@[^\s@]+\.[^\s@]{2,}$/.test(v)
                  ? undefined
                  : invalid,
    minLength:
        (min: number, required: string, short: string): Rule =>
        (v) =>
            !v ? required : [...v].length < min ? short : undefined,
};
