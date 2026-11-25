'use client';

import { useState, useTransition } from 'react';
import { ShieldBan } from 'lucide-react';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { RadioCard, RadioCards } from '@/components/ui/radio-cards';
import { Separator } from '@/components/ui/separator';
import { Switch } from '@/components/ui/switch';
import { Textarea } from '@/components/ui/textarea';
import { notify } from '@/components/notify';
import { useT } from '@/i18n/client';
import { cleanKeywords } from '@/lib/consent';
import type { Schemas } from '@/lib/types';
import { KeywordsInput } from '../../automations/inputs';
import { saveConsentSettingsAction } from '../actions';
import { Row, Section } from '../section';

type ConsentSettings = Schemas['ConsentSettings'];
type KeywordList = keyof ConsentSettings['keywords'];
type MessageField = 'stop_message' | 'resume_message';

const LISTS: KeywordList[] = ['stop', 'request', 'resume'];
const MAX_KEYWORDS = 50;

function same(a: ConsentSettings, b: ConsentSettings): boolean {
    return JSON.stringify(a) === JSON.stringify(b);
}

/** Settings › Subscription & consent: policy, keywords and confirmations. */
export function ConsentForm({
    settings,
    readOnly,
}: {
    settings: ConsentSettings;
    readOnly: boolean;
}) {
    const t = useT();
    const [saved, setSaved] = useState(settings);
    const [v, setV] = useState(settings);
    const [errors, setErrors] = useState<Record<string, string>>({});
    const [saving, start] = useTransition();
    const dirty = !same(v, saved);

    const save = () =>
        start(async () => {
            const input: ConsentSettings = {
                ...v,
                keywords: {
                    stop: cleanKeywords(v.keywords.stop),
                    request: cleanKeywords(v.keywords.request),
                    resume: cleanKeywords(v.keywords.resume),
                },
            };
            const r = await saveConsentSettingsAction(input);
            setErrors(r.ok ? {} : (r.fieldErrors ?? {}));
            if (notify(r)) {
                setSaved(r.data);
                setV(r.data);
            }
        });

    const setList = (k: KeywordList, list: string[]) =>
        setV((p) => ({ ...p, keywords: { ...p.keywords, [k]: list } }));
    const setMessage = (f: MessageField, lang: 'tr' | 'en', text: string) =>
        setV((p) => ({ ...p, [f]: { ...p[f], [lang]: text } }));
    const fieldError = (f: string) =>
        errors[f] ? (
            <p
                id={`${f}-error`}
                role="alert"
                className="text-sm text-destructive"
            >
                {errors[f]}
            </p>
        ) : null;

    return (
        <Section
            title={t('consent.settings.title')}
            description={t('consent.settings.description')}
            readOnly={readOnly}
            dirty={dirty}
            saving={saving}
            onSave={save}
            onReset={() => {
                setV(saved);
                setErrors({});
            }}
        >
            <Alert data-testid="consent-hard-blocks">
                <ShieldBan />
                <AlertTitle>{t('consent.settings.hardTitle')}</AlertTitle>
                <AlertDescription>
                    {t('consent.settings.hardBody')}
                </AlertDescription>
            </Alert>

            <Row
                label={t('consent.settings.unknownTitle')}
                hint={t('consent.settings.requestNote')}
            >
                <p className="text-sm text-muted-foreground">
                    {t('consent.settings.unknownDescription')}
                </p>
                <RadioCards
                    aria-label={t('consent.settings.unknownTitle')}
                    value={v.unknown_marketing}
                    onValueChange={(x) =>
                        setV((p) => ({
                            ...p,
                            unknown_marketing:
                                x as ConsentSettings['unknown_marketing'],
                        }))
                    }
                    disabled={readOnly}
                    className="sm:grid-cols-2"
                    data-testid="consent-unknown-policy"
                >
                    {(['allow', 'block'] as const).map((k) => (
                        <RadioCard key={k} value={k}>
                            <span className="block font-medium">
                                {t(`consent.settings.unknown.${k}`)}
                            </span>
                            <span className="block text-xs text-muted-foreground">
                                {t(`consent.settings.unknown.${k}Hint`)}
                            </span>
                        </RadioCard>
                    ))}
                </RadioCards>
                {fieldError('unknown_marketing')}
            </Row>

            <Separator />

            <div className="space-y-1">
                <h3 className="font-medium">
                    {t('consent.settings.keywordsTitle')}
                </h3>
                <p className="text-sm text-muted-foreground">
                    {t('consent.settings.keywordsDescription')}
                </p>
            </div>
            <div className="grid gap-5 lg:grid-cols-3">
                {LISTS.map((k) => (
                    <KeywordsInput
                        key={k}
                        id={`consent-keywords-${k}`}
                        label={t(`consent.settings.${k}`)}
                        value={v.keywords[k]}
                        onChange={(list) => setList(k, list)}
                        max={MAX_KEYWORDS}
                        readOnly={readOnly}
                        error={errors[`keywords.${k}`]}
                        hint={t(`consent.settings.${k}Hint`)}
                    />
                ))}
            </div>

            <Separator />

            <div className="flex items-start justify-between gap-4">
                <div className="space-y-1">
                    <label
                        htmlFor="consent-confirm"
                        className="text-sm font-medium"
                    >
                        {t('consent.settings.confirm')}
                    </label>
                    <p className="text-xs text-muted-foreground">
                        {t('consent.settings.confirmHint')}
                    </p>
                </div>
                <Switch
                    id="consent-confirm"
                    checked={v.confirm}
                    disabled={readOnly}
                    onCheckedChange={(confirm) =>
                        setV((p) => ({ ...p, confirm }))
                    }
                />
            </div>
            {v.confirm && (
                <div className="grid gap-5 sm:grid-cols-2">
                    {(['stop_message', 'resume_message'] as const).map((f) => (
                        <fieldset key={f} className="space-y-3">
                            <legend className="text-sm font-medium">
                                {t(
                                    f === 'stop_message'
                                        ? 'consent.settings.stopMessage'
                                        : 'consent.settings.resumeMessage',
                                )}
                            </legend>
                            {(['tr', 'en'] as const).map((lang) => (
                                <div key={lang} className="space-y-1.5">
                                    <label
                                        htmlFor={`${f}-${lang}`}
                                        className="text-xs text-muted-foreground"
                                    >
                                        {t(`consent.settings.${lang}`)}
                                    </label>
                                    <Textarea
                                        id={`${f}-${lang}`}
                                        rows={2}
                                        maxLength={1024}
                                        value={v[f][lang]}
                                        disabled={readOnly}
                                        aria-invalid={
                                            errors[f] ? true : undefined
                                        }
                                        aria-describedby={
                                            errors[f] ? `${f}-error` : undefined
                                        }
                                        onChange={(e) =>
                                            setMessage(f, lang, e.target.value)
                                        }
                                    />
                                </div>
                            ))}
                            {fieldError(f)}
                        </fieldset>
                    ))}
                </div>
            )}
            {!v.confirm &&
                (['stop_message', 'resume_message'] as const).map((f) => (
                    <div key={f}>{fieldError(f)}</div>
                ))}
        </Section>
    );
}
