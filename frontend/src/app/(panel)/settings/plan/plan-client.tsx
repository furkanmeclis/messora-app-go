'use client';

import { useState, useTransition } from 'react';
import { useRouter } from 'next/navigation';
import { Check, Clock, Gem, Minus } from 'lucide-react';
import { useFormatDate } from '@/components/display-preferences';
import { notify } from '@/components/notify';
import {
    PlanFeatureList,
    PlanUsage,
    usePlanName,
} from '@/components/plan-usage';
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
import { Label } from '@/components/ui/label';
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from '@/components/ui/select';
import { Textarea } from '@/components/ui/textarea';
import { useLocale, useT } from '@/i18n/client';
import type { MessageKey } from '@/i18n/translate';
import {
    formatBytes,
    formatCredits,
    formatPrice,
    PLAN_FEATURES,
} from '@/lib/plan';
import type { Schemas } from '@/lib/types';
import { cn } from '@/lib/utils';
import { Section } from '../section';
import { requestPlanChangeAction } from './actions';

type Overview = Schemas['PlanOverview'];
type Plan = Schemas['CatalogPlan'];

const COMPARE: (keyof Schemas['PlanLimits'])[] = [
    'users',
    'phone_numbers',
    'ai_agents',
    'automation_flows',
    'ai_credits_per_month',
    'api_calls_per_month',
    'storage_bytes',
    'webhook_endpoints',
];

export function PlanPage({
    overview: o,
    plans,
    canRequest,
}: {
    overview: Overview;
    plans: Plan[];
    canRequest: boolean;
}) {
    const t = useT();
    const fmtDate = useFormatDate();
    const planName = usePlanName();
    const [open, setOpen] = useState(false);
    const req = o.open_request;
    const nameOf = (key: string) => {
        const p = plans.find((x) => x.key === key);
        return p ? planName(p) : key;
    };
    return (
        <div className="grid gap-6" data-testid="plan-page">
            <section className="flex flex-wrap items-center justify-between gap-4 rounded-2xl border bg-linear-to-br from-primary/10 to-transparent p-5">
                <div className="flex items-center gap-4">
                    <span className="flex size-12 items-center justify-center rounded-2xl bg-primary/15 text-primary">
                        <Gem className="size-6" />
                    </span>
                    <div className="grid gap-1">
                        <span className="text-xs text-muted-foreground">
                            {t('plans.current')}
                        </span>
                        <h2
                            className="text-xl font-semibold tracking-tight"
                            data-testid="plan-name"
                            data-plan={o.key}
                        >
                            {planName(o)}
                        </h2>
                        <div className="flex flex-wrap items-center gap-2 text-sm">
                            <Badge variant="outline">
                                {t(
                                    `plans.status.${o.subscription_status}` as MessageKey,
                                )}
                            </Badge>
                            {o.subscription_status === 'trial' &&
                                o.trial_ends_at && (
                                    <span className="text-muted-foreground">
                                        {t('plans.trialUntil', {
                                            date: fmtDate(o.trial_ends_at),
                                        })}
                                    </span>
                                )}
                            {o.overridden && (
                                <Badge
                                    variant="outline"
                                    title={t('plans.overriddenHint')}
                                    className="border-primary/40 text-primary"
                                >
                                    {t('plans.overridden')}
                                </Badge>
                            )}
                        </div>
                    </div>
                </div>
                {req ? (
                    <p
                        className="flex max-w-md items-start gap-2 text-sm text-muted-foreground"
                        data-testid="plan-request-open"
                    >
                        <Clock className="mt-0.5 size-4 shrink-0" />
                        {t('plans.change.open', {
                            plan: nameOf(req.requested_plan),
                            cycle:
                                req.billing_cycle === 'yearly'
                                    ? t('plans.yearly')
                                    : t('plans.monthly'),
                            date: fmtDate(req.created_at),
                        })}
                    </p>
                ) : (
                    canRequest && (
                        <Button
                            onClick={() => setOpen(true)}
                            data-testid="plan-change"
                        >
                            {t('plans.change.button')}
                        </Button>
                    )
                )}
            </section>

            <Section
                title={t('plans.usageTitle')}
                description={t('plans.usageHint')}
            >
                <PlanUsage overview={o} />
            </Section>

            <Section title={t('plans.featuresTitle')}>
                <PlanFeatureList features={o.features} />
            </Section>

            {plans.length > 0 && (
                <Section
                    title={t('plans.compareTitle')}
                    description={t('plans.compareHint')}
                >
                    <Compare plans={plans} current={o.key} />
                </Section>
            )}

            {canRequest && (
                <ChangeDialog
                    open={open}
                    onOpenChange={setOpen}
                    plans={plans}
                    current={o.key}
                />
            )}
        </div>
    );
}

