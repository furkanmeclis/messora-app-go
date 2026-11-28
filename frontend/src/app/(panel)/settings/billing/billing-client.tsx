'use client';

import { useState, useTransition } from 'react';
import { useRouter } from 'next/navigation';
import {
    AlertTriangle,
    CreditCard,
    FileDown,
    Lock,
    MessageCircle,
    Sparkles,
} from 'lucide-react';
import {
    BillingStatusBadge,
    ProformaPayCard,
    ProformaStatusBadge,
    useMoney,
} from '@/components/billing';
import { useConfirm } from '@/components/confirm';
import { useFormatDate } from '@/components/display-preferences';
import { notify } from '@/components/notify';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Segmented } from '@/components/ui/segmented';
import {
    Table,
    TableBody,
    TableCell,
    TableHead,
    TableHeader,
    TableRow,
} from '@/components/ui/table';
import { useLocale, useT } from '@/i18n/client';
import { fromKurus, toKurus, withVAT } from '@/lib/billing';
import { formatCredits } from '@/lib/plan';
import type { Schemas } from '@/lib/types';
import { cn } from '@/lib/utils';
import { Row, Section } from '../section';
import {
    cancelBillingChangeAction,
    cancelBillingProformaAction,
    changeBillingPlanAction,
    createBillingProformaAction,
    saveBillingProfileAction,
} from './actions';

type Overview = Schemas['BillingOverview'];
type Cycle = Schemas['BillingCycle'];

const proformaPdf = (id: string) => `/bff/billing/proformas/${id}/pdf`;
const invoicePdf = (id: string) => `/bff/billing/invoices/${id}/pdf`;

export function BillingPage({
    overview: o,
    proformas,
    invoices,
    canManage,
}: {
    overview: Overview;
    proformas: Schemas['Proforma'][];
    invoices: Schemas['Invoice'][];
    canManage: boolean;
}) {
    const t = useT();
    const s = o.subscription;
    const configured = o.seller.bank_accounts.length > 0;
    return (
        <div className="grid gap-6" data-testid="billing-page">
            <SubscriptionCard overview={o} canManage={canManage} />
            {s.read_only && (
                <Alert variant="destructive" data-testid="billing-read-only">
                    <Lock />
                    <AlertTitle>{t('billing.alert.readOnlyTitle')}</AlertTitle>
                    <AlertDescription>
                        {t('billing.alert.readOnly')}
                    </AlertDescription>
                </Alert>
            )}
            {!configured && (
                <Alert>
                    <AlertTriangle />
                    <AlertDescription>
                        {t('billing.alert.notConfigured')}
                    </AlertDescription>
                </Alert>
            )}
            {!o.profile && canManage && (
                <ProfileForm profile={null} canManage={canManage} prominent />
            )}
            <Section title={t('billing.proforma.open')}>
                {o.open_proformas.length === 0 ? (
                    <p
                        className="text-sm text-muted-foreground"
                        data-testid="no-open-proformas"
                    >
                        {t('billing.proforma.none')}
                    </p>
                ) : (
                    <div className="grid gap-4">
                        {o.open_proformas.map((f) => (
                            <ProformaPayCard
                                key={f.id}
                                proforma={f}
                                pdfHref={proformaPdf(f.id)}
                                actions={
                                    canManage &&
                                    !f.renewal &&
                                    Number(f.paid) === 0 && (
                                        <CancelProforma proforma={f} />
                                    )
                                }
                            />
                        ))}
                    </div>
                )}
            </Section>
            <Balances overview={o} />
            {canManage && <PlanChooser overview={o} />}
            {canManage && <Purchases overview={o} />}
            <Documents proformas={proformas} invoices={invoices} />
            {o.profile && (
                <ProfileForm profile={o.profile} canManage={canManage} />
            )}
        </div>
    );
}

