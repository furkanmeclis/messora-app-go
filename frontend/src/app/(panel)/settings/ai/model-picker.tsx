'use client';

import { Input } from '@/components/ui/input';
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from '@/components/ui/select';
import { useT } from '@/i18n/client';
import type { Schemas } from '@/lib/types';

const NONE = '__none__';

/**
 * Picks a provider and one of its catalogue models. Without synced models
 * the model id can be typed.
 */
export function ProviderModelPicker({
    providers,
    models,
    providerId,
    model,
    onChange,
    disabled,
    noneLabel,
    idPrefix,
    platform = [],
}: {
    providers: Schemas['AIProvider'][];
    /** Messora's keys (paid with AI credits); listed after the own keys. */
    platform?: Schemas['AIPlatformProvider'][];
    models: Schemas['AIModel'][];
    providerId: string | null;
    model: string;
    onChange: (providerId: string | null, model: string) => void;
    disabled?: boolean;
    /** Label of the "no provider" choice (e.g. company default). */
    noneLabel: string;
    idPrefix: string;
}) {
    const t = useT();
    const own = models.filter((m) => m.provider_id === providerId);
    return (
        <div className="grid gap-3 sm:grid-cols-2">
            <Select
                value={providerId ?? NONE}
                onValueChange={(v) => onChange(v === NONE ? null : v, '')}
                disabled={disabled}
            >
                <SelectTrigger
                    id={`${idPrefix}-provider`}
                    aria-label={t('ai.profiles.provider')}
                    className="w-full"
                >
                    <SelectValue
                        placeholder={t('ai.settings.chooseProvider')}
                    />
                </SelectTrigger>
                <SelectContent>
                    <SelectItem value={NONE}>{noneLabel}</SelectItem>
                    {providers.map((p) => (
                        <SelectItem
                            key={p.id}
                            value={p.id}
                            disabled={!p.enabled}
                        >
                            {p.name} · {t(`ai.kinds.${p.kind}`)}
                            {platform.length > 0 &&
                                ` (${t('plans.ai.sourceOwn')})`}
                        </SelectItem>
                    ))}
                    {platform.map((p) => (
                        <SelectItem
                            key={p.id}
                            value={p.id}
                            disabled={!p.enabled}
                            data-source="platform"
                        >
                            {t(`ai.kinds.${p.kind}`)} · {p.name} (
                            {t('plans.ai.sourcePlatform')})
                        </SelectItem>
                    ))}
                </SelectContent>
            </Select>
            {providerId && own.length > 0 ? (
                <Select
                    value={model || undefined}
                    onValueChange={(v) => onChange(providerId, v)}
                    disabled={disabled}
                >
                    <SelectTrigger
                        id={`${idPrefix}-model`}
                        aria-label={t('ai.profiles.model')}
                        className="w-full"
                    >
                        <SelectValue
                            placeholder={t('ai.settings.chooseModel')}
                        />
                    </SelectTrigger>
                    <SelectContent>
                        {own.map((m) => (
                            <SelectItem key={m.model_id} value={m.model_id}>
                                {m.display_name !== m.model_id
                                    ? `${m.display_name} (${m.model_id})`
                                    : m.model_id}
                            </SelectItem>
                        ))}
                    </SelectContent>
                </Select>
            ) : (
                <Input
                    id={`${idPrefix}-model`}
                    aria-label={t('ai.profiles.model')}
                    value={model}
                    onChange={(e) => onChange(providerId, e.target.value)}
                    placeholder={
                        providerId
                            ? t('ai.settings.noModels')
                            : t('ai.settings.chooseProvider')
                    }
                    disabled={disabled || !providerId}
                    className="font-mono text-sm"
                />
            )}
        </div>
    );
}
