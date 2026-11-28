'use client';

import { useState, useTransition } from 'react';
import { AlertTriangle, Mail } from 'lucide-react';
import { useFormatDateTime } from '@/components/display-preferences';
import { notify } from '@/components/notify';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { Badge } from '@/components/ui/badge';
import { Input } from '@/components/ui/input';
import { useT } from '@/i18n/client';
import type { Schemas } from '@/lib/types';
import { saveEmailSettingsAction } from '../actions';
import { Row, Section } from '../section';

/** Messora's sender: address, the company's name, its reply-to address. */
export function EmailSender({
    email,
    readOnly,
}: {
    email: Schemas['EmailSettings'];
    readOnly: boolean;
}) {
    const t = useT();
    const fmt = useFormatDateTime();
    const [saved, setSaved] = useState(email.reply_to ?? '');
    const [replyTo, setReplyTo] = useState(saved);
    const [saving, start] = useTransition();
    const save = () =>
        start(async () => {
            const r = await saveEmailSettingsAction(replyTo.trim() || null);
            if (notify(r)) {
                setSaved(r.data.reply_to ?? '');
                setReplyTo(r.data.reply_to ?? '');
            }
        });
    return (
        <Section
            title={
                <span className="flex flex-wrap items-center gap-2">
                    {t('plans.email.senderTitle')}
                    <Badge
                        variant="outline"
                        data-testid="email-mode"
                        data-mode={email.mode}
                    >
                        {t(`plans.email.mode.${email.mode}`)}
                    </Badge>
                </span>
            }
            description={t('plans.email.senderHint')}
            readOnly={readOnly}
            dirty={replyTo.trim() !== saved}
            saving={saving}
            onSave={save}
            onReset={() => setReplyTo(saved)}
        >
            {email.last_failure && (
                <Alert variant="destructive" data-testid="smtp-last-failure">
                    <AlertTriangle />
                    <AlertDescription>
                        {t('plans.email.lastFailure', {
                            date: fmt(email.last_failure.at),
                            error: email.last_failure.error,
                        })}
                    </AlertDescription>
                </Alert>
            )}
            {!email.sender.configured && (
                <Alert>
                    <Mail />
                    <AlertDescription>
                        {t('plans.email.notConfigured')}
                    </AlertDescription>
                </Alert>
            )}
            <dl className="grid gap-4 sm:grid-cols-2">
                <div className="grid gap-0.5">
                    <dt className="text-xs text-muted-foreground">
                        {t('plans.email.displayName')}
                    </dt>
                    <dd className="text-sm font-medium">
                        {email.display_name}
                    </dd>
                </div>
                <div className="grid gap-0.5">
                    <dt className="text-xs text-muted-foreground">
                        {t('plans.email.from')}
                    </dt>
                    <dd className="font-mono text-sm break-all">
                        {email.sender.from || '—'}
                    </dd>
                </div>
            </dl>
            <Row
                label={t('plans.email.replyTo')}
                htmlFor="email-reply-to"
                hint={t('plans.email.replyToHint')}
            >
                <Input
                    id="email-reply-to"
                    type="email"
                    value={replyTo}
                    disabled={readOnly}
                    className="max-w-sm"
                    onChange={(e) => setReplyTo(e.target.value)}
                />
            </Row>
        </Section>
    );
}