function SubscriptionCard({
    overview: o,
    canManage,
}: {
    overview: Overview;
    canManage: boolean;
}) {
    const t = useT();
    const money = useMoney();
    const fmtDate = useFormatDate();
    const locale = useLocale();
    const router = useRouter();
    const [pending, start] = useTransition();
    const s = o.subscription;
    const name = (key: string | null | undefined) => {
        const p = o.offer.plans.find((x) => x.key === key);
        if (!p) return key ?? t('billing.plan.none');
        return locale === 'tr' ? p.name_tr : p.name_en;
    };
    const planName =
        (locale === 'tr' ? s.plan_name_tr : s.plan_name_en) || name(s.plan_key);
    return (
        <section className="flex flex-wrap items-start justify-between gap-4 rounded-2xl border bg-linear-to-br from-primary/10 to-transparent p-5">
            <div className="flex items-start gap-4">
                <span className="flex size-12 items-center justify-center rounded-2xl bg-primary/15 text-primary">
                    <CreditCard className="size-6" />
                </span>
                <div className="grid gap-1">
                    <span className="text-xs text-muted-foreground">
                        {t('billing.plan.current')}
                    </span>
                    <h2
                        className="text-xl font-semibold tracking-tight"
                        data-testid="billing-plan"
                        data-plan={s.plan_key ?? ''}
                    >
                        {planName}
                        {s.price && (
                            <span className="ml-2 text-base font-normal text-muted-foreground">
                                {t('billing.plan.price', {
                                    price: money(s.price),
                                    per: t(`billing.per.${s.billing_cycle}`),
                                })}
                            </span>
                        )}
                    </h2>
                    <div className="flex flex-wrap items-center gap-2 text-sm">
                        <BillingStatusBadge status={s.status} />
                        <Badge variant="outline">
                            {t(`billing.cycle.${s.billing_cycle}`)}
                        </Badge>
                        {s.status === 'trial' && s.trial_ends_at && (
                            <span className="text-muted-foreground">
                                {t('billing.plan.trialUntil', {
                                    date: fmtDate(s.trial_ends_at),
                                })}
                            </span>
                        )}
                        {s.status === 'past_due' && s.grace_ends_at && (
                            <span className="text-warning">
                                {t('billing.plan.graceUntil', {
                                    date: fmtDate(s.grace_ends_at),
                                })}
                            </span>
                        )}
                    </div>
                    <p className="text-sm text-muted-foreground">
                        {s.period_start && s.period_end
                            ? t('billing.plan.period', {
                                  start: fmtDate(s.period_start),
                                  end: fmtDate(s.period_end),
                              })
                            : t('billing.plan.noPeriod')}
                        {s.next_renewal_at &&
                            ' · ' +
                                t('billing.plan.nextRenewal', {
                                    date: fmtDate(s.next_renewal_at),
                                })}
                    </p>
                    {(s.pending_plan_key || s.pending_cycle) && (
                        <div
                            className="flex flex-wrap items-center gap-2 text-sm"
                            data-testid="billing-pending"
                        >
                            {t('billing.plan.pending', {
                                plan: name(s.pending_plan_key ?? s.plan_key),
                                cycle: t(
                                    `billing.cycle.${s.pending_cycle ?? s.billing_cycle}`,
                                ),
                            })}
                            {canManage && (
                                <Button
                                    size="xs"
                                    variant="outline"
                                    disabled={pending}
                                    onClick={() =>
                                        start(async () => {
                                            if (
                                                notify(
                                                    await cancelBillingChangeAction(),
                                                )
                                            ) {
                                                router.refresh();
                                            }
                                        })
                                    }
                                >
                                    {t('billing.plan.pendingCancel')}
                                </Button>
                            )}
                        </div>
                    )}
                </div>
            </div>
            <dl className="grid gap-1 text-right text-sm">
                <dt className="text-xs text-muted-foreground">
                    {t('billing.plan.accountCredit')}
                </dt>
                <dd className="font-semibold" data-testid="account-credit">
                    {money(s.account_credit)}
                </dd>
                <dt className="mt-2 text-xs text-muted-foreground">
                    {t('billing.plan.code')}
                </dt>
                <dd className="font-mono">{s.code}</dd>
            </dl>
        </section>
    );
}

