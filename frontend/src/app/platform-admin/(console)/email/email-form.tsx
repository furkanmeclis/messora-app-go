'use client';

import { useState, useTransition } from 'react';
import { useRouter } from 'next/navigation';
import { Info, Send } from 'lucide-react';
import { notify } from '@/components/notify';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
    Card,
    CardContent,
    CardDescription,
    CardHeader,
    CardTitle,
} from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { NumberField } from '@/components/ui/number-field';
import { Segmented } from '@/components/ui/segmented';
import { useT } from '@/i18n/client';
import type { Schemas } from '@/lib/types';
import {
    testPlatformEmailAction,
    updatePlatformEmailAction,
} from '../../plan-actions';

type Input_ = Schemas['SmtpSettingsInput'];

export function EmailSenderForm({
    email: e,
    defaultTestTo,
}: {
    email: Schemas['PlatformEmail'];
    defaultTestTo: string;
}) {
    const t = useT();
    const router = useRouter();
    const panel = e.source === 'panel';
    const [v, setV] = useState<Input_>({
        host: panel ? e.host : '',
        port: panel ? e.port : 465,
        username: panel ? e.username : 'resend',
        from: panel ? e.from : '',
        tls: panel ? e.tls : 'tls',
    });
    const [password, setPassword] = useState('');
    const [testTo, setTestTo] = useState(defaultTestTo);
    const [pending, start] = useTransition();
    const input = (): Input_ => ({
        ...v,
        port: v.port || 0,
        ...(password ? { password } : {}),
    });
    const save = (body: Input_) =>
        start(async () => {
            if (notify(await updatePlatformEmailAction(body))) {
                setPassword('');
                router.refresh();
            }
        });
    const test = (withDraft: boolean) =>
        start(async () => {
            notify(
                await testPlatformEmailAction(
                    withDraft && v.host
                        ? { to: testTo, config: input() }
                        : { to: testTo },
                ),
            );
        });
    const tone =
        e.source === 'none'
            ? 'border-warning/40 bg-warning/10 text-warning'
            : 'border-success/40 bg-success/10 text-success';
    return (
        <div className="grid max-w-3xl gap-6">
            <Card>
                <CardHeader>
                    <CardTitle className="flex items-center gap-2">
                        {t('plans.admin.email.title')}
                        <Badge
                            variant="outline"
                            className={tone}
                            data-testid="email-source"
                            data-source={e.source}
                        >
                            {t(`plans.admin.email.source.${e.source}`)}
                        </Badge>
                    </CardTitle>
                    <CardDescription>
                        {e.source !== 'none' && (
                            <span className="font-mono">
                                {e.from} · {e.host}:{e.port}
                            </span>
                        )}
                    </CardDescription>
                </CardHeader>
                <CardContent className="grid gap-5">
                    <Alert>
                        <Info />
                        <AlertDescription>
                            {t('plans.admin.email.resendHint')}
                        </AlertDescription>
                    </Alert>
                    <form
                        className="grid gap-4"
                        onSubmit={(ev) => {
                            ev.preventDefault();
                            save(input());
                        }}
                    >
                        <div className="grid gap-4 sm:grid-cols-[1fr_8rem]">
                            <div className="grid gap-2">
                                <Label htmlFor="pe-host">
                                    {t('plans.admin.email.host')}
                                </Label>
                                <Input
                                    id="pe-host"
                                    value={v.host}
                                    placeholder="smtp.resend.com"
                                    autoComplete="off"
                                    onChange={(x) =>
                                        setV({ ...v, host: x.target.value })
                                    }
                                />
                            </div>
                            <div className="grid gap-2">
                                <Label htmlFor="pe-port">
                                    {t('plans.admin.email.port')}
                                </Label>
                                <NumberField
                                    id="pe-port"
                                    aria-label={t('plans.admin.email.port')}
                                    value={v.port || null}
                                    allowEmpty
                                    min={0}
                                    max={65535}
                                    onValueChange={(n) =>
                                        setV({ ...v, port: n ?? 0 })
                                    }
                                />
                            </div>
                        </div>
                        <div className="grid gap-2">
                            <Label>{t('plans.admin.email.tls')}</Label>
                            <Segmented
                                value={v.tls}
                                onValueChange={(tls) => setV({ ...v, tls })}
                                options={(
                                    ['tls', 'starttls', 'none'] as const
                                ).map((k) => ({
                                    value: k,
                                    label: t(`settings.email.tls.${k}`),
                                }))}
                            />
                        </div>
                        <div className="grid gap-4 sm:grid-cols-2">
                            <div className="grid gap-2">
                                <Label htmlFor="pe-user">
                                    {t('plans.admin.email.username')}
                                </Label>
                                <Input
                                    id="pe-user"
                                    value={v.username}
                                    autoComplete="off"
                                    onChange={(x) =>
                                        setV({ ...v, username: x.target.value })
                                    }
                                />
                            </div>
                            <div className="grid gap-2">
                                <Label htmlFor="pe-pass">
                                    {t('plans.admin.email.password')}
                                </Label>
                                <Input
                                    id="pe-pass"
                                    type="password"
                                    autoComplete="new-password"
                                    value={password}
                                    placeholder={
                                        e.password_set && panel
                                            ? t(
                                                  'plans.admin.email.passwordKeep',
                                              )
                                            : ''
                                    }
                                    onChange={(x) =>
                                        setPassword(x.target.value)
                                    }
                                />
                            </div>
                        </div>
                        <div className="grid gap-2">
                            <Label htmlFor="pe-from">
                                {t('plans.admin.email.from')}
                            </Label>
                            <Input
                                id="pe-from"
                                value={v.from}
                                placeholder="Messora <no-reply@messora.tech>"
                                onChange={(x) =>
                                    setV({ ...v, from: x.target.value })
                                }
                            />
                        </div>
                        <div className="flex flex-wrap justify-end gap-2">
                            {panel && (
                                <Button
                                    type="button"
                                    variant="outline"
                                    disabled={pending}
                                    onClick={() =>
                                        save({
                                            host: '',
                                            port: 0,
                                            username: '',
                                            from: '',
                                            tls: 'tls',
                                        })
                                    }
                                >
                                    {t('plans.admin.email.clear')}
                                </Button>
                            )}
                            <Button
                                type="submit"
                                disabled={pending || !v.host}
                                data-testid="platform-email-save"
                            >
                                {t('plans.admin.email.save')}
                            </Button>
                        </div>
                    </form>
                    <div className="grid gap-2 rounded-xl border border-dashed p-4">
                        <Label htmlFor="pe-test">
                            {t('plans.admin.email.testTo')}
                        </Label>
                        <div className="flex gap-2">
                            <Input
                                id="pe-test"
                                type="email"
                                value={testTo}
                                onChange={(x) => setTestTo(x.target.value)}
                            />
                            <Button
                                type="button"
                                variant="secondary"
                                disabled={pending || !testTo}
                                onClick={() =>
                                    test(
                                        password !== '' ||
                                            v.host !== (panel ? e.host : ''),
                                    )
                                }
                            >
                                <Send /> {t('plans.admin.email.test')}
                            </Button>
                        </div>
                    </div>
                </CardContent>
            </Card>
            <Card>
                <CardHeader>
                    <CardTitle>{t('plans.admin.email.envTitle')}</CardTitle>
                    <CardDescription>
                        {t('plans.admin.email.envHint')}
                    </CardDescription>
                </CardHeader>
                <CardContent>
                    <dl className="grid gap-4 text-sm sm:grid-cols-3">
                        <div>
                            <dt className="text-xs text-muted-foreground">
                                SMTP_HOST
                            </dt>
                            <dd className="font-mono break-all">
                                {e.env.host || '—'}
                            </dd>
                        </div>
                        <div>
                            <dt className="text-xs text-muted-foreground">
                                SMTP_FROM
                            </dt>
                            <dd className="font-mono break-all">
                                {e.env.from || '—'}
                            </dd>
                        </div>
                        <div>
                            <dt className="text-xs text-muted-foreground">
                                {t('plans.admin.email.replyTo')}
                            </dt>
                            <dd className="font-mono break-all">
                                {e.reply_to ?? '—'}
                            </dd>
                        </div>
                    </dl>
                </CardContent>
            </Card>
        </div>
    );
}
