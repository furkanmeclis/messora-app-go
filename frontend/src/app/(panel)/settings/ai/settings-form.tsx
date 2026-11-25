'use client';

import { useState, useTransition } from 'react';
import { useRouter } from 'next/navigation';
import { ShieldCheck } from 'lucide-react';
import { notify } from '@/components/notify';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { Switch } from '@/components/ui/switch';
import { Textarea } from '@/components/ui/textarea';
import { useT } from '@/i18n/client';
import type { Schemas } from '@/lib/types';
import { Row, Section } from '../section';
import { saveAISettingsAction } from './actions';
import { ProviderModelPicker } from './model-picker';

type Settings = Schemas['AISettings'];

function initial(s: Settings): Schemas['UpdateAISettingsRequest'] {
    return {
        enabled: s.enabled,
        default_provider_id: s.default_provider_id,
        default_model: s.default_model,
        disclosure: { ...s.disclosure },
        data_sharing_notice: { ...s.data_sharing_notice },
    };
}

/** Company AI switch, default model and the texts shown to customers. */
export function AISettingsForm({
    settings,
    providers,
    models,
    readOnly,
}: {
    settings: Settings;
    providers: Schemas['AIProvider'][];
    models: Schemas['AIModel'][];
    readOnly: boolean;
}) {
    const t = useT();
    const router = useRouter();
    const [v, setV] = useState(() => initial(settings));
    const [saving, start] = useTransition();
    const dirty = JSON.stringify(v) !== JSON.stringify(initial(settings));

    const save = () =>
        start(async () => {
            if (notify(await saveAISettingsAction(v))) router.refresh();
        });

    const texts = (
        key: 'disclosure' | 'data_sharing_notice',
        label: string,
        hint: string,
    ) => (
        <Row label={label} hint={hint}>
            <div className="grid gap-3 sm:grid-cols-2">
                {(['tr', 'en'] as const).map((lang) => (
                    <div key={lang} className="space-y-1">
                        <span className="text-xs text-muted-foreground">
                            {lang === 'tr'
                                ? t('ai.settings.turkish')
                                : t('ai.settings.english')}
                        </span>
                        <Textarea
                            aria-label={`${label} (${lang})`}
                            value={v[key][lang]}
                            maxLength={1000}
                            rows={3}
                            disabled={readOnly}
                            onChange={(e) =>
                                setV({
                                    ...v,
                                    [key]: {
                                        ...v[key],
                                        [lang]: e.target.value,
                                    },
                                })
                            }
                        />
                    </div>
                ))}
            </div>
        </Row>
    );

    return (
        <Section
            title={t('ai.settings.title')}
            description={t('ai.settings.description')}
            dirty={dirty}
            saving={saving}
            onSave={save}
            onReset={() => setV(initial(settings))}
            readOnly={readOnly}
        >
            <label className="flex items-start justify-between gap-4">
                <span>
                    <span className="block text-sm font-medium">
                        {t('ai.settings.enabled')}
                    </span>
                    <span className="block text-xs text-muted-foreground">
                        {t('ai.settings.enabledHint')}
                    </span>
                </span>
                <Switch
                    checked={v.enabled}
                    disabled={readOnly}
                    onCheckedChange={(c) => setV({ ...v, enabled: c })}
                    aria-label={t('ai.settings.enabled')}
                />
            </label>
            <Row
                label={t('ai.settings.defaultModel')}
                hint={t('ai.settings.defaultModelHint')}
            >
                <ProviderModelPicker
                    idPrefix="ai-default"
                    providers={providers}
                    models={models}
                    providerId={v.default_provider_id}
                    model={v.default_model}
                    noneLabel={t('ai.settings.none')}
                    disabled={readOnly}
                    onChange={(p, m) =>
                        setV({ ...v, default_provider_id: p, default_model: m })
                    }
                />
            </Row>
            {texts(
                'disclosure',
                t('ai.settings.disclosure'),
                t('ai.settings.disclosureHint'),
            )}
            {texts(
                'data_sharing_notice',
                t('ai.settings.notice'),
                t('ai.settings.noticeHint'),
            )}
            <Alert>
                <ShieldCheck />
                <AlertDescription>{t('ai.settings.training')}</AlertDescription>
            </Alert>
        </Section>
    );
}
