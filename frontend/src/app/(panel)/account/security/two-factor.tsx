'use client';

/* eslint-disable @next/next/no-img-element -- the QR code is a data: URL */

import { useActionState, useState, useTransition } from 'react';
import { useRouter } from 'next/navigation';
import { Copy, ShieldCheck } from 'lucide-react';
import { toast } from 'sonner';
import {
    FormStatus,
    SubmitButton,
    TextField,
    fieldError,
} from '@/components/forms';
import { notify } from '@/components/notify';
import { useReauth } from '@/components/reauth';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
    Card,
    CardContent,
    CardDescription,
    CardHeader,
    CardTitle,
} from '@/components/ui/card';
import { useT } from '@/i18n/client';
import type { ActionResult } from '@/lib/types';
import {
    confirmTotpAction,
    disableTotpAction,
    regenerateCodesAction,
    startTotpAction,
} from '../../actions';

function RecoveryCodes({
    codes,
    onDone,
}: {
    codes: string[];
    onDone: () => void;
}) {
    const t = useT();
    const text = codes.join('\n');
    return (
        <div className="space-y-3">
            <p className="text-sm">
                {t('account.twoFactor.recoveryIntro')}{' '}
                <strong>{t('account.twoFactor.recoveryWarning')}</strong>
            </p>
            <pre className="grid grid-cols-2 gap-1 rounded-md bg-muted p-3 font-mono text-sm">
                {codes.map((c) => (
                    <span key={c}>{c}</span>
                ))}
            </pre>
            <div className="flex gap-2">
                <Button
                    type="button"
                    variant="outline"
                    onClick={() => {
                        void navigator.clipboard.writeText(text);
                        toast.success(t('account.twoFactor.recoveryCopied'));
                    }}
                >
                    <Copy /> {t('common.copy')}
                </Button>
                <Button type="button" onClick={onDone}>
                    {t('account.twoFactor.savedThem')}
                </Button>
            </div>
        </div>
    );
}

export function TwoFactorCard({
    enabled,
    required,
}: {
    enabled: boolean;
    required: boolean;
}) {
    const t = useT();
    const router = useRouter();
    const { guard } = useReauth();
    const [pending, startTransition] = useTransition();
    const [setup, setSetup] = useState<{ secret: string; qr: string } | null>(
        null,
    );
    const [codes, setCodes] = useState<string[] | null>(null);
    const [confirmState, confirm] = useActionState<
        ActionResult<string[]> | null,
        FormData
    >(async (prev, form) => {
        const r = await confirmTotpAction(prev, form);
        if (r.ok) {
            setSetup(null);
            setCodes(r.data);
        }
        return r;
    }, null);

    const finish = () => {
        setCodes(null);
        router.refresh();
    };

    return (
        <Card>
            <CardHeader>
                <CardTitle className="flex items-center gap-2">
                    {t('account.twoFactor.title')}
                    <Badge variant={enabled ? 'secondary' : 'outline'}>
                        {enabled
                            ? t('account.twoFactor.on')
                            : t('account.twoFactor.off')}
                    </Badge>
                </CardTitle>
                <CardDescription>
                    {t('account.twoFactor.description')}
                </CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
                {codes ? (
                    <RecoveryCodes codes={codes} onDone={finish} />
                ) : setup ? (
                    <div className="space-y-4">
                        <div className="flex flex-wrap items-start gap-6">
                            <img
                                src={setup.qr}
                                alt={t('account.twoFactor.qrAlt')}
                                width={180}
                                height={180}
                                className="rounded-md border bg-white p-1"
                            />
                            <div className="min-w-0 flex-1 space-y-2 text-sm">
                                <p>{t('account.twoFactor.scan')}</p>
                                <code className="block rounded bg-muted p-2 font-mono break-all">
                                    {setup.secret}
                                </code>
                            </div>
                        </div>
                        <form action={confirm} className="max-w-xs space-y-3">
                            <FormStatus state={confirmState} />
                            <TextField
                                label={t('account.twoFactor.code')}
                                name="code"
                                inputMode="numeric"
                                pattern="\d{6}"
                                maxLength={6}
                                autoComplete="one-time-code"
                                required
                                autoFocus
                                error={fieldError(confirmState, 'code')}
                            />
                            <div className="flex gap-2">
                                <SubmitButton>
                                    {t('account.twoFactor.turnOn')}
                                </SubmitButton>
                                <Button
                                    type="button"
                                    variant="ghost"
                                    onClick={() => setSetup(null)}
                                >
                                    {t('common.cancel')}
                                </Button>
                            </div>
                        </form>
                    </div>
                ) : enabled ? (
                    <div className="flex flex-wrap gap-2">
                        <Button
                            variant="outline"
                            disabled={pending}
                            onClick={() =>
                                startTransition(async () => {
                                    const r = await guard(() =>
                                        regenerateCodesAction(),
                                    );
                                    if (notify(r)) setCodes(r.data);
                                })
                            }
                        >
                            {t('account.twoFactor.newCodes')}
                        </Button>
                        <Button
                            variant="outline"
                            disabled={pending}
                            onClick={() =>
                                startTransition(async () => {
                                    if (
                                        notify(
                                            await guard(() =>
                                                disableTotpAction(),
                                            ),
                                        )
                                    )
                                        router.refresh();
                                })
                            }
                        >
                            {t('account.twoFactor.turnOff')}
                        </Button>
                    </div>
                ) : (
                    <Button
                        disabled={pending}
                        onClick={() =>
                            startTransition(async () => {
                                const r = await guard(() => startTotpAction());
                                if (notify(r)) setSetup(r.data);
                            })
                        }
                    >
                        <ShieldCheck />{' '}
                        {required
                            ? t('account.twoFactor.setUpNow')
                            : t('account.twoFactor.turnOn')}
                    </Button>
                )}
            </CardContent>
        </Card>
    );
}