function Balances({ overview: o }: { overview: Overview }) {
    const t = useT();
    const locale = useLocale();
    const money = useMoney();
    const fmtDate = useFormatDate();
    const ai = o.ai_credits;
    const mb = o.message_balance;
    return (
        <Section title={t('billing.balances.title')}>
            <div className="grid gap-4 md:grid-cols-2">
                <div className="rounded-xl border p-4" data-testid="ai-balance">
                    <div className="mb-1 flex items-center gap-2 text-sm font-medium">
                        <Sparkles className="size-4 text-primary" />
                        {t('billing.balances.ai')}
                    </div>
                    <div className="text-2xl font-semibold">
                        {ai.unlimited
                            ? t('billing.balances.aiUnlimited')
                            : formatCredits(ai.remaining, locale)}
                    </div>
                    {!ai.unlimited && (
                        <p className="text-sm text-muted-foreground">
                            {t('billing.balances.aiLeft', {
                                remaining: formatCredits(ai.remaining, locale),
                                granted: formatCredits(ai.granted, locale),
                            })}
                        </p>
                    )}
                    <p className="mt-2 text-xs text-muted-foreground">
                        {t('billing.balances.aiResets', {
                            date: fmtDate(ai.resets_at),
                        })}
                    </p>
                </div>
                <div
                    className="rounded-xl border p-4"
                    data-testid="message-balance"
                    data-used={String(mb.used)}
                >
                    <div className="mb-1 flex items-center gap-2 text-sm font-medium">
                        <MessageCircle className="size-4 text-primary" />
                        {t('billing.balances.message')}
                        {mb.used && mb.exhausted && (
                            <Badge variant="destructive">
                                {t('billing.balances.exhausted')}
                            </Badge>
                        )}
                        {mb.used && !mb.exhausted && mb.low && (
                            <Badge variant="outline" className="text-warning">
                                {t('billing.balances.low')}
                            </Badge>
                        )}
                    </div>
                    {mb.used ? (
                        <>
                            <div className="text-2xl font-semibold">
                                {money(mb.balance)}
                            </div>
                            <p className="mt-2 text-xs text-muted-foreground">
                                {t('billing.balances.messageRates', {
                                    markup: mb.marketing_markup,
                                    usd: mb.other_markup_usd,
                                    fx: mb.fx_usd_try,
                                })}
                            </p>
                        </>
                    ) : (
                        <p className="text-sm text-muted-foreground">
                            {t('billing.balances.messageNotUsed')}
                        </p>
                    )}
                </div>
            </div>
        </Section>
    );
}

