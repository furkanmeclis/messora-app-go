'use client';

import { useState, useTransition } from 'react';
import { useRouter } from 'next/navigation';
import { Input } from '@/components/ui/input';
import { notify } from '@/components/notify';
import { useT } from '@/i18n/client';
import type { Locale } from '@/i18n/config';
import type { Settings } from '@/lib/types';
import { saveSettingsAction } from './actions';
import {
    DateFormatField,
    LocaleField,
    TimeFormatField,
    TimezoneField,
} from './fields';
import { LockedBadge } from './locked';
import { Row, Section } from './section';

type Values = Pick<
    Settings,
    | 'organization_name'
    | 'default_locale'
    | 'default_timezone'
    | 'date_format'
    | 'time_format'
>;

const pick = (s: Settings): Values => ({
    organization_name: s.organization_name,
    default_locale: s.default_locale,
    default_timezone: s.default_timezone,
    date_format: s.date_format,
    time_format: s.time_format,
});

export function GeneralForm({
    settings,
    readOnly,
}: {
    settings: Settings;
    readOnly: boolean;
}) {
    const t = useT();
    const router = useRouter();
    const [saved, setSaved] = useState(() => pick(settings));
    const [v, setV] = useState(saved);
    const [saving, start] = useTransition();
    const locked = (f: keyof Values) => settings.locked.includes(f);
    const badge = (f: keyof Values) =>
        locked(f) ? <LockedBadge field={f} /> : null;
    const set = <K extends keyof Values>(k: K, val: Values[K]) =>
        setV((p) => ({ ...p, [k]: val }));

    const changed = (Object.keys(v) as (keyof Values)[]).filter(
        (k) => v[k] !== saved[k] && !locked(k),
    );
    const save = () =>
        start(async () => {
            const patch = Object.fromEntries(changed.map((k) => [k, v[k]]));
            const r = await saveSettingsAction(patch);
            if (notify(r)) {
                setSaved(pick(r.data));
                setV(pick(r.data));
                router.refresh();
            }
        });

    return (
        <div className="grid max-w-3xl gap-6">
            <Section
                title={t('settings.general.organization')}
                readOnly={readOnly}
                dirty={changed.includes('organization_name')}
                saving={saving}
                onSave={save}
                onReset={() =>
                    set('organization_name', saved.organization_name)
                }
            >
                <Row
                    label={t('settings.general.organizationName')}
                    hint={t('settings.general.organizationHint')}
                    badge={badge('organization_name')}
                    htmlFor="organization_name"
                >
                    <Input
                        id="organization_name"
                        value={v.organization_name}
                        maxLength={120}
                        disabled={readOnly || locked('organization_name')}
                        onChange={(e) =>
                            set('organization_name', e.target.value)
                        }
                    />
                </Row>
            </Section>
            <Section
                title={t('settings.general.regional')}
                description={t('settings.general.regionalHint')}
                readOnly={readOnly}
                dirty={changed.some((k) => k !== 'organization_name')}
                saving={saving}
                onSave={save}
                onReset={() =>
                    setV((p) => ({
                        ...saved,
                        organization_name: p.organization_name,
                    }))
                }
            >
                <Row
                    label={t('settings.general.defaultLocale')}
                    badge={badge('default_locale')}
                >
                    <LocaleField
                        value={v.default_locale as Locale}
                        onChange={(l) => set('default_locale', l)}
                        disabled={readOnly || locked('default_locale')}
                    />
                </Row>
                <Row
                    label={t('settings.general.defaultTimezone')}
                    badge={badge('default_timezone')}
                    htmlFor="default_timezone"
                >
                    <TimezoneField
                        id="default_timezone"
                        value={v.default_timezone}
                        onChange={(z) => set('default_timezone', z)}
                        disabled={readOnly || locked('default_timezone')}
                    />
                </Row>
                <Row
                    label={t('settings.general.dateFormat')}
                    badge={badge('date_format')}
                >
                    <DateFormatField
                        value={v.date_format}
                        onChange={(f) => set('date_format', f)}
                        locale={v.default_locale as Locale}
                        disabled={readOnly || locked('date_format')}
                    />
                </Row>
                <Row
                    label={t('settings.general.timeFormat')}
                    badge={badge('time_format')}
                >
                    <TimeFormatField
                        value={v.time_format}
                        onChange={(f) => set('time_format', f)}
                        locale={v.default_locale as Locale}
                        disabled={readOnly || locked('time_format')}
                    />
                </Row>
            </Section>
        </div>
    );
}
