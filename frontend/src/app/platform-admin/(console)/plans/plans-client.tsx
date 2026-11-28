'use client';

import { useState, useTransition } from 'react';
import { useRouter } from 'next/navigation';
import { Pencil, Plus } from 'lucide-react';
import { notify } from '@/components/notify';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { NumberField } from '@/components/ui/number-field';
import { Switch } from '@/components/ui/switch';
import {
    Table,
    TableBody,
    TableCell,
    TableHead,
    TableHeader,
    TableRow,
} from '@/components/ui/table';
import { Textarea } from '@/components/ui/textarea';
import { useLocale, useT } from '@/i18n/client';
import type { MessageKey } from '@/i18n/translate';
import {
    formatBytes,
    formatPrice,
    PLAN_FEATURES,
    PLAN_LIMIT_KEYS,
} from '@/lib/plan';
import type { Schemas } from '@/lib/types';
import { createPlanAction, updatePlanAction } from '../../plan-actions';

type Plan = Schemas['PlatformPlan'];
type Limits = Schemas['PlanLimits'];
type Features = Schemas['PlanFeatures'];

const EMPTY_LIMITS = Object.fromEntries(
    PLAN_LIMIT_KEYS.map((k) => [k, 0]),
) as Limits;
const EMPTY_FEATURES = Object.fromEntries(
    PLAN_FEATURES.map((f) => [f, false]),
) as Features;

export function PlansClient({ plans }: { plans: Plan[] }) {
    const t = useT();
    const locale = useLocale();
    const [editing, setEditing] = useState<Plan | 'new' | null>(null);
    const num = (n: number) =>
        n === 0
            ? t('plans.unlimited')
            : new Intl.NumberFormat(locale).format(n);
    return (
        <div className="grid gap-4">
            <div className="flex justify-end">
                <Button onClick={() => setEditing('new')}>
                    <Plus /> {t('plans.admin.plans.add')}
                </Button>
            </div>
            <div className="overflow-x-auto rounded-xl border bg-card">
                <Table data-testid="plans-table">
                    <TableHeader>
                        <TableRow>
                            <TableHead>{t('plans.admin.plans.name')}</TableHead>
                            <TableHead className="text-right">
                                {t('plans.admin.plans.tenants')}
                            </TableHead>
                            <TableHead className="text-right">
                                {t('plans.limits.users')}
                            </TableHead>
                            <TableHead className="text-right">
                                {t('plans.limits.phone_numbers')}
                            </TableHead>
                            <TableHead className="text-right">
                                {t('plans.limits.ai_agents')}
                            </TableHead>
                            <TableHead className="text-right">
                                {t('plans.limits.ai_credits_per_month')}
                            </TableHead>
                            <TableHead className="text-right">
                                {t('plans.limits.storage_bytes')}
                            </TableHead>
                            <TableHead>
                                {t('plans.admin.plans.prices')}
                            </TableHead>
                            <TableHead />
                        </TableRow>
                    </TableHeader>
                    <TableBody>
                        {plans.map((p) => (
                            <TableRow
                                key={p.key}
                                data-testid="plan-row"
                                data-plan={p.key}
                            >
                                <TableCell>
                                    <div className="flex flex-wrap items-center gap-2 font-medium">
                                        {locale === 'tr'
                                            ? p.name_tr
                                            : p.name_en}
                                        {!p.active && (
                                            <Badge variant="outline">
                                                {t(
                                                    'plans.admin.plans.inactive',
                                                )}
                                            </Badge>
                                        )}
                                        {!p.public && (
                                            <Badge variant="outline">
                                                {t('plans.admin.plans.hidden')}
                                            </Badge>
                                        )}
                                    </div>
                                    <div className="font-mono text-xs text-muted-foreground">
                                        {p.key}
                                    </div>
                                </TableCell>
                                <TableCell className="text-right tabular-nums">
                                    {p.tenant_count}
                                </TableCell>
                                <TableCell
                                    className="text-right tabular-nums"
                                    data-testid="plan-users"
                                >
                                    {num(p.limits.users)}
                                </TableCell>
                                <TableCell className="text-right tabular-nums">
                                    {num(p.limits.phone_numbers)}
                                </TableCell>
                                <TableCell className="text-right tabular-nums">
                                    {num(p.limits.ai_agents)}
                                </TableCell>
                                <TableCell className="text-right tabular-nums">
                                    {num(p.limits.ai_credits_per_month)}
                                </TableCell>
                                <TableCell className="text-right whitespace-nowrap tabular-nums">
                                    {p.limits.storage_bytes === 0
                                        ? t('plans.unlimited')
                                        : formatBytes(
                                              p.limits.storage_bytes,
                                              locale,
                                          )}
                                </TableCell>
                                <TableCell className="text-sm whitespace-nowrap">
                                    {p.custom_pricing && (
                                        <span className="text-muted-foreground">
                                            {t('plans.customPricing')} ·{' '}
                                        </span>
                                    )}
                                    {formatPrice(
                                        p.price_monthly,
                                        p.currency,
                                        locale,
                                    ) ?? '—'}
                                    {p.price_yearly && (
                                        <div className="text-xs text-muted-foreground">
                                            {t('plans.perYear', {
                                                price:
                                                    formatPrice(
                                                        p.price_yearly,
                                                        p.currency,
                                                        locale,
                                                    ) ?? '',
                                            })}
                                        </div>
                                    )}
                                </TableCell>
                                <TableCell className="text-right">
                                    <Button
                                        size="sm"
                                        variant="outline"
                                        onClick={() => setEditing(p)}
                                        data-testid="plan-edit"
                                    >
                                        <Pencil /> {t('plans.admin.plans.edit')}
                                    </Button>
                                </TableCell>
                            </TableRow>
                        ))}
                    </TableBody>
                </Table>
            </div>
            {editing && (
                <PlanDialog
                    key={editing === 'new' ? 'new' : editing.key}
                    plan={editing === 'new' ? null : editing}
                    onClose={() => setEditing(null)}
                />
            )}
        </div>
    );
}

