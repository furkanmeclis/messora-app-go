'use client';

import { useState, useTransition } from 'react';
import { notify } from '@/components/notify';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from '@/components/ui/dialog';
import { Field, FieldDescription, FieldLabel } from '@/components/ui/field';
import { DatePicker } from '@/components/ui/date-picker';
import { Input } from '@/components/ui/input';
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from '@/components/ui/select';
import { Spinner } from '@/components/ui/spinner';
import { Textarea } from '@/components/ui/textarea';
import { useT } from '@/i18n/client';
import type { Schemas } from '@/lib/types';
import { createPriceAction, updatePriceAction } from '../actions';

type Price = Schemas['AIPrice'];
type Kind = Schemas['AIProviderKind'];

const RATES = [
    'input',
    'cached_input',
    'cache_write_5m',
    'cache_write_1h',
    'output',
] as const;
type Rate = (typeof RATES)[number];

const LABEL: Record<
    Rate,
    'input' | 'cachedInput' | 'cacheWrite5m' | 'cacheWrite1h' | 'output'
> = {
    input: 'input',
    cached_input: 'cachedInput',
    cache_write_5m: 'cacheWrite5m',
    cache_write_1h: 'cacheWrite1h',
    output: 'output',
};

/** Adds or edits (and confirms) one price row. */
export function PriceDialog({
    price,
    model,
    fakeAvailable,
    onClose,
    onDone,
}: {
    price?: Price;
    model?: Schemas['AIModel'];
    fakeAvailable: boolean;
    onClose: () => void;
    onDone: () => void;
}) {
    const t = useT();
    const today = new Date().toISOString().slice(0, 10);
    const [kind, setKind] = useState<Kind>(
        price?.provider_kind ?? model?.provider_kind ?? 'openai',
    );
    const [pattern, setPattern] = useState(
        price?.model_pattern ?? model?.model_id ?? '',
    );
    const [currency, setCurrency] = useState(price?.currency ?? 'USD');
    const [rates, setRates] = useState<Record<Rate, string>>({
        input: price?.input ?? '',
        cached_input: price?.cached_input ?? '',
        cache_write_5m: price?.cache_write_5m ?? '',
        cache_write_1h: price?.cache_write_1h ?? '',
        output: price?.output ?? '',
    });
    const [from, setFrom] = useState(price?.effective_from ?? today);
    const [to, setTo] = useState(price?.effective_to ?? '');
    const [source, setSource] = useState(price?.source_url ?? '');
    const [notes, setNotes] = useState(price?.notes ?? '');
    const [rules, setRules] = useState(
        price && (price.rules.thresholds?.length || price.rules.windows?.length)
            ? JSON.stringify(price.rules, null, 2)
            : '',
    );
    const [confirm, setConfirm] = useState(false);
    const [errors, setErrors] = useState<Record<string, string>>({});
    const [pending, start] = useTransition();
    const kinds: Kind[] = ['openai', 'anthropic', 'gemini', 'deepseek'];
    if (fakeAvailable) kinds.push('fake');

    const submit = (e: React.FormEvent) => {
        e.preventDefault();
        let parsed: Schemas['AIPriceRules'] = {};
        if (rules.trim()) {
            try {
                parsed = JSON.parse(rules) as Schemas['AIPriceRules'];
                if (typeof parsed !== 'object' || Array.isArray(parsed))
                    throw new Error('not an object');
            } catch {
                setErrors({ rules: t('ai.prices.rulesInvalid') });
                return;
            }
        }
        const opt = (v: string) => (v.trim() ? v.trim() : null);
        start(async () => {
            const common = {
                model_pattern: pattern.trim(),
                currency: currency.trim().toUpperCase(),
                input: rates.input.trim(),
                cached_input: opt(rates.cached_input),
                cache_write_5m: opt(rates.cache_write_5m),
                cache_write_1h: opt(rates.cache_write_1h),
                output: rates.output.trim(),
                rules: parsed,
                effective_from: from,
                effective_to: to || null,
                source_url: source.trim(),
                notes: notes.trim(),
            };
            const r = price
                ? await updatePriceAction(price.id, { ...common, confirm })
                : await createPriceAction({ ...common, provider_kind: kind });
            if (notify(r)) onDone();
            else if (!r.ok) setErrors(r.fieldErrors ?? {});
        });
    };

    const err = (k: string) =>
        errors[k] ? (
            <FieldDescription className="text-destructive">
                {errors[k]}
            </FieldDescription>
        ) : null;

    return (
        <Dialog open onOpenChange={(o) => !o && onClose()}>
            <DialogContent className="max-h-[90svh] overflow-y-auto sm:max-w-2xl">
                <DialogHeader>
                    <DialogTitle>
                        {price
                            ? t('ai.models.editPrice')
                            : t('ai.models.addPrice')}
                    </DialogTitle>
                    <DialogDescription>
                        {t('ai.prices.perMillion')}
                    </DialogDescription>
                </DialogHeader>
                <form onSubmit={submit} className="space-y-4">
                    <div className="grid gap-4 sm:grid-cols-2">
                        <Field>
                            <FieldLabel htmlFor="ai-price-kind">
                                {t('ai.providers.kind')}
                            </FieldLabel>
                            <Select
                                value={kind}
                                onValueChange={(v) => setKind(v as Kind)}
                                disabled={!!price}
                            >
                                <SelectTrigger
                                    id="ai-price-kind"
                                    className="w-full"
                                >
                                    <SelectValue />
                                </SelectTrigger>
                                <SelectContent>
                                    {kinds.map((k) => (
                                        <SelectItem key={k} value={k}>
                                            {t(`ai.kinds.${k}`)}
                                        </SelectItem>
                                    ))}
                                </SelectContent>
                            </Select>
                        </Field>
                        <Field data-invalid={!!errors.model_pattern}>
                            <FieldLabel htmlFor="ai-price-pattern">
                                {t('ai.prices.pattern')}
                            </FieldLabel>
                            <Input
                                id="ai-price-pattern"
                                required
                                value={pattern}
                                onChange={(e) => setPattern(e.target.value)}
                                className="font-mono"
                            />
                            {err('model_pattern') ?? (
                                <FieldDescription>
                                    {t('ai.prices.patternHint')}
                                </FieldDescription>
                            )}
                        </Field>
                    </div>
                    <div className="grid gap-4 sm:grid-cols-3">
                        {RATES.map((r) => (
                            <Field key={r} data-invalid={!!errors[r]}>
                                <FieldLabel htmlFor={`ai-price-${r}`}>
                                    {t(`ai.prices.${LABEL[r]}`)}
                                </FieldLabel>
                                <Input
                                    id={`ai-price-${r}`}
                                    inputMode="decimal"
                                    pattern="[0-9]+(\.[0-9]+)?"
                                    required={r === 'input' || r === 'output'}
                                    value={rates[r]}
                                    onChange={(e) =>
                                        setRates({
                                            ...rates,
                                            [r]: e.target.value,
                                        })
                                    }
                                    className="font-mono"
                                />
                                {err(r)}
                            </Field>
                        ))}
                        <Field data-invalid={!!errors.currency}>
                            <FieldLabel htmlFor="ai-price-currency">
                                {t('ai.prices.currency')}
                            </FieldLabel>
                            <Input
                                id="ai-price-currency"
                                maxLength={3}
                                value={currency}
                                onChange={(e) => setCurrency(e.target.value)}
                                className="font-mono uppercase"
                            />
                            {err('currency')}
                        </Field>
                    </div>
                    <div className="grid gap-4 sm:grid-cols-2">
                        <Field data-invalid={!!errors.effective_from}>
                            <FieldLabel htmlFor="ai-price-from">
                                {t('ai.prices.effectiveFrom')}
                            </FieldLabel>
                            <DatePicker
                                id="ai-price-from"
                                value={from}
                                onValueChange={setFrom}
                            />
                            {err('effective_from')}
                        </Field>
                        <Field data-invalid={!!errors.effective_to}>
                            <FieldLabel htmlFor="ai-price-to">
                                {t('ai.prices.effectiveTo')}
                            </FieldLabel>
                            <DatePicker
                                id="ai-price-to"
                                value={to}
                                onValueChange={setTo}
                            />
                            {err('effective_to')}
                        </Field>
                    </div>
                    <Field>
                        <FieldLabel htmlFor="ai-price-source">
                            {t('ai.prices.sourceUrl')}
                        </FieldLabel>
                        <Input
                            id="ai-price-source"
                            value={source}
                            maxLength={500}
                            onChange={(e) => setSource(e.target.value)}
                        />
                    </Field>
                    <Field>
                        <FieldLabel htmlFor="ai-price-notes">
                            {t('ai.prices.notes')}
                        </FieldLabel>
                        <Textarea
                            id="ai-price-notes"
                            rows={2}
                            maxLength={2000}
                            value={notes}
                            onChange={(e) => setNotes(e.target.value)}
                        />
                    </Field>
                    <Field data-invalid={!!errors.rules}>
                        <FieldLabel htmlFor="ai-price-rules">
                            {t('ai.prices.rules')}
                        </FieldLabel>
                        <Textarea
                            id="ai-price-rules"
                            rows={4}
                            value={rules}
                            onChange={(e) => setRules(e.target.value)}
                            className="font-mono text-xs"
                            placeholder="{}"
                        />
                        {err('rules') ?? (
                            <FieldDescription className="font-mono text-[11px]">
                                {t('ai.prices.rulesHint')}
                            </FieldDescription>
                        )}
                    </Field>
                    {price && (
                        <label className="flex items-center gap-2 text-sm">
                            <Checkbox
                                checked={confirm}
                                onCheckedChange={(c) => setConfirm(c === true)}
                            />
                            {t('ai.prices.confirm')}
                        </label>
                    )}
                    <DialogFooter>
                        <Button type="submit" disabled={pending}>
                            {pending && <Spinner />}
                            {t('common.save')}
                        </Button>
                    </DialogFooter>
                </form>
            </DialogContent>
        </Dialog>
    );
}
