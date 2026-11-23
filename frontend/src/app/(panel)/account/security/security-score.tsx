import Link from 'next/link';
import { CircleCheck, CircleAlert, ShieldCheck } from 'lucide-react';
import { cn } from '@/lib/utils';

/** A summary of how well the account is protected, with what to fix. */
export function SecurityScore({
    mfa,
    title,
    levels,
    checks,
}: {
    mfa: boolean;
    sessions: number;
    title: string;
    levels: { strong: string; fair: string; weak: string };
    checks: { ok: boolean; label: string; href: string }[];
}) {
    // Two-factor carries most of the weight; the password is always set.
    const score = 40 + (mfa ? 45 : 0) + (checks[1]?.ok ? 15 : 5);
    const level = score >= 85 ? 'strong' : score >= 60 ? 'fair' : 'weak';
    const tone = {
        strong: 'text-success',
        fair: 'text-warning',
        weak: 'text-destructive',
    }[level];
    const r = 34;
    const c = 2 * Math.PI * r;
    return (
        <div className="mb-6 flex flex-col gap-6 rounded-3xl border bg-linear-to-br from-card to-accent/30 p-5 shadow-xs sm:flex-row sm:items-center">
            <div className="flex items-center gap-4">
                <div className="relative size-20 shrink-0">
                    <svg viewBox="0 0 80 80" className="size-20 -rotate-90">
                        <circle
                            cx="40"
                            cy="40"
                            r={r}
                            className="fill-none stroke-muted"
                            strokeWidth="8"
                        />
                        <circle
                            cx="40"
                            cy="40"
                            r={r}
                            className={cn(
                                'fill-none stroke-current transition-[stroke-dashoffset] duration-700',
                                tone,
                            )}
                            strokeWidth="8"
                            strokeLinecap="round"
                            strokeDasharray={c}
                            strokeDashoffset={c * (1 - score / 100)}
                        />
                    </svg>
                    <ShieldCheck
                        className={cn('absolute inset-0 m-auto size-7', tone)}
                    />
                </div>
                <div>
                    <p className="text-sm text-muted-foreground">{title}</p>
                    <p
                        className={cn(
                            'text-2xl font-semibold tracking-tight',
                            tone,
                        )}
                    >
                        {levels[level]}
                    </p>
                </div>
            </div>
            <ul className="grid flex-1 gap-2 sm:grid-cols-2">
                {checks.map((ch) => (
                    <li key={ch.label}>
                        <Link
                            href={ch.href}
                            className="flex items-center gap-2 rounded-xl border bg-background/70 px-3 py-2.5 text-sm transition-colors hover:bg-accent"
                        >
                            {ch.ok ? (
                                <CircleCheck className="size-4 shrink-0 text-success" />
                            ) : (
                                <CircleAlert className="size-4 shrink-0 text-warning" />
                            )}
                            {ch.label}
                        </Link>
                    </li>
                ))}
            </ul>
        </div>
    );
}