function Compare({ plans, current }: { plans: Plan[]; current: string }) {
    const t = useT();
    const locale = useLocale();
    const planName = usePlanName();
    const num = (n: number) => new Intl.NumberFormat(locale).format(n);
    const value = (k: keyof Schemas['PlanLimits'], v: number) => {
        if (v === 0) return t('plans.unlimited');
        if (k === 'storage_bytes') return formatBytes(v, locale);
        if (k === 'ai_credits_per_month') return formatCredits(v, locale);
        return num(v);
    };
    return (
        <div className="-mx-5 overflow-x-auto px-5">
            <table
                className="w-full min-w-[40rem] text-sm"
                data-testid="plan-compare"
            >
                <thead>
                    <tr className="border-b">
                        <th className="py-3 pr-4 text-left font-medium text-muted-foreground" />
                        {plans.map((p) => {
                            const isCurrent = p.key === current || p.current;
                            const monthly = formatPrice(
                                p.price_monthly,
                                p.currency,
                                locale,
                            );
                            const yearly = formatPrice(
                                p.price_yearly,
                                p.currency,
                                locale,
                            );
                            return (
                                <th
                                    key={p.key}
                                    data-plan={p.key}
                                    className={cn(
                                        'px-3 py-3 text-left align-top font-normal',
                                        isCurrent && 'bg-primary/5',
                                    )}
                                >
                                    <div className="flex items-center gap-2 font-semibold">
                                        {planName(p)}
                                        {isCurrent && (
                                            <Badge className="text-[10px]">
                                                {t('plans.yourPlan')}
                                            </Badge>
                                        )}
                                    </div>
                                    <div className="mt-1 text-xs text-muted-foreground">
                                        {p.custom_pricing ? (
                                            <>
                                                {t('plans.customPricing')}
                                                {monthly && (
                                                    <>
                                                        {' '}
                                                        ·{' '}
                                                        {t('plans.fromPrice', {
                                                            price: monthly,
                                                        })}
                                                    </>
                                                )}
                                            </>
                                        ) : monthly ? (
                                            <>
                                                <span className="text-base font-semibold text-foreground">
                                                    {t('plans.perMonth', {
                                                        price: monthly,
                                                    })}
                                                </span>{' '}
                                                {t('plans.plusVat')}
                                                {yearly && (
                                                    <div>
                                                        {t('plans.perYear', {
                                                            price: yearly,
                                                        })}
                                                    </div>
                                                )}
                                            </>
                                        ) : null}
                                    </div>
                                </th>
                            );
                        })}
                    </tr>
                </thead>
                <tbody>
                    {COMPARE.map((k) => (
                        <tr key={k} className="border-b last:border-0">
                            <td className="py-2.5 pr-4 text-muted-foreground">
                                {t(`plans.limits.${k}` as MessageKey)}
                            </td>
                            {plans.map((p) => (
                                <td
                                    key={p.key}
                                    className={cn(
                                        'px-3 py-2.5 tabular-nums',
                                        (p.key === current || p.current) &&
                                            'bg-primary/5',
                                    )}
                                >
                                    {k === 'api_calls_per_month' &&
                                    !p.features.public_api ? (
                                        <Minus className="size-4 text-muted-foreground" />
                                    ) : (
                                        value(k, p.limits[k])
                                    )}
                                </td>
                            ))}
                        </tr>
                    ))}
                    {PLAN_FEATURES.map((f) => (
                        <tr key={f} className="border-b last:border-0">
                            <td className="py-2.5 pr-4 text-muted-foreground">
                                {t(`plans.features.${f}.name`)}
                            </td>
                            {plans.map((p) => (
                                <td
                                    key={p.key}
                                    className={cn(
                                        'px-3 py-2.5',
                                        (p.key === current || p.current) &&
                                            'bg-primary/5',
                                    )}
                                >
                                    {p.features[f] ? (
                                        <Check
                                            className="size-4 text-success"
                                            aria-label={t('plans.included')}
                                        />
                                    ) : (
                                        <Minus
                                            className="size-4 text-muted-foreground"
                                            aria-label={t('plans.notIncluded')}
                                        />
                                    )}
                                </td>
                            ))}
                        </tr>
                    ))}
                </tbody>
            </table>
        </div>
    );
}

