'use client';

import { useState, useTransition } from 'react';
import { CircleCheck, CircleOff, Lock, Send } from 'lucide-react';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { NumberField } from '@/components/ui/number-field';
import { Segmented } from '@/components/ui/segmented';
import { notify } from '@/components/notify';
import { useT } from '@/i18n/client';
import type { Schemas, SmtpSettings } from '@/lib/types';
import { saveSmtpAction, testSmtpAction } from '../actions';
import { Row, Section } from '../section';

type SmtpInput = Schemas['SmtpSettingsInput'];

/** SMTP settings with an inline test. Shared by Settings and the wizard. */
export function SmtpForm({
    smtp,
    readOnly,
    defaultTestTo,
    onSaved,
    bare,
}: {
    smtp: SmtpSettings;
    readOnly: boolean;
    defaultTestTo: string;
    onSaved?: (s: SmtpSettings) => void;
    /** Without the card chrome (inside the wizard). */
    bare?: boolean;
}) {
    const t = useT();
    const [current, setCurrent] = useState(smtp);
    const [v, setV] = useState<SmtpInput>({
        host: smtp.host,
        port: smtp.port,
        username: smtp.username,
        from: smtp.from,
        tls: smtp.tls,
    });
    const [password, setPassword] = useState('');
    const [testTo, setTestTo] = useState(defaultTestTo);
    const [saving, startSave] = useTransition();
    const [testing, startTest] = useTransition();
    const locked = smtp.managed_by_env;
    const disabled = readOnly || locked;
    const input = (): SmtpInput => ({
        ...v,
        port: v.port || 0,
        ...(password ? { password } : {}),
    });
    const dirty =
        password !== '' ||
        v.host !== current.host ||
        v.port !== current.port ||
        v.username !== current.username ||
        v.from !== current.from ||
        v.tls !== current.tls;

    const save = () =>
        startSave(async () => {
            const r = await saveSmtpAction(input());
            if (notify(r)) {
                setCurrent(r.data);
                setPassword('');
                onSaved?.(r.data);
            }
        });
    const test = () =>
        startTest(async () => {
            notify(await testSmtpAction(testTo, dirty ? input() : undefined));
        });

    const body = (
        <>
            <div className="flex flex-wrap items-center gap-2">
                {current.enabled ? (
                    <Badge className="gap-1 bg-success/15 text-success">
                        <CircleCheck className="size-3" />
                        {t('settings.email.enabled')}
                    </Badge>
                ) : (
                    <Badge variant="outline" className="gap-1">
                        <CircleOff className="size-3" />
                        {t('settings.email.disabled')}
                    </Badge>
                )}
            </div>
            {locked && (
                <Alert>
                    <Lock />
                    <AlertDescription>
                        {t('settings.email.managedByEnv')}
                    </AlertDescription>
                </Alert>
            )}
            <div className="grid gap-5 sm:grid-cols-[1fr_9rem]">
                <Row label={t('settings.email.host')} htmlFor="smtp-host">
                    <Input
                        id="smtp-host"
                        value={v.host}
                        placeholder={t('settings.email.hostPlaceholder')}
                        disabled={disabled}
                        autoComplete="off"
                        onChange={(e) => setV({ ...v, host: e.target.value })}
                    />
                </Row>
                <Row label={t('settings.email.port')}>
                    <NumberField
                        value={v.port || null}
                        onValueChange={(n) => setV({ ...v, port: n ?? 0 })}
                        min={0}
                        max={65535}
                        allowEmpty
                        disabled={disabled}
                        placeholder={
                            { starttls: '587', tls: '465', none: '25' }[v.tls]
                        }
                        aria-label={t('settings.email.port')}
                    />
                </Row>
            </div>
            <Row label={t('settings.email.security')}>
                <Segmented
                    value={v.tls}
                    onValueChange={(tls) => setV({ ...v, tls })}
                    disabled={disabled}
                    options={(['starttls', 'tls', 'none'] as const).map(
                        (k) => ({
                            value: k,
                            label: t(`settings.email.tls.${k}`),
                        }),
                    )}
                />
            </Row>
            <div className="grid gap-5 sm:grid-cols-2">
                <Row label={t('settings.email.username')} htmlFor="smtp-user">
                    <Input
                        id="smtp-user"
                        value={v.username}
                        disabled={disabled}
                        autoComplete="off"
                        onChange={(e) =>
                            setV({ ...v, username: e.target.value })
                        }
                    />
                </Row>
                <Row label={t('settings.email.password')} htmlFor="smtp-pass">
                    <Input
                        id="smtp-pass"
                        type="password"
                        value={password}
                        disabled={disabled}
                        autoComplete="new-password"
                        placeholder={
                            current.password_set
                                ? t('settings.email.passwordKeep')
                                : ''
                        }
                        onChange={(e) => setPassword(e.target.value)}
                    />
                </Row>
            </div>
            <Row label={t('settings.email.from')} htmlFor="smtp-from">
                <Input
                    id="smtp-from"
                    value={v.from}
                    placeholder={t('settings.email.fromPlaceholder')}
                    disabled={disabled}
                    onChange={(e) => setV({ ...v, from: e.target.value })}
                />
            </Row>
            {!readOnly && (
                <div className="rounded-xl border border-dashed p-4">
                    <Row label={t('settings.email.testTo')} htmlFor="smtp-test">
                        <div className="flex gap-2">
                            <Input
                                id="smtp-test"
                                type="email"
                                value={testTo}
                                onChange={(e) => setTestTo(e.target.value)}
                            />
                            <Button
                                type="button"
                                variant="secondary"
                                onClick={test}
                                disabled={
                                    testing ||
                                    !testTo ||
                                    (!dirty && !current.enabled) ||
                                    (dirty && !v.host)
                                }
                            >
                                <Send />
                                {t('settings.email.test')}
                            </Button>
                        </div>
                    </Row>
                </div>
            )}
        </>
    );

    if (bare) {
        return (
            <div className="space-y-5">
                {body}
                {!disabled && (
                    <div className="flex justify-end">
                        <Button onClick={save} disabled={!dirty || saving}>
                            {saving ? t('common.saving') : t('common.save')}
                        </Button>
                    </div>
                )}
            </div>
        );
    }
    return (
        <Section
            title={t('settings.email.title')}
            description={t('settings.email.description')}
            readOnly={disabled}
            dirty={dirty}
            saving={saving}
            onSave={save}
        >
            {body}
        </Section>
    );
}
