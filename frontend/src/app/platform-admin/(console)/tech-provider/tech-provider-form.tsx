'use client';

import { useState, useTransition } from 'react';
import { useRouter } from 'next/navigation';
import { CheckCircle2, CircleAlert, Stethoscope, XCircle } from 'lucide-react';
import { notify } from '@/components/notify';
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
import { Spinner } from '@/components/ui/spinner';
import { Switch } from '@/components/ui/switch';
import { useT } from '@/i18n/client';
import type { MessageKey } from '@/i18n/translate';
import type { Schemas } from '@/lib/types';
import { cn } from '@/lib/utils';
import { CopyField } from '../copy-field';
import {
    testTechProviderAction,
    updateTechProviderAction,
} from '../../tech-provider-actions';

type Config = Schemas['PlatformTechProvider'];
type Check = Schemas['PlatformTechProviderCheck'];

const ICON = { ok: CheckCircle2, warning: CircleAlert, error: XCircle };
const TONE = {
    ok: 'text-success',
    warning: 'text-warning',
    error: 'text-destructive',
};

function Status({ on, label }: { on: boolean; label: string }) {
    return (
        <Badge
            variant="outline"
            className={
                on
                    ? 'border-success/40 bg-success/10 text-success'
                    : 'border-warning/40 bg-warning/10 text-warning'
            }
        >
            {label}
        </Badge>
    );
}