function PlanChooser({ overview: o }: { overview: Overview }) {
    const t = useT();
    const money = useMoney();
    const router = useRouter();
    const [cycle, setCycle] = useState<Cycle>(o.subscription.billing_cycle);
    const [pending, start] = useTransition();
    const [result, setResult] = useState<string | null>(null);
    const vat = o.offer.vat_rate_percent;
    const locale = useLocale();
    const tr = locale === 'tr';
    const choose = (key: string) =>
        start(async () => {
            const r = await changeBillingPlanAction({
                plan_key: key,
                billing_cycle: cycle,
            });
            if (!notify(r)) return;
            const f = r.data.proforma;
            setResult(
                r.data.effect === 'proforma' && f
                    ? t('billing.choose.proforma', {
                          number: f.number,
                          total: money(f.total),
                      })
                    : t('billing.choose.scheduled'),
            );
            router.refresh();
        });
    return (
        <Section
            title={t('billing.choose.title')}
            description={t('billing.choose.description')}
        >
            <div className="flex flex-wrap items-center gap-3">
                <Segmented<Cycle>
                    value={cycle}
                    onValueChange={setCycle}
                    aria-label={t('billing.choose.title')}
                    options={[
                        { value: 'monthly', label: t('billing.cycle.monthly') },
                        { value: 'yearly', label: t('billing.cycle.yearly') },
                    ]}
                />
                <span className="text-sm text-muted-foreground">
                    {t('billing.choose.yearlyHint')}
                </span>
            </div>
            {result && (
                <p
                    className="rounded-lg bg-accent/50 px-3 py-2 text-sm"
                    data-testid="plan-change-result"
                >
                    {result}
                </p>
            )}
            <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
                {o.offer.plans.map((p) => {
                    const price = cycle === 'yearly' ? p.yearly : p.monthly;
                    const gross = price ? withVAT(price, vat) : null;
                    // Without a paid period (trial, read-only) the
                    // current plan is bought like any other.
                    const paid =
                        o.subscription.period_end !== null &&
                        (o.subscription.status === 'active' ||
                            o.subscription.status === 'past_due');
                    const current =
                        paid &&
                        p.current &&
                        cycle === o.subscription.billing_cycle;
                    return (
                        <div
                            key={p.key}
                            className={cn(
                                'flex flex-col gap-2 rounded-xl border p-4',
                                current && 'border-primary ring-1 ring-primary',
                            )}
                            data-testid="billing-plan-option"
                            data-plan={p.key}
                        >
                            <div className="font-semibold">
                                {tr ? p.name_tr : p.name_en}
                            </div>
                            {p.custom_pricing || !price ? (
                                <p className="text-sm text-muted-foreground">
                                    {t('billing.choose.custom')}
                                </p>
                            ) : (
                                <>
                                    <div className="text-lg font-semibold">
                                        {t('billing.choose.net', {
                                            price: money(price),
                                        })}
                                        <span className="text-sm font-normal text-muted-foreground">
                                            {' / '}
                                            {t(`billing.per.${cycle}`)}
                                        </span>
                                    </div>
                                    {gross && (
                                        <div className="text-xs text-muted-foreground">
                                            {t('billing.choose.gross', {
                                                price: money(gross.total),
                                            })}
                                        </div>
                                    )}
                                </>
                            )}
                            <div className="mt-auto pt-2">
                                {current ? (
                                    <Badge variant="outline">
                                        {t('billing.choose.current')}
                                    </Badge>
                                ) : (
                                    !p.custom_pricing &&
                                    price && (
                                        <Button
                                            size="sm"
                                            disabled={pending}
                                            onClick={() => choose(p.key)}
                                            data-testid="choose-plan"
                                        >
                                            {t('billing.choose.select')}
                                        </Button>
                                    )
                                )}
                            </div>
                        </div>
                    );
                })}
            </div>
        </Section>
    );
}

