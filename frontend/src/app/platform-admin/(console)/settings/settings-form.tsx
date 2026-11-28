'use client';

import { useState, useTransition } from 'react';
import { useRouter } from 'next/navigation';
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
import { Switch } from '@/components/ui/switch';
import { useT } from '@/i18n/client';
import type { Schemas } from '@/lib/types';
import { updatePlatformSettingsAction } from '../../actions';

function Value({ label, value }: { label: string; value: React.ReactNode }) {
    return (
        <div className="grid gap-0.5">
            <dt className="text-xs text-muted-foreground">{label}</dt>
            <dd className="font-mono text-sm break-all">{value}</dd>
        </div>
    );
}

export function SettingsForm({
    settings: s,
}: {
    settings: Schemas['PlatformSettings'];
}) {
    const t = useT();
    const router = useRouter();
    const [pending, start] = useTransition();
    const [replyTo, setReplyTo] = useState(s.email.reply_to ?? '');
    const save = (body: Schemas['UpdatePlatformSettingsRequest']) =>
        start(async () => {
            if (notify(await updatePlatformSettingsAction(body)))
                router.refresh();
        });
    const notSet = t('platform.settings.notSet');
    return (
        <div className="grid max-w-3xl gap-6">
            <Card>
                <CardHeader>
                    <CardTitle>{t('platform.settings.signup')}</CardTitle>
                    <CardDescription>
                        {t('platform.settings.signupHint')}
                    </CardDescription>
                </CardHeader>
                <CardContent>
                    <Label className="flex items-center gap-3 font-normal">
                        <Switch
                            data-testid="signup-enabled"
                            checked={s.signup_enabled}
                            disabled={pending}
                            onCheckedChange={(v) => save({ signup_enabled: v })}
                        />
                        {s.signup_enabled ? t('common.on') : t('common.off')}
                    </Label>
                </CardContent>
            </Card>
            <Card>
                <CardHeader>
                    <CardTitle>{t('platform.settings.addresses')}</CardTitle>
                </CardHeader>
                <CardContent>
                    <dl className="grid gap-4 sm:grid-cols-3">
                        <Value
                            label={t('platform.settings.baseDomain')}
                            value={s.base_domain || notSet}
                        />
                        <Value
                            label={t('platform.settings.adminUrl')}
                            value={s.admin_url ?? notSet}
                        />
                        <Value
                            label={t('platform.settings.platformUrl')}
                            value={s.platform_url ?? notSet}
                        />
                    </dl>
                </CardContent>
            </Card>
            <Card>
                <CardHeader>
                    <CardTitle className="flex items-center gap-2">
                        {t('platform.settings.email')}
                        <Badge
                            variant="outline"
                            className={
                                s.email.configured
                                    ? 'border-success/40 bg-success/10 text-success'
                                    : 'border-warning/40 bg-warning/10 text-warning'
                            }
                        >
                            {s.email.configured
                                ? t('platform.settings.configured')
                                : t('platform.settings.notConfigured')}
                        </Badge>
                    </CardTitle>
                    <CardDescription>
                        {t('platform.settings.emailHint')}
                    </CardDescription>
                </CardHeader>
                <CardContent className="grid gap-5">
                    <dl className="grid gap-4 sm:grid-cols-2">
                        <Value
                            label={t('platform.settings.from')}
                            value={s.email.from || notSet}
                        />
                        <Value
                            label={t('platform.settings.host')}
                            value={s.email.host || notSet}
                        />
                    </dl>
                    <form
                        className="grid gap-2"
                        onSubmit={(e) => {
                            e.preventDefault();
                            save({ reply_to: replyTo.trim() || null });
                        }}
                    >
                        <Label htmlFor="reply-to">
                            {t('platform.settings.replyTo')}
                        </Label>
                        <div className="flex gap-2">
                            <Input
                                id="reply-to"
                                type="email"
                                value={replyTo}
                                onChange={(e) => setReplyTo(e.target.value)}
                                className="max-w-sm"
                            />
                            <Button
                                type="submit"
                                variant="outline"
                                disabled={pending}
                            >
                                {t('common.save')}
                            </Button>
                        </div>
                        <p className="text-xs text-muted-foreground">
                            {t('platform.settings.replyToHint')}
                        </p>
                    </form>
                </CardContent>
            </Card>
        </div>
    );
}
