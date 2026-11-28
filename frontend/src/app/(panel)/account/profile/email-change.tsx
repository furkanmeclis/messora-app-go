'use client';

import { useState, useTransition } from 'react';
import { MailCheck } from 'lucide-react';
import { notify } from '@/components/notify';
import { useReauth } from '@/components/reauth';
import { Button } from '@/components/ui/button';
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
    DialogTrigger,
} from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { useT } from '@/i18n/client';
import { requestEmailChangeAction } from '../../actions';

/**
 * Changes the sign-in address: a confirmation link goes to the new
 * address, and the change applies once it is followed. Needs a fresh
 * password confirmation like other sensitive actions.
 */
export function EmailChange({ current }: { current: string }) {
    const t = useT();
    const { guard } = useReauth();
    const [open, setOpen] = useState(false);
    const [email, setEmail] = useState('');
    const [sent, setSent] = useState<string | null>(null);
    const [pending, start] = useTransition();

    const submit = () =>
        start(async () => {
            const r = await guard(() => requestEmailChangeAction(email));
            if (notify(r)) setSent(email);
        });

    return (
        <Dialog
            open={open}
            onOpenChange={(o) => {
                setOpen(o);
                if (!o) {
                    setSent(null);
                    setEmail('');
                }
            }}
        >
            <DialogTrigger asChild>
                <Button type="button" variant="link" className="h-auto p-0">
                    {t('account.emailChange.open')}
                </Button>
            </DialogTrigger>
            <DialogContent>
                <DialogHeader>
                    <DialogTitle>{t('account.emailChange.title')}</DialogTitle>
                    <DialogDescription>
                        {t('account.emailChange.description', { current })}
                    </DialogDescription>
                </DialogHeader>
                {sent ? (
                    <p
                        role="status"
                        className="flex items-start gap-2 rounded-lg border border-success/40 bg-success/10 p-3 text-sm"
                    >
                        <MailCheck className="mt-0.5 size-4 shrink-0 text-success" />
                        {t('account.emailChange.sent', { email: sent })}
                    </p>
                ) : (
                    <form
                        id="email-change"
                        className="grid gap-2"
                        onSubmit={(e) => {
                            e.preventDefault();
                            submit();
                        }}
                    >
                        <Label htmlFor="new-email">
                            {t('account.emailChange.newEmail')}
                        </Label>
                        <Input
                            id="new-email"
                            type="email"
                            autoComplete="email"
                            required
                            value={email}
                            onChange={(e) => setEmail(e.target.value)}
                        />
                    </form>
                )}
                <DialogFooter>
                    {sent ? (
                        <Button onClick={() => setOpen(false)}>
                            {t('common.close')}
                        </Button>
                    ) : (
                        <Button
                            type="submit"
                            form="email-change"
                            disabled={pending || !email}
                        >
                            {t('account.emailChange.submit')}
                        </Button>
                    )}
                </DialogFooter>
            </DialogContent>
        </Dialog>
    );
}
