'use client';

import { useRef, useState } from 'react';
import { useFormStatus } from 'react-dom';
import { AlertCircle, ArrowBigUp, Eye, EyeOff } from 'lucide-react';
import { Field, FieldDescription, FieldLabel } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { Spinner } from '@/components/ui/spinner';
import { Button } from '@/components/ui/button';
import { useT } from '@/i18n/client';
import {
    applyOtpBackspace,
    applyOtpInput,
    applyOtpPaste,
    emptyOtp,
    type OtpChange,
} from '@/lib/auth/otp';
import { passwordStrength, PASSWORD_MIN } from '@/lib/auth/password-strength';
import { cn } from '@/lib/utils';

/** Inputs on the sign-in screens: 44 px tall, roomy, readable. */
export const authInputClass =
    'h-11 rounded-xl bg-background px-3.5 text-[15px] md:text-[15px] dark:bg-input/30';

function describedBy(...ids: (string | false | null | undefined)[]) {
    const s = ids.filter(Boolean).join(' ');
    return s || undefined;
}

function FieldMessage({ id, error }: { id: string; error?: string }) {
    if (!error) return null;
    return (
        <p
            id={id}
            className="flex items-start gap-1.5 text-sm text-destructive"
        >
            <AlertCircle className="mt-0.5 size-4 shrink-0" aria-hidden />
            {error}
        </p>
    );
}

type AuthFieldProps = React.ComponentProps<typeof Input> & {
    label: string;
    name: string;
    error?: string;
    hint?: React.ReactNode;
    /** Content inside the input's right edge (e.g. a suffix). */
    trailing?: React.ReactNode;
};

/** Labelled input with an inline hint or error. */
export function AuthField({
    label,
    name,
    error,
    hint,
    id,
    className,
    trailing,
    ...props
}: AuthFieldProps) {
    const inputId = id ?? `field-${name}`;
    return (
        <Field data-invalid={error ? true : undefined} className="gap-2">
            <FieldLabel htmlFor={inputId} className="text-sm font-medium">
                {label}
            </FieldLabel>
            <div className="relative">
                <Input
                    id={inputId}
                    name={name}
                    aria-invalid={error ? true : undefined}
                    aria-describedby={describedBy(
                        error && `${inputId}-error`,
                        hint ? `${inputId}-hint` : null,
                    )}
                    className={cn(authInputClass, className)}
                    {...props}
                />
                {trailing}
            </div>
            <FieldMessage id={`${inputId}-error`} error={error} />
            {hint && (
                <FieldDescription id={`${inputId}-hint`}>
                    {hint}
                </FieldDescription>
            )}
        </Field>
    );
}

const STRENGTH_BAR = [
    '',
    'bg-destructive',
    'bg-coral',
    'bg-primary',
    'bg-emerald-500 dark:bg-mint',
];

/**
 * Password input with a show/hide toggle and a Caps Lock warning. New
 * passwords also get a strength meter (`meter`), scored against the
 * person's own name and e-mail (`personal`).
 */