function Purchases({ overview: o }: { overview: Overview }) {
    const t = useT();
    const money = useMoney();
    const router = useRouter();
    const [pending, start] = useTransition();
    const min = o.offer.min_message_topup;
    const [amount, setAmount] = useState(o.offer.message_topups[0] ?? min);
    const vat = o.offer.vat_rate_percent;
    const buy = (body: Schemas['CreateBillingProformaRequest']) =>
        start(async () => {
            const r = await createBillingProformaAction(body);
            if (
                notify({
                    ...r,
                    ...(r.ok
                        ? {
                              message: t('billing.buy.created', {
                                  number: r.data.number,
                              }),
                          }
                        : {}),
                })
            ) {
                router.refresh();
            }
        });
    const custom = toKurus(amount);
    const tooLow = custom === null || custom < (toKurus(min) ?? 0);
    return (
        <Section title={t('billing.buy.title')}>
            <div className="grid gap-6 lg:grid-cols-2">
                <div className="grid gap-3">
                    <h3 className="text-sm font-medium">
                        {t('billing.buy.packs')}
                    </h3>
                    {o.offer.ai_credit_packs.map((p) => (
                        <div
                            key={p.key}
                            className="flex items-center justify-between gap-3 rounded-xl border p-3"
                            data-testid="ai-pack"
                            data-pack={p.key}
                        >
                            <div>
                                <div className="font-medium">
                                    {t('billing.buy.credits', {
                                        credits: p.credits.toLocaleString(),
                                    })}
                                </div>
                                <div className="text-xs text-muted-foreground">
                                    {t('billing.choose.net', {
                                        price: money(p.price),
                                    })}{' '}
                                    ·{' '}
                                    {t('billing.choose.gross', {
                                        price: money(
                                            withVAT(p.price, vat)?.total ?? 0,
                                        ),
                                    })}
                                </div>
                            </div>
                            <Button
                                size="sm"
                                variant="outline"
                                disabled={pending}
                                onClick={() =>
                                    buy({ kind: 'ai_credits', pack_key: p.key })
                                }
                            >
                                {t('billing.buy.buy')}
                            </Button>
                        </div>
                    ))}
                </div>
                <div className="grid content-start gap-3">
                    <h3 className="text-sm font-medium">
                        {t('billing.buy.topup')}
                    </h3>
                    <p className="text-xs text-muted-foreground">
                        {t('billing.buy.topupHint', { min: money(min) })}
                    </p>
                    <div className="flex flex-wrap gap-2">
                        {o.offer.message_topups.map((a) => (
                            <Button
                                key={a}
                                size="sm"
                                variant={a === amount ? 'default' : 'outline'}
                                onClick={() => setAmount(a)}
                            >
                                {money(a)}
                            </Button>
                        ))}
                    </div>
                    <Row label={t('billing.buy.custom')} htmlFor="topup-amount">
                        <Input
                            id="topup-amount"
                            inputMode="decimal"
                            value={amount}
                            onChange={(e) => setAmount(e.target.value)}
                            className="max-w-48"
                        />
                    </Row>
                    {custom !== null && !tooLow && (
                        <p className="text-xs text-muted-foreground">
                            {t('billing.choose.gross', {
                                price: money(
                                    withVAT(fromKurus(custom), vat)?.total ?? 0,
                                ),
                            })}
                        </p>
                    )}
                    {tooLow && (
                        <p className="text-xs text-destructive">
                            {t('billing.buy.minimum', { min: money(min) })}
                        </p>
                    )}
                    <div>
                        <Button
                            size="sm"
                            disabled={pending || tooLow}
                            onClick={() =>
                                custom !== null &&
                                buy({
                                    kind: 'message_balance',
                                    amount: fromKurus(custom),
                                })
                            }
                        >
                            {t('billing.buy.topupButton')}
                        </Button>
                    </div>
                </div>
            </div>
        </Section>
    );
}

function CancelProforma({ proforma: f }: { proforma: Schemas['Proforma'] }) {
    const t = useT();
    const ask = useConfirm();
    const router = useRouter();
    const [pending, start] = useTransition();
    return (
        <Button
            size="sm"
            variant="ghost"
            disabled={pending}
            onClick={async () => {
                if (
                    !(await ask({
                        title: t('billing.proforma.cancelConfirm', {
                            number: f.number,
                        }),
                        destructive: true,
                    }))
                ) {
                    return;
                }
                start(async () => {
                    if (notify(await cancelBillingProformaAction(f.id))) {
                        router.refresh();
                    }
                });
            }}
        >
            {t('billing.proforma.cancel')}
        </Button>
    );
}