type Draft = {
    key: string;
    name_tr: string;
    name_en: string;
    description_tr: string;
    description_en: string;
    sort_order: number;
    limits: Limits;
    features: Features;
    currency: string;
    price_monthly: string;
    price_yearly: string;
    custom_pricing: boolean;
    active: boolean;
    public: boolean;
};

function draftOf(p: Plan | null): Draft {
    return {
        key: p?.key ?? '',
        name_tr: p?.name_tr ?? '',
        name_en: p?.name_en ?? '',
        description_tr: p?.description_tr ?? '',
        description_en: p?.description_en ?? '',
        sort_order: p?.sort_order ?? 50,
        limits: { ...EMPTY_LIMITS, ...(p?.limits ?? {}) },
        features: { ...EMPTY_FEATURES, ...(p?.features ?? {}) },
        currency: p?.currency ?? 'TRY',
        price_monthly: p?.price_monthly ?? '',
        price_yearly: p?.price_yearly ?? '',
        custom_pricing: p?.custom_pricing ?? false,
        active: p?.active ?? true,
        public: p?.public ?? true,
    };
}

function PlanDialog({
    plan,
    onClose,
}: {
    plan: Plan | null;
    onClose: () => void;
}) {
    const t = useT();
    const router = useRouter();
    const [d, setD] = useState<Draft>(() => draftOf(plan));
    const [pending, start] = useTransition();
    const set = <K extends keyof Draft>(k: K, v: Draft[K]) =>
        setD((x) => ({ ...x, [k]: v }));
    const submit = () =>
        start(async () => {
            const common = {
                name_tr: d.name_tr.trim(),
                name_en: d.name_en.trim(),
                description_tr: d.description_tr.trim(),
                description_en: d.description_en.trim(),
                sort_order: d.sort_order,
                limits: d.limits,
                features: d.features,
                currency: d.currency.trim().toUpperCase(),
                price_monthly: d.price_monthly.trim() || null,
                price_yearly: d.price_yearly.trim() || null,
                custom_pricing: d.custom_pricing,
                active: d.active,
                public: d.public,
            };
            const r = plan
                ? await updatePlanAction(plan.key, common)
                : await createPlanAction({ key: d.key.trim(), ...common });
            if (notify(r)) {
                onClose();
                router.refresh();
            }
        });
    return (
        <Dialog open onOpenChange={(o) => !o && onClose()}>
            <DialogContent className="max-h-[90vh] overflow-y-auto sm:max-w-3xl">
                <DialogHeader>
                    <DialogTitle>
                        {plan
                            ? `${t('plans.admin.plans.edit')} · ${plan.key}`
                            : t('plans.admin.plans.add')}
                    </DialogTitle>
                    <DialogDescription>
                        {t('plans.admin.plans.description')}
                    </DialogDescription>
                </DialogHeader>
                <form
                    id="plan-form"
                    className="grid gap-6"
                    onSubmit={(e) => {
                        e.preventDefault();
                        submit();
                    }}
                >
                    <div className="grid gap-4 sm:grid-cols-2">
                        {!plan && (
                            <div className="grid gap-2 sm:col-span-2">
                                <Label htmlFor="plan-key">
                                    {t('plans.admin.plans.key')}
                                </Label>
                                <Input
                                    id="plan-key"
                                    value={d.key}
                                    pattern="[a-z][a-z0-9_\-]{1,30}"
                                    required
                                    onChange={(e) => set('key', e.target.value)}
                                    className="font-mono"
                                />
                                <p className="text-xs text-muted-foreground">
                                    {t('plans.admin.plans.keyHint')}
                                </p>
                            </div>
                        )}
                        <div className="grid gap-2">
                            <Label htmlFor="plan-name-tr">
                                {t('plans.admin.plans.nameTr')}
                            </Label>
                            <Input
                                id="plan-name-tr"
                                value={d.name_tr}
                                required
                                maxLength={60}
                                onChange={(e) => set('name_tr', e.target.value)}
                            />
                        </div>
                        <div className="grid gap-2">
                            <Label htmlFor="plan-name-en">
                                {t('plans.admin.plans.nameEn')}
                            </Label>
                            <Input
                                id="plan-name-en"
                                value={d.name_en}
                                required
                                maxLength={60}
                                onChange={(e) => set('name_en', e.target.value)}
                            />
                        </div>
                        <div className="grid gap-2">
                            <Label htmlFor="plan-desc-tr">
                                {t('plans.admin.plans.descriptionTr')}
                            </Label>
                            <Textarea
                                id="plan-desc-tr"
                                value={d.description_tr}
                                maxLength={500}
                                onChange={(e) =>
                                    set('description_tr', e.target.value)
                                }
                            />
                        </div>
                        <div className="grid gap-2">
                            <Label htmlFor="plan-desc-en">
                                {t('plans.admin.plans.descriptionEn')}
                            </Label>
                            <Textarea
                                id="plan-desc-en"
                                value={d.description_en}
                                maxLength={500}
                                onChange={(e) =>
                                    set('description_en', e.target.value)
                                }
                            />
                        </div>
                    </div>

                    <fieldset className="grid gap-3">
                        <legend className="mb-1 text-sm font-semibold">
                            {t('plans.admin.plans.limitsTitle')}{' '}
                            <span className="font-normal text-muted-foreground">
                                · {t('plans.admin.plans.limitsHint')}
                            </span>
                        </legend>
                        <div className="grid gap-3 sm:grid-cols-2">
                            {PLAN_LIMIT_KEYS.map((k) => (
                                <div key={k} className="grid gap-1.5">
                                    <Label
                                        htmlFor={`plan-limit-${k}`}
                                        className="text-xs"
                                    >
                                        {t(`plans.limits.${k}` as MessageKey)}
                                    </Label>
                                    <NumberField
                                        id={`plan-limit-${k}`}
                                        aria-label={t(
                                            `plans.limits.${k}` as MessageKey,
                                        )}
                                        value={d.limits[k]}
                                        min={0}
                                        onValueChange={(v) =>
                                            set('limits', {
                                                ...d.limits,
                                                [k]: v ?? 0,
                                            })
                                        }
                                    />
                                </div>
                            ))}
                        </div>
                    </fieldset>

                    <fieldset className="grid gap-3">
                        <legend className="mb-1 text-sm font-semibold">
                            {t('plans.admin.plans.featuresTitle')}
                        </legend>
                        <div className="grid gap-3 sm:grid-cols-2">
                            {PLAN_FEATURES.map((f) => (
                                <Label
                                    key={f}
                                    className="flex items-center gap-3 font-normal"
                                >
                                    <Switch
                                        checked={d.features[f]}
                                        data-testid={`plan-feature-${f}`}
                                        onCheckedChange={(v) =>
                                            set('features', {
                                                ...d.features,
                                                [f]: v,
                                            })
                                        }
                                    />
                                    {t(`plans.features.${f}.name`)}
                                </Label>
                            ))}
                        </div>
                    </fieldset>

                    <fieldset className="grid gap-3">
                        <legend className="mb-1 text-sm font-semibold">
                            {t('plans.admin.plans.prices')}
                        </legend>
                        <div className="grid gap-3 sm:grid-cols-4">
                            <div className="grid gap-1.5">
                                <Label
                                    htmlFor="plan-price-m"
                                    className="text-xs"
                                >
                                    {t('plans.admin.plans.priceMonthly')}
                                </Label>
                                <Input
                                    id="plan-price-m"
                                    inputMode="decimal"
                                    pattern="[0-9]{1,10}(\.[0-9]{1,2})?"
                                    value={d.price_monthly}
                                    onChange={(e) =>
                                        set('price_monthly', e.target.value)
                                    }
                                />
                            </div>
                            <div className="grid gap-1.5">
                                <Label
                                    htmlFor="plan-price-y"
                                    className="text-xs"
                                >
                                    {t('plans.admin.plans.priceYearly')}
                                </Label>
                                <Input
                                    id="plan-price-y"
                                    inputMode="decimal"
                                    pattern="[0-9]{1,10}(\.[0-9]{1,2})?"
                                    value={d.price_yearly}
                                    onChange={(e) =>
                                        set('price_yearly', e.target.value)
                                    }
                                />
                            </div>
                            <div className="grid gap-1.5">
                                <Label
                                    htmlFor="plan-currency"
                                    className="text-xs"
                                >
                                    {t('plans.admin.plans.currency')}
                                </Label>
                                <Input
                                    id="plan-currency"
                                    value={d.currency}
                                    maxLength={3}
                                    onChange={(e) =>
                                        set('currency', e.target.value)
                                    }
                                />
                            </div>
                            <div className="grid gap-1.5">
                                <Label htmlFor="plan-sort" className="text-xs">
                                    {t('plans.admin.plans.sort')}
                                </Label>
                                <NumberField
                                    id="plan-sort"
                                    aria-label={t('plans.admin.plans.sort')}
                                    value={d.sort_order}
                                    onValueChange={(v) =>
                                        set('sort_order', v ?? 0)
                                    }
                                />
                            </div>
                        </div>
                        <div className="grid gap-3 sm:grid-cols-3">
                            <Label className="flex items-center gap-3 font-normal">
                                <Switch
                                    checked={d.custom_pricing}
                                    onCheckedChange={(v) =>
                                        set('custom_pricing', v)
                                    }
                                />
                                {t('plans.admin.plans.customPricing')}
                            </Label>
                            <Label
                                className="flex items-center gap-3 font-normal"
                                title={t('plans.admin.plans.activeHint')}
                            >
                                <Switch
                                    checked={d.active}
                                    onCheckedChange={(v) => set('active', v)}
                                />
                                {t('plans.admin.plans.active')}
                            </Label>
                            <Label
                                className="flex items-center gap-3 font-normal"
                                title={t('plans.admin.plans.publicHint')}
                            >
                                <Switch
                                    checked={d.public}
                                    onCheckedChange={(v) => set('public', v)}
                                />
                                {t('plans.admin.plans.public')}
                            </Label>
                        </div>
                    </fieldset>
                </form>
                <DialogFooter>
                    <Button variant="outline" onClick={onClose} type="button">
                        {t('common.cancel')}
                    </Button>
                    <Button
                        type="submit"
                        form="plan-form"
                        disabled={pending}
                        data-testid="plan-save"
                    >
                        {t('common.save')}
                    </Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>
    );
}
