'use client';

import { useState, useTransition } from 'react';
import Link from 'next/link';
import { Button } from '@/components/ui/button';
import {
    Dialog,
    DialogContent,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from '@/components/ui/dialog';
import { Textarea } from '@/components/ui/textarea';
import { useT } from '@/i18n/client';
import { cn } from '@/lib/utils';

/** A row of status filters as links (?status=…). */
export function FilterLinks({
    base,
    current,
    options,
}: {
    base: string;
    current: string;
    options: { value: string; label: string }[];
}) {
    return (
        <nav className="flex flex-wrap gap-2">
            {options.map((o) => (
                <Link
                    key={o.value}
                    href={`${base}?status=${o.value}`}
                    className={cn(
                        'rounded-full border px-3 py-1 text-sm',
                        current === o.value
                            ? 'border-primary bg-primary/10 text-primary'
                            : 'text-muted-foreground hover:text-foreground',
                    )}
                >
                    {o.label}
                </Link>
            ))}
        </nav>
    );
}

/** A button that asks for a reason (3–500 characters) before acting. */
export function ReasonButton({
    label,
    title,
    onConfirm,
    variant = 'outline',
    testId,
}: {
    label: string;
    title: string;
    onConfirm: (reason: string) => Promise<boolean>;
    variant?: 'outline' | 'ghost' | 'destructive';
    testId?: string;
}) {
    const t = useT();
    const [open, setOpen] = useState(false);
    const [reason, setReason] = useState('');
    const [pending, start] = useTransition();
    return (
        <>
            <Button
                size="sm"
                variant={variant}
                onClick={() => setOpen(true)}
                data-testid={testId}
            >
                {label}
            </Button>
            <Dialog open={open} onOpenChange={setOpen}>
                <DialogContent>
                    <DialogHeader>
                        <DialogTitle>{title}</DialogTitle>
                    </DialogHeader>
                    <Textarea
                        value={reason}
                        onChange={(e) => setReason(e.target.value)}
                        maxLength={500}
                        aria-label={title}
                    />
                    <DialogFooter>
                        <Button
                            variant="ghost"
                            onClick={() => setOpen(false)}
                            disabled={pending}
                        >
                            {t('common.cancel')}
                        </Button>
                        <Button
                            disabled={pending || reason.trim().length < 3}
                            onClick={() =>
                                start(async () => {
                                    if (await onConfirm(reason.trim())) {
                                        setOpen(false);
                                        setReason('');
                                    }
                                })
                            }
                        >
                            {label}
                        </Button>
                    </DialogFooter>
                </DialogContent>
            </Dialog>
        </>
    );
}

/** A number on the revenue page. */
export function Stat({
    label,
    value,
    testId,
    tone,
}: {
    label: string;
    value: string;
    testId?: string;
    tone?: 'warning';
}) {
    return (
        <div className="rounded-2xl border bg-card p-4 shadow-xs">
            <div className="text-xs text-muted-foreground">{label}</div>
            <div
                className={cn(
                    'mt-1 text-2xl font-semibold tracking-tight',
                    tone === 'warning' && 'text-warning',
                )}
                data-testid={testId}
            >
                {value}
            </div>
        </div>
    );
}