function Documents({
    proformas,
    invoices,
}: {
    proformas: Schemas['Proforma'][];
    invoices: Schemas['Invoice'][];
}) {
    const t = useT();
    const money = useMoney();
    const fmtDate = useFormatDate();
    return (
        <div className="grid gap-6 lg:grid-cols-2">
            <Section title={t('billing.docs.proformas')}>
                {proformas.length === 0 ? (
                    <p className="text-sm text-muted-foreground">
                        {t('billing.docs.noProformas')}
                    </p>
                ) : (
                    <Table data-testid="proformas-table">
                        <TableHeader>
                            <TableRow>
                                <TableHead>
                                    {t('billing.docs.document')}
                                </TableHead>
                                <TableHead>{t('billing.docs.total')}</TableHead>
                                <TableHead>
                                    {t('billing.docs.status')}
                                </TableHead>
                                <TableHead />
                            </TableRow>
                        </TableHeader>
                        <TableBody>
                            {proformas.map((f) => (
                                <TableRow key={f.id}>
                                    <TableCell>
                                        <div className="font-medium">
                                            {f.number}
                                        </div>
                                        <div className="text-xs text-muted-foreground">
                                            {t(
                                                `billing.proforma.kind.${f.kind}`,
                                            )}{' '}
                                            · {fmtDate(f.created_at)}
                                        </div>
                                    </TableCell>
                                    <TableCell>{money(f.total)}</TableCell>
                                    <TableCell>
                                        <ProformaStatusBadge
                                            status={f.status}
                                        />
                                    </TableCell>
                                    <TableCell className="text-right">
                                        <Button
                                            asChild
                                            size="icon-xs"
                                            variant="ghost"
                                        >
                                            <a
                                                href={proformaPdf(f.id)}
                                                download
                                                aria-label={t(
                                                    'billing.docs.download',
                                                )}
                                            >
                                                <FileDown />
                                            </a>
                                        </Button>
                                    </TableCell>
                                </TableRow>
                            ))}
                        </TableBody>
                    </Table>
                )}
            </Section>
            <Section title={t('billing.docs.invoices')}>
                {invoices.length === 0 ? (
                    <p className="text-sm text-muted-foreground">
                        {t('billing.docs.noInvoices')}
                    </p>
                ) : (
                    <Table data-testid="invoices-table">
                        <TableHeader>
                            <TableRow>
                                <TableHead>
                                    {t('billing.docs.document')}
                                </TableHead>
                                <TableHead>{t('billing.docs.total')}</TableHead>
                                <TableHead>
                                    {t('billing.docs.status')}
                                </TableHead>
                                <TableHead />
                            </TableRow>
                        </TableHeader>
                        <TableBody>
                            {invoices.map((i) => (
                                <TableRow
                                    key={i.id}
                                    data-testid="invoice-row"
                                    data-status={i.status}
                                >
                                    <TableCell>
                                        <div className="font-medium">
                                            {i.official_number
                                                ? t('billing.docs.official', {
                                                      number: i.official_number,
                                                  })
                                                : i.proforma_number}
                                        </div>
                                        <div className="text-xs text-muted-foreground">
                                            {i.proforma_number} ·{' '}
                                            {fmtDate(
                                                i.issued_on ?? i.created_at,
                                            )}
                                        </div>
                                    </TableCell>
                                    <TableCell>{money(i.total)}</TableCell>
                                    <TableCell>
                                        <Badge variant="outline">
                                            {t(
                                                `billing.docs.invoiceState.${i.status}`,
                                            )}
                                        </Badge>
                                    </TableCell>
                                    <TableCell className="text-right">
                                        {i.has_pdf && (
                                            <Button
                                                asChild
                                                size="icon-xs"
                                                variant="ghost"
                                            >
                                                <a
                                                    href={invoicePdf(i.id)}
                                                    download
                                                    aria-label={t(
                                                        'billing.docs.download',
                                                    )}
                                                >
                                                    <FileDown />
                                                </a>
                                            </Button>
                                        )}
                                    </TableCell>
                                </TableRow>
                            ))}
                        </TableBody>
                    </Table>
                )}
            </Section>
        </div>
    );
}