export function PasswordField({
    label,
    name,
    error,
    hint,
    meter = false,
    personal,
    id,
    onChange,
    ...props
}: Omit<AuthFieldProps, 'type' | 'trailing'> & {
    meter?: boolean;
    personal?: readonly string[];
}) {
    const t = useT();
    const inputId = id ?? `field-${name}`;
    const [visible, setVisible] = useState(false);
    const [caps, setCaps] = useState(false);
    const [value, setValue] = useState('');
    const strength = meter ? passwordStrength(value, personal) : null;
    const missing = Math.max(0, PASSWORD_MIN - [...value].length);
    const keyState = (e: React.KeyboardEvent<HTMLInputElement>) => {
        if (typeof e.getModifierState === 'function') {
            setCaps(e.getModifierState('CapsLock'));
        }
    };
    const tip =
        strength && strength.score > 0
            ? strength.tooShort
                ? t('auth.passwordField.tooShort', { count: missing })
                : t(
                      `auth.passwordField.tip${
                          (
                              {
                                  weak: 'Weak',
                                  fair: 'Fair',
                                  good: 'Good',
                                  strong: 'Strong',
                                  empty: 'Weak',
                              } as const
                          )[strength.label]
                      }`,
                  )
            : null;

    return (
        <Field data-invalid={error ? true : undefined} className="gap-2">
            <FieldLabel htmlFor={inputId} className="text-sm font-medium">
                {label}
            </FieldLabel>
            <div className="relative">
                <Input
                    id={inputId}
                    name={name}
                    type={visible ? 'text' : 'password'}
                    autoCapitalize="none"
                    autoCorrect="off"
                    spellCheck={false}
                    aria-invalid={error ? true : undefined}
                    aria-describedby={describedBy(
                        error && `${inputId}-error`,
                        caps && `${inputId}-caps`,
                        strength && `${inputId}-strength`,
                        hint ? `${inputId}-hint` : null,
                    )}
                    className={cn(authInputClass, 'pr-12')}
                    onKeyDown={keyState}
                    onKeyUp={keyState}
                    onBlur={() => setCaps(false)}
                    onChange={(e) => {
                        setValue(e.target.value);
                        onChange?.(e);
                    }}
                    {...props}
                />
                <button
                    type="button"
                    onClick={() => setVisible((v) => !v)}
                    aria-label={
                        visible
                            ? t('auth.passwordField.hide')
                            : t('auth.passwordField.show')
                    }
                    aria-controls={inputId}
                    className="absolute inset-y-0 right-0 flex w-11 items-center justify-center rounded-r-xl text-muted-foreground transition-colors hover:text-foreground focus-visible:ring-3 focus-visible:ring-ring/50 focus-visible:outline-none"
                >
                    {visible ? (
                        <EyeOff className="size-[18px]" aria-hidden />
                    ) : (
                        <Eye className="size-[18px]" aria-hidden />
                    )}
                </button>
            </div>
            {caps && (
                <p
                    id={`${inputId}-caps`}
                    role="status"
                    className="flex items-center gap-1.5 text-sm font-medium text-[#A63D1C] dark:text-coral"
                >
                    <ArrowBigUp className="size-4" aria-hidden />
                    {t('auth.passwordField.capsLock')}
                </p>
            )}
            <FieldMessage id={`${inputId}-error`} error={error} />
            {strength && (
                <div
                    id={`${inputId}-strength`}
                    className="flex flex-col gap-1.5"
                >
                    <div className="grid grid-cols-4 gap-1.5" aria-hidden>
                        {[1, 2, 3, 4].map((i) => (
                            <span
                                key={i}
                                className={cn(
                                    'h-1.5 rounded-full transition-colors duration-300',
                                    i <= strength.score
                                        ? STRENGTH_BAR[strength.score]
                                        : 'bg-foreground/10',
                                )}
                            />
                        ))}
                    </div>
                    {strength.score > 0 && (
                        <p className="flex flex-wrap justify-between gap-x-3 text-xs text-muted-foreground">
                            <span aria-live="polite" className="font-medium">
                                {t('auth.passwordField.strength', {
                                    label: t(
                                        `auth.passwordField.${
                                            strength.label === 'empty'
                                                ? 'weak'
                                                : strength.label
                                        }`,
                                    ),
                                })}
                            </span>
                            <span>{tip}</span>
                        </p>
                    )}
                </div>
            )}
            {hint && !strength?.score && (
                <FieldDescription id={`${inputId}-hint`}>
                    {hint}
                </FieldDescription>
            )}
        </Field>
    );
}

/**
 * Six single-digit boxes for an authenticator code. Typing moves on,
 * backspace steps back, arrows move, and a pasted or autofilled code fills
 * every box. The joined code is submitted as `name`.
 */
export function OtpInput({
    name,
    label,
    error,
    autoFocus,
}: {
    name: string;
    label: string;
    error?: string;
    autoFocus?: boolean;
}) {
    const t = useT();
    const [digits, setDigits] = useState(emptyOtp);
    const refs = useRef<(HTMLInputElement | null)[]>([]);
    const focus = (i: number) => {
        const el = refs.current[Math.max(0, Math.min(digits.length - 1, i))];
        el?.focus();
        el?.select();
    };
    const apply = (c: OtpChange) => {
        setDigits(c.digits);
        focus(c.focus);
    };
    return (
        <Field data-invalid={error ? true : undefined} className="gap-2">
            <FieldLabel
                id={`otp-${name}-label`}
                htmlFor={`otp-${name}-0`}
                className="text-sm font-medium"
            >
                {label}
            </FieldLabel>
            <input type="hidden" name={name} value={digits.join('')} />
            <div
                role="group"
                aria-labelledby={`otp-${name}-label`}
                aria-describedby={error ? `otp-${name}-error` : undefined}
                className="grid grid-cols-6 gap-2 sm:gap-2.5"
            >
                {digits.map((d, i) => (
                    <input
                        key={i}
                        ref={(el) => {
                            refs.current[i] = el;
                        }}
                        id={`otp-${name}-${i}`}
                        value={d}
                        inputMode="numeric"
                        pattern="[0-9]*"
                        autoComplete={i === 0 ? 'one-time-code' : 'off'}
                        autoFocus={autoFocus && i === 0}
                        aria-label={t('auth.verify.digit', {
                            n: i + 1,
                            total: digits.length,
                        })}
                        aria-invalid={error ? true : undefined}
                        onFocus={(e) => e.target.select()}
                        onChange={(e) =>
                            apply(applyOtpInput(digits, i, e.target.value))
                        }
                        onPaste={(e) => {
                            e.preventDefault();
                            apply(
                                applyOtpPaste(
                                    digits,
                                    i,
                                    e.clipboardData.getData('text'),
                                ),
                            );
                        }}
                        onKeyDown={(e) => {
                            if (e.key === 'Backspace') {
                                e.preventDefault();
                                apply(applyOtpBackspace(digits, i));
                            } else if (e.key === 'ArrowLeft') {
                                e.preventDefault();
                                focus(i - 1);
                            } else if (e.key === 'ArrowRight') {
                                e.preventDefault();
                                focus(i + 1);
                            } else if (e.key === 'Home') {
                                e.preventDefault();
                                focus(0);
                            } else if (e.key === 'End') {
                                e.preventDefault();
                                focus(digits.length - 1);
                            }
                        }}
                        className="h-14 w-full min-w-0 rounded-xl border border-input bg-background text-center font-display text-2xl font-bold tabular-nums caret-primary transition-colors outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 aria-invalid:border-destructive aria-invalid:ring-3 aria-invalid:ring-destructive/20 sm:h-[60px] dark:bg-input/30"
                    />
                ))}
            </div>
            <FieldMessage id={`otp-${name}-error`} error={error} />
        </Field>
    );
}