export function TechProviderForm({ config: c }: { config: Config }) {
    const t = useT();
    const router = useRouter();
    const [pending, start] = useTransition();
    const [appId, setAppId] = useState(c.app_id ?? '');
    const [configId, setConfigId] = useState(c.configuration_id ?? '');
    const [version, setVersion] = useState(c.graph_api_version ?? '');
    const [secret, setSecret] = useState('');
    const [systemToken, setSystemToken] = useState('');
    const [checks, setChecks] = useState<Check[] | null>(null);

    const save = (body: Schemas['UpdatePlatformTechProviderRequest']) =>
        start(async () => {
            if (notify(await updateTechProviderAction(body))) {
                setSecret('');
                setSystemToken('');
                router.refresh();
            }
        });
    const test = () =>
        start(async () => {
            const r = await testTechProviderAction();
            if (r.ok) setChecks(r.data.checks);
            else notify(r);
        });
    const text = (ch: Check) => {
        const key = `platform.techProvider.codes.${ch.code}` as MessageKey;
        const out = t(key, ch.params);
        return out === key ? ch.message : out;
    };

    return (
        <div className="grid max-w-3xl gap-6">
            <Card>
                <CardHeader>
                    <CardTitle className="flex items-center gap-2">
                        {t('platform.techProvider.switch')}
                        <Status
                            on={c.ready}
                            label={
                                c.ready
                                    ? t('platform.techProvider.ready')
                                    : t('platform.techProvider.incomplete')
                            }
                        />
                    </CardTitle>
                    <CardDescription>
                        {t('platform.techProvider.switchHint')}
                    </CardDescription>
                </CardHeader>
                <CardContent>
                    <Label className="flex items-center gap-3 font-normal">
                        <Switch
                            data-testid="tp-enabled"
                            checked={c.enabled}
                            disabled={pending || (!c.ready && !c.enabled)}
                            onCheckedChange={(v) => save({ enabled: v })}
                        />
                        {c.enabled ? t('common.on') : t('common.off')}
                    </Label>
                </CardContent>
            </Card>

            <Card>
                <CardHeader>
                    <CardTitle>{t('platform.techProvider.app')}</CardTitle>
                    <CardDescription>
                        {t('platform.techProvider.appHint')}
                    </CardDescription>
                </CardHeader>
                <CardContent>
                    <form
                        className="grid gap-4"
                        onSubmit={(e) => {
                            e.preventDefault();
                            const body: Schemas['UpdatePlatformTechProviderRequest'] =
                                {
                                    app_id: appId.trim(),
                                    configuration_id: configId.trim(),
                                    graph_api_version: version.trim(),
                                };
                            if (secret.trim()) body.app_secret = secret.trim();
                            if (systemToken.trim())
                                body.system_user_token = systemToken.trim();
                            save(body);
                        }}
                    >
                        <div className="grid gap-4 sm:grid-cols-2">
                            <div className="grid gap-1.5">
                                <Label htmlFor="tp-app-id">
                                    {t('platform.techProvider.appId')}
                                </Label>
                                <Input
                                    id="tp-app-id"
                                    inputMode="numeric"
                                    value={appId}
                                    onChange={(e) => setAppId(e.target.value)}
                                />
                            </div>
                            <div className="grid gap-1.5">
                                <Label htmlFor="tp-config-id">
                                    {t('platform.techProvider.configurationId')}
                                </Label>
                                <Input
                                    id="tp-config-id"
                                    inputMode="numeric"
                                    value={configId}
                                    onChange={(e) =>
                                        setConfigId(e.target.value)
                                    }
                                />
                            </div>
                            <div className="grid gap-1.5">
                                <Label htmlFor="tp-secret">
                                    {t('platform.techProvider.appSecret')}{' '}
                                    <Status
                                        on={c.app_secret_configured}
                                        label={
                                            c.app_secret_configured
                                                ? t(
                                                      'platform.settings.configured',
                                                  )
                                                : t(
                                                      'platform.settings.notConfigured',
                                                  )
                                        }
                                    />
                                </Label>
                                <Input
                                    id="tp-secret"
                                    type="password"
                                    autoComplete="off"
                                    value={secret}
                                    placeholder={t(
                                        'platform.techProvider.secretPlaceholder',
                                    )}
                                    onChange={(e) => setSecret(e.target.value)}
                                />
                            </div>
                            <div className="grid gap-1.5">
                                <Label htmlFor="tp-system-token">
                                    {t('platform.techProvider.systemToken')}{' '}
                                    <Status
                                        on={c.system_user_token_configured}
                                        label={
                                            c.system_user_token_configured
                                                ? t(
                                                      'platform.settings.configured',
                                                  )
                                                : t(
                                                      'platform.settings.notConfigured',
                                                  )
                                        }
                                    />
                                </Label>
                                <Input
                                    id="tp-system-token"
                                    type="password"
                                    autoComplete="off"
                                    value={systemToken}
                                    placeholder={t(
                                        'platform.techProvider.secretPlaceholder',
                                    )}
                                    onChange={(e) =>
                                        setSystemToken(e.target.value)
                                    }
                                />
                            </div>
                            <div className="grid gap-1.5">
                                <Label htmlFor="tp-version">
                                    {t('platform.techProvider.version')}
                                </Label>
                                <Input
                                    id="tp-version"
                                    value={version}
                                    placeholder={c.default_graph_api_version}
                                    onChange={(e) => setVersion(e.target.value)}
                                />
                            </div>
                        </div>
                        <p className="text-xs text-muted-foreground">
                            {t('platform.techProvider.scopes', {
                                scopes: c.scopes.join(', '),
                            })}
                        </p>
                        <div>
                            <Button type="submit" disabled={pending}>
                                {pending && <Spinner />} {t('common.save')}
                            </Button>
                        </div>
                    </form>
                </CardContent>
            </Card>

            <Card>
                <CardHeader>
                    <CardTitle>{t('platform.techProvider.webhook')}</CardTitle>
                    <CardDescription>
                        {t('platform.techProvider.webhookHint')}
                    </CardDescription>
                </CardHeader>
                <CardContent className="grid gap-4">
                    <div className="grid gap-1.5">
                        <Label>{t('platform.techProvider.callbackUrl')}</Label>
                        <CopyField
                            value={c.callback_url}
                            testId="tp-callback"
                        />
                    </div>
                    {c.verify_token && (
                        <div className="grid gap-1.5">
                            <Label>
                                {t('platform.techProvider.verifyToken')}
                            </Label>
                            <CopyField
                                value={c.verify_token}
                                testId="tp-verify-token"
                            />
                        </div>
                    )}
                    <div className="flex flex-wrap items-center gap-3">
                        <Button
                            type="button"
                            variant="outline"
                            disabled={pending}
                            onClick={() => save({ rotate_verify_token: true })}
                        >
                            {t('platform.techProvider.rotate')}
                        </Button>
                        <span className="text-xs text-muted-foreground">
                            {t('platform.techProvider.unrouted', {
                                count: String(c.unrouted_entries),
                            })}
                            {c.last_unrouted_waba_id &&
                                ` (${c.last_unrouted_waba_id})`}
                        </span>
                    </div>
                </CardContent>
            </Card>

            <Card>
                <CardHeader>
                    <CardTitle>{t('platform.techProvider.test')}</CardTitle>
                    <CardDescription>
                        {t('platform.techProvider.testHint')}
                    </CardDescription>
                </CardHeader>
                <CardContent className="grid gap-4">
                    <div>
                        <Button
                            type="button"
                            disabled={pending}
                            onClick={test}
                            data-testid="tp-test"
                        >
                            {pending ? <Spinner /> : <Stethoscope />}{' '}
                            {t('platform.techProvider.testButton')}
                        </Button>
                    </div>
                    {checks && (
                        <ul
                            className="grid gap-2 text-sm"
                            role="status"
                            data-testid="tp-checks"
                        >
                            {checks.map((ch) => {
                                const Icon = ICON[ch.status];
                                return (
                                    <li key={ch.key} className="flex gap-2">
                                        <Icon
                                            className={cn(
                                                'mt-0.5 size-4 shrink-0',
                                                TONE[ch.status],
                                            )}
                                            aria-label={ch.status}
                                        />
                                        <span>
                                            <span className="font-medium">
                                                {t(
                                                    `platform.techProvider.keys.${ch.key}`,
                                                )}
                                            </span>
                                            <span className="text-muted-foreground">
                                                {' — '}
                                                {text(ch)}
                                            </span>
                                        </span>
                                    </li>
                                );
                            })}
                        </ul>
                    )}
                </CardContent>
            </Card>
        </div>
    );
}