function ChangeDialog({
    open,
    onOpenChange,
    plans,
    current,
}: {
    open: boolean;
    onOpenChange: (v: boolean) => void;
    plans: Plan[];
    current: string;
}) {
    const t = useT();
    const router = useRouter();
    const planName = usePlanName();
    const others = plans.filter((p) => p.key !== current);
    const [plan, setPlan] = useState(others[0]?.key ?? '');
    const [cycle, setCycle] = useState<'monthly' | 'yearly'>('monthly');
    const [message, setMessage] = useState('');
    const [pending, start] = useTransition();
    const submit = () =>
        start(async () => {
            const r = await requestPlanChangeAction({
                plan_key: plan,
                billing_cycle: cycle,
                message: message.trim(),
            });
            if (notify(r)) {
                onOpenChange(false);
                router.refresh();
            }
        });
    return (
        <Dialog open={open} onOpenChange={onOpenChange}>
            <DialogContent>
                <DialogHeader>
                    <DialogTitle>{t('plans.change.title')}</DialogTitle>
                    <DialogDescription>
                        {t('plans.change.description')}
                    </DialogDescription>
                </DialogHeader>
                <form
                    id="plan-change-form"
                    className="grid gap-4"
                    onSubmit={(e) => {
                        e.preventDefault();
                        submit();
                    }}
                >
                    <div className="grid gap-2">
                        <Label htmlFor="plan-change-plan">
                            {t('plans.change.plan')}
                        </Label>
                        <Select value={plan} onValueChange={setPlan}>
                            <SelectTrigger
                                id="plan-change-plan"
                                data-testid="plan-change-plan"
                            >
                                <SelectValue />
                            </SelectTrigger>
                            <SelectContent>
                                {others.map((p) => (
                                    <SelectItem key={p.key} value={p.key}>
                                        {planName(p)}
                                    </SelectItem>
                                ))}
                            </SelectContent>
                        </Select>
                    </div>
                    <div className="grid gap-2">
                        <Label htmlFor="plan-change-cycle">
                            {t('plans.change.cycle')}
                        </Label>
                        <Select
                            value={cycle}
                            onValueChange={(v) =>
                                setCycle(v as 'monthly' | 'yearly')
                            }
                        >
                            <SelectTrigger id="plan-change-cycle">
                                <SelectValue />
                            </SelectTrigger>
                            <SelectContent>
                                <SelectItem value="monthly">
                                    {t('plans.monthly')}
                                </SelectItem>
                                <SelectItem value="yearly">
                                    {t('plans.yearly')}
                                </SelectItem>
                            </SelectContent>
                        </Select>
                    </div>
                    <div className="grid gap-2">
                        <Label htmlFor="plan-change-message">
                            {t('plans.change.message')}
                        </Label>
                        <Textarea
                            id="plan-change-message"
                            value={message}
                            maxLength={2000}
                            placeholder={t('plans.change.messagePlaceholder')}
                            onChange={(e) => setMessage(e.target.value)}
                        />
                    </div>
                </form>
                <DialogFooter>
                    <Button
                        type="submit"
                        form="plan-change-form"
                        disabled={pending || !plan}
                        data-testid="plan-change-submit"
                    >
                        {t('plans.change.submit')}
                    </Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>
    );
}
