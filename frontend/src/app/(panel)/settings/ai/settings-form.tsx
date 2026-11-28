'use client';

import { useState, useTransition } from 'react';
import { useRouter } from 'next/navigation';
import { ShieldCheck } from 'lucide-react';
import { notify } from '@/components/notify';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { Input } from '@/components/ui/input';
import { NumberField } from '@/components/ui/number-field';
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
        max_replies_per_hour: s.max_replies_per_hour,
        context_messages: s.context_messages,
        monthly_budget: s.monthly_budget ?? '',
        classify_campaign_replies: s.classify_campaign_replies,
    };
}

const BUDGET_RE = /^$|^[0-9]{1,8}(\.[0-9]{1,6})?$/;

/** Company AI switch, default model and the texts shown to customers. */
export function AISettingsForm({
    settings,
    providers,
    models,
    readOnly,
    platform,
}: {
    settings: Settings;
    providers: Schemas['AIProvider'][];
    platform?: Schemas['AIPlatformProvider'][];
    models: Schemas['AIModel'][];
    readOnly: boolean;
}) {
    const t = useT();
    const router = useRouter();
    const [v, setV] = useState(() => initial(settings));
    const [saving, start] = useTransition();
    const dirty = JSON.stringify(v) !== JSON.stringify(initial(settings));
    const budgetOK = BUDGET_RE.test(v.monthly_budget ?? '');

    const save = () =>
        start(async () => {
            if (!budgetOK) return;
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
                    platform={platform}
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
            <div className="grid gap-4 sm:grid-cols-2">
                <Row
                    label={t('ai.settings.maxReplies')}
                    hint={t('ai.settings.maxRepliesHint')}
                >
                    <NumberField
                        aria-label={t('ai.settings.maxReplies')}
                        min={1}
                        max={1000}
                        className="w-32"
                        value={v.max_replies_per_hour ?? 1}
                        disabled={readOnly}
                        onValueChange={(n) =>
                            setV({
                                ...v,
                                max_replies_per_hour: Math.min(
                                    1000,
                                    Math.max(1, n ?? 1),
                                ),
                            })
                        }
                    />
                </Row>
                <Row
                    label={t('ai.settings.contextMessages')}
                    hint={t('ai.settings.contextMessagesHint')}
                >
                    <NumberField
                        aria-label={t('ai.settings.contextMessages')}
                        min={2}
                        max={100}
                        className="w-32"
                        value={v.context_messages ?? 2}
                        disabled={readOnly}
                        onValueChange={(n) =>
                            setV({
                                ...v,
                                context_messages: Math.min(
                                    100,
                                    Math.max(2, n ?? 2),
                                ),
                            })
                        }
                    />
                </Row>
            </div>
            <div className="grid gap-4 sm:grid-cols-2">
                <Row
                    label={t('ai.settings.budget')}
                    hint={t('ai.settings.budgetHint')}
                >
                    <div className="space-y-1">
                        <div className="flex items-center gap-2">
                            <Input
                                aria-label={t('ai.settings.budget')}
                                inputMode="decimal"
                                className="w-32"
                                placeholder={t('ai.settings.noBudget')}
                                value={v.monthly_budget ?? ''}
                                disabled={readOnly}
                                aria-invalid={!budgetOK}
                                onChange={(e) =>
                                    setV({
                                        ...v,
                                        monthly_budget: e.target.value
                                            .trim()
                                            .replace(',', '.'),
                                    })
                                }
                            />
                            <span className="text-sm text-muted-foreground">
                                USD
                            </span>
                        </div>
                        {!budgetOK && (
                            <p
                                className="text-xs text-destructive"
                                role="alert"
                            >
                                {t('ai.settings.budgetInvalid')}
                            </p>
                        )}
                    </div>
                </Row>
            </div>
            <label className="flex items-start justify-between gap-4">
                <span>
                    <span className="block text-sm font-medium">
                        {t('ai.settings.classify')}
                    </span>
                    <span className="block text-xs text-muted-foreground">
                        {t('ai.settings.classifyHint')}
                    </span>
                </span>
                <Switch
                    checked={!!v.classify_campaign_replies}
                    disabled={readOnly}
                    onCheckedChange={(c) =>
                        setV({ ...v, classify_campaign_replies: c })
                    }
                    aria-label={t('ai.settings.classify')}
                />
            </label>
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
