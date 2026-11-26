'use client';

import { useState, useTransition } from 'react';
import { notify } from '@/components/notify';
import { NumberField } from '@/components/ui/number-field';
import { RadioCard, RadioCards } from '@/components/ui/radio-cards';
import { Separator } from '@/components/ui/separator';
import { Switch } from '@/components/ui/switch';
import { useT } from '@/i18n/client';
import type { Schemas } from '@/lib/types';
import { saveCampaignProtectionAction } from '../actions';
import { Row, Section } from '../section';

type Settings = Schemas['CampaignProtectionSettings'];

/** Settings › Campaign protection: spam thresholds and attribution window. */
export function ProtectionForm({
    settings,
    readOnly,
}: {
    settings: Settings;
    readOnly: boolean;
}) {
    const t = useT();
    const [saved, setSaved] = useState(settings);
    const [v, setV] = useState(settings);
    const [errors, setErrors] = useState<Record<string, string>>({});
    const [saving, start] = useTransition();
    const dirty = JSON.stringify(v) !== JSON.stringify(saved);
    const save = () =>
        start(async () => {
            const r = await saveCampaignProtectionAction(v);
            setErrors(r.ok ? {} : (r.fieldErrors ?? {}));
            if (notify(r)) {
                setSaved(r.data);
                setV(r.data);
            }
        });
    const toggle = (k: keyof Settings, label: string) => (
        <label className="flex items-center justify-between gap-4 text-sm">
            <span>{label}</span>
            <Switch
                checked={v[k] as boolean}
                disabled={readOnly}
                onCheckedChange={(on) => setV((p) => ({ ...p, [k]: on }))}
                data-testid={`protection-${k}`}
            />
        </label>
    );
    const num = (k: keyof Settings, label: string, step = 1) => (
        <Row label={label}>
            <NumberField
                step={step}
                className="w-40"
                value={v[k] as number}
                disabled={readOnly}
                aria-label={label}
                onValueChange={(x) => setV((p) => ({ ...p, [k]: x ?? 0 }))}
                data-testid={`protection-${k}`}
            />
            {errors[k] && (
                <p role="alert" className="text-sm text-destructive">
                    {errors[k]}
                </p>
            )}
        </Row>
    );
    return (
        <Section
            title={t('analytics.settings.title')}
            description={t('analytics.settings.description')}
            readOnly={readOnly}
            dirty={dirty}
            saving={saving}
            onSave={save}
            onReset={() => {
                setV(saved);
                setErrors({});
            }}
        >
            <div className="space-y-3">
                {toggle('enabled', t('analytics.settings.enabled'))}
                {toggle('stop_on_131048', t('analytics.settings.stop131048'))}
                {toggle(
                    'stop_on_account_restriction',
                    t('analytics.settings.stopRestriction'),
                )}
                {toggle(
                    'stop_on_template_paused',
                    t('analytics.settings.templatePaused'),
                )}
            </div>
            <Separator />
            {num('window', t('analytics.settings.window'))}
            {num('min_attempts', t('analytics.settings.minAttempts'))}
            {num(
                'ecosystem_rate_pct',
                t('analytics.settings.ecosystemRate'),
                0.1,
            )}
            {num('opt_out_rate_pct', t('analytics.settings.optOutRate'), 0.1)}
            <Row label={t('analytics.settings.qualityStop')}>
                <RadioCards
                    aria-label={t('analytics.settings.qualityStop')}
                    value={v.quality_stop}
                    onValueChange={(x) =>
                        setV((p) => ({
                            ...p,
                            quality_stop: x as Settings['quality_stop'],
                        }))
                    }
                    disabled={readOnly}
                    className="sm:grid-cols-3"
                >
                    {(['red', 'yellow', 'off'] as const).map((k) => (
                        <RadioCard key={k} value={k}>
                            {t(`analytics.settings.quality.${k}`)}
                        </RadioCard>
                    ))}
                </RadioCards>
            </Row>
            <Separator />
            <Row
                label={t('analytics.settings.attribution')}
                hint={t('analytics.settings.attributionHint')}
            >
                <NumberField
                    className="w-40"
                    min={1}
                    max={30}
                    value={v.attribution_days}
                    disabled={readOnly}
                    aria-label={t('analytics.settings.attribution')}
                    onValueChange={(x) =>
                        setV((p) => ({ ...p, attribution_days: x ?? 7 }))
                    }
                    data-testid="protection-attribution_days"
                />
            </Row>
        </Section>
    );
}
