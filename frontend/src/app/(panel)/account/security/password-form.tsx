'use client';

import { useActionState, useRef, useState } from 'react';
import { Eye, EyeOff, KeyRound, Lock } from 'lucide-react';
import { FormStatus, SubmitButton, fieldError } from '@/components/forms';
import { Input } from '@/components/ui/input';
import { useT } from '@/i18n/client';
import type { ActionResult } from '@/lib/types';
import { cn } from '@/lib/utils';
import { changePasswordAction } from '../../actions';

/** A rough strength estimate for the meter (the API decides). */
function strength(pw: string): 0 | 1 | 2 | 3 | 4 {
    if (!pw) return 0;
    let score = 0;
    if (pw.length >= 12) score++;
    if (pw.length >= 16) score++;
    if (/[a-z]/.test(pw) && /[A-Z]/.test(pw)) score++;
    if (/\d/.test(pw) && /[^A-Za-z0-9]/.test(pw)) score++;
    return Math.max(1, Math.min(4, score)) as 1 | 2 | 3 | 4;
}

const LEVEL = ['', 'weak', 'fair', 'good', 'strong'] as const;
const TONE = ['', 'bg-destructive', 'bg-warning', 'bg-chart-2', 'bg-success'];

function PasswordInput({
    id,
    name,
    autoComplete,
    value,
    onChange,
    invalid,
    minLength,
}: {
    id: string;
    name: string;
    autoComplete: string;
    value?: string;
    onChange?: (v: string) => void;
    invalid?: boolean;
    minLength?: number;
}) {
    const t = useT();
    const [show, setShow] = useState(false);
    return (
        <div className="relative">
            <Lock className="pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2 text-muted-foreground" />
            <Input
                id={id}
                name={name}
                type={show ? 'text' : 'password'}
                autoComplete={autoComplete}
                required
                minLength={minLength}
                value={value}
                aria-invalid={invalid || undefined}
                onChange={
                    onChange ? (e) => onChange(e.target.value) : undefined
                }
                className="px-8"
            />
            <button
                type="button"
                onClick={() => setShow((s) => !s)}
                aria-label={
                    show
                        ? t('account.password.hide')
                        : t('account.password.show')
                }
                className="absolute top-1/2 right-1.5 flex size-6 -translate-y-1/2 items-center justify-center rounded text-muted-foreground hover:text-foreground"
            >
                {show ? (
                    <EyeOff className="size-4" />
                ) : (
                    <Eye className="size-4" />
                )}
            </button>
        </div>
    );
}

export function PasswordForm() {
    const t = useT();
    const ref = useRef<HTMLFormElement>(null);
    const [next, setNext] = useState('');
    const [state, action] = useActionState<ActionResult | null, FormData>(
        async (prev, form) => {
            const r = await changePasswordAction(prev, form);
            if (r.ok) {
                ref.current?.reset();
                setNext('');
            }
            return r;
        },
        null,
    );
    const level = strength(next);
    const currentErr = fieldError(state, 'current_password');
    const newErr = fieldError(state, 'new_password');
    return (
        <form ref={ref} action={action} className="space-y-4">
            <FormStatus state={state} />
            <div className="space-y-2">
                <label htmlFor="pw-current" className="text-sm font-medium">
                    {t('account.password.current')}
                </label>
                <PasswordInput
                    id="pw-current"
                    name="current_password"
                    autoComplete="current-password"
                    invalid={!!currentErr}
                />
                {currentErr && (
                    <p className="text-xs text-destructive">{currentErr}</p>
                )}
            </div>
            <div className="space-y-2">
                <label htmlFor="pw-new" className="text-sm font-medium">
                    {t('account.password.new')}
                </label>
                <PasswordInput
                    id="pw-new"
                    name="new_password"
                    autoComplete="new-password"
                    minLength={12}
                    value={next}
                    onChange={setNext}
                    invalid={!!newErr}
                />
                <div className="flex items-center gap-2">
                    <div className="grid flex-1 grid-cols-4 gap-1">
                        {[1, 2, 3, 4].map((i) => (
                            <span
                                key={i}
                                className={cn(
                                    'h-1.5 rounded-full bg-muted transition-colors',
                                    i <= level && TONE[level],
                                )}
                            />
                        ))}
                    </div>
                    <span className="w-14 text-right text-xs text-muted-foreground">
                        {level > 0 &&
                            t(
                                `account.password.strength.${LEVEL[level]}` as Parameters<
                                    typeof t
                                >[0],
                            )}
                    </span>
                </div>
                <p className="text-xs text-muted-foreground">
                    {newErr ?? t('account.password.newHint')}
                </p>
            </div>
            <SubmitButton>
                <KeyRound />
                {t('account.password.submit')}
            </SubmitButton>
        </form>
    );
}