/** The main button: full width, 44 px, spinner while the action runs. */
export function AuthSubmit({
    children,
    variant,
}: {
    children: React.ReactNode;
    variant?: React.ComponentProps<typeof Button>['variant'];
}) {
    const { pending } = useFormStatus();
    return (
        <Button
            type="submit"
            variant={variant}
            disabled={pending}
            aria-busy={pending || undefined}
            className="h-11 w-full rounded-xl text-[15px] font-semibold"
        >
            {pending && <Spinner aria-hidden />}
            {children}
        </Button>
    );
}

/**
 * The list of problems shown above a form after a failed submission,
 * linking to each field. Screen readers hear it at once.
 */
export function ErrorSummary({
    title,
    errors,
    labels,
    summaryRef,
}: {
    title: string;
    errors: Record<string, string | undefined>;
    labels: Record<string, string>;
    summaryRef?: React.Ref<HTMLDivElement>;
}) {
    const items = Object.entries(errors).filter(
        (e): e is [string, string] => !!e[1],
    );
    return (
        <div
            ref={summaryRef}
            tabIndex={-1}
            role="alert"
            aria-live="assertive"
            className={cn(
                'rounded-xl border border-destructive/25 bg-destructive/5 px-4 py-3 text-sm outline-none focus-visible:ring-3 focus-visible:ring-destructive/30',
                items.length === 0 && 'sr-only',
            )}
        >
            {items.length > 0 && (
                <>
                    <p className="flex items-center gap-2 font-semibold text-destructive">
                        <AlertCircle className="size-4 shrink-0" aria-hidden />
                        {title}
                    </p>
                    <ul className="mt-1.5 flex flex-wrap gap-x-3 gap-y-1 pl-6">
                        {items.map(([name]) => (
                            <li key={name}>
                                <a
                                    href={`#field-${name}`}
                                    onClick={(e) => {
                                        const el = document.getElementById(
                                            `field-${name}`,
                                        );
                                        if (el) {
                                            e.preventDefault();
                                            el.focus();
                                        }
                                    }}
                                    className="text-foreground underline underline-offset-2 hover:text-destructive"
                                >
                                    {labels[name] ?? name}
                                </a>
                            </li>
                        ))}
                    </ul>
                </>
            )}
        </div>
    );
}

/** Banner for the server's answer that is not about a single field. */
export function FormAlert({
    tone = 'error',
    children,
}: {
    tone?: 'error' | 'success' | 'info';
    children: React.ReactNode;
}) {
    return (
        <div
            role={tone === 'error' ? 'alert' : 'status'}
            aria-live={tone === 'error' ? 'assertive' : 'polite'}
            className={cn(
                'flex items-start gap-2.5 rounded-xl border px-4 py-3 text-sm',
                tone === 'error' &&
                    'border-destructive/25 bg-destructive/5 text-destructive',
                tone === 'success' &&
                    'border-emerald-600/20 bg-emerald-500/8 text-emerald-800 dark:border-mint/25 dark:bg-mint/10 dark:text-mint',
                tone === 'info' &&
                    'border-primary/20 bg-primary/5 text-foreground',
            )}
        >
            {children}
        </div>
    );
}