function ProfileForm({
    profile,
    canManage,
    prominent,
}: {
    profile: Schemas['BillingProfile'] | null;
    canManage: boolean;
    prominent?: boolean;
}) {
    const t = useT();
    const router = useRouter();
    const [pending, start] = useTransition();
    const initial = {
        kind: (profile?.kind ?? 'company') as 'company' | 'individual',
        title: profile?.title ?? '',
        tax_office: profile?.tax_office ?? '',
        tax_number: profile?.tax_number ?? '',
        address: profile?.address ?? '',
        city: profile?.city ?? '',
        email: profile?.email ?? '',
        phone: profile?.phone ?? '',
    };
    const [v, setV] = useState(initial);
    const [errors, setErrors] = useState<Record<string, string>>({});
    const dirty = JSON.stringify(v) !== JSON.stringify(initial);
    const set = (k: keyof typeof v) => (e: { target: { value: string } }) =>
        setV({ ...v, [k]: e.target.value });
    const save = () =>
        start(async () => {
            const r = await saveBillingProfileAction({
                kind: v.kind,
                title: v.title.trim(),
                tax_office: v.tax_office.trim() || undefined,
                tax_number: v.tax_number.trim(),
                address: v.address.trim(),
                city: v.city.trim() || undefined,
                email: v.email.trim(),
                phone: v.phone.trim() || undefined,
            });
            setErrors(r.ok ? {} : (r.fieldErrors ?? {}));
            if (notify(r)) router.refresh();
        });
    const field = (
        k: keyof typeof v,
        label: string,
        props: React.ComponentProps<typeof Input> = {},
    ) => (
        <Row label={label} htmlFor={`bp-${k}`} hint={errors[k]}>
            <Input
                id={`bp-${k}`}
                name={k}
                value={v[k]}
                onChange={set(k)}
                disabled={!canManage}
                aria-invalid={!!errors[k]}
                {...props}
            />
        </Row>
    );
    const company = v.kind === 'company';
    return (
        <Section
            title={t('billing.profile.title')}
            description={
                prominent
                    ? t('billing.alert.profile')
                    : t('billing.profile.description')
            }
            className={cn(prominent && 'border-primary ring-1 ring-primary/40')}
            dirty={dirty}
            saving={pending}
            onSave={save}
            onReset={() => setV(initial)}
            readOnly={!canManage}
        >
            <div data-testid="billing-profile" className="grid gap-4">
                <Row label={t('billing.profile.kind')}>
                    <Segmented<'company' | 'individual'>
                        value={v.kind}
                        onValueChange={(kind) => setV({ ...v, kind })}
                        disabled={!canManage}
                        options={[
                            {
                                value: 'company',
                                label: t('billing.profile.company'),
                            },
                            {
                                value: 'individual',
                                label: t('billing.profile.individual'),
                            },
                        ]}
                    />
                </Row>
                <div className="grid gap-4 md:grid-cols-2">
                    {field(
                        'title',
                        company
                            ? t('billing.profile.companyTitle')
                            : t('billing.profile.fullName'),
                    )}
                    {company &&
                        field('tax_office', t('billing.profile.taxOffice'))}
                    {field(
                        'tax_number',
                        company
                            ? t('billing.profile.vkn')
                            : t('billing.profile.tckn'),
                        { inputMode: 'numeric', maxLength: company ? 10 : 11 },
                    )}
                    {field('city', t('billing.profile.city'))}
                </div>
                {field('address', t('billing.profile.address'))}
                <div className="grid gap-4 md:grid-cols-2">
                    {field('email', t('billing.profile.email'), {
                        type: 'email',
                    })}
                    {field('phone', t('billing.profile.phone'), {
                        type: 'tel',
                    })}
                </div>
                <p className="text-xs text-muted-foreground">
                    {t('billing.profile.emailHint')}
                </p>
            </div>
        </Section>
    );
}
