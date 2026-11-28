'use client';

import { useState, useTransition } from 'react';
import { useRouter } from 'next/navigation';
import { Plus, Trash2 } from 'lucide-react';
import { useFormatDateTime } from '@/components/display-preferences';
import { notify } from '@/components/notify';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { useT } from '@/i18n/client';
import { fromKurus, toKurus } from '@/lib/billing';
import type { Schemas } from '@/lib/types';
import { saveBillingSettingsAction } from '../../billing-actions';

type Seller = Schemas['BillingSeller'];
type Pricing = Schemas['BillingPricing'];

const money = (v: string) => {
    const k = toKurus(v);
    return k === null ? v : fromKurus(k);
};

function Field({
    id,
    label,
    value,
    onChange,
    ...rest
}: {
    id: string;
    label: string;
    value: string | number;
    onChange: (v: string) => void;
} & Omit<React.ComponentProps<typeof Input>, 'onChange' | 'value' | 'id'>) {
    return (
        <div className="grid gap-1.5">
            <Label htmlFor={id}>{label}</Label>
            <Input
                id={id}
                value={value}
                onChange={(e) => onChange(e.target.value)}
                {...rest}
            />
        </div>
    );
}

function Card({
    title,
    children,
    onSave,
    pending,
    testId,
}: {
    title: string;
    children: React.ReactNode;
    onSave: () => void;
    pending: boolean;
    testId: string;
}) {
    const t = useT();
    return (
        <section
            className="rounded-2xl border bg-card shadow-xs"
            data-testid={testId}
        >
            <header className="border-b bg-muted/30 px-5 py-4 font-semibold">
                {title}
            </header>
            <div className="grid gap-4 p-5">{children}</div>
            <footer className="flex justify-end border-t px-5 py-3">
                <Button size="sm" onClick={onSave} disabled={pending}>
                    {t('common.save')}
                </Button>
            </footer>
        </section>
    );
}

export function BillingSettingsForm({
    settings,
}: {
    settings: Schemas['PlatformBillingSettings'];
}) {
    const t = useT();
    const fmt = useFormatDateTime();
    const router = useRouter();
    const [pending, start] = useTransition();
    const [seller, setSeller] = useState<Seller>(settings.seller);
    const [pricing, setPricing] = useState<Pricing>(settings.pricing);
    const [topups, setTopups] = useState(
        settings.pricing.message_topups.join(', '),
    );
    const s = (k: keyof Seller) => (v: string) =>
        setSeller({ ...seller, [k]: v });
    const p = (k: keyof Pricing) => (v: string) =>
        setPricing({ ...pricing, [k]: v });
    const n = (k: keyof Pricing) => (v: string) =>
        setPricing({ ...pricing, [k]: Number(v) || 0 });
    const save = (body: Schemas['UpdatePlatformBillingSettingsRequest']) =>
        start(async () => {
            if (notify(await saveBillingSettingsAction(body))) router.refresh();
        });
    const saveSeller = () =>
        save({
            seller: {
                ...seller,
                bank_accounts: seller.bank_accounts.map((b) => ({
                    ...b,
                    iban: b.iban.replace(/\s+/g, ''),
                })),
            },
        });
    const savePricing = () =>
        save({
            pricing: {
                ...pricing,
                min_message_topup: money(pricing.min_message_topup),
                low_balance_threshold: money(pricing.low_balance_threshold),
                message_topups: topups
                    .split(/[,;\s]+/)
                    .filter(Boolean)
                    .map(money),
                ai_credit_packs: pricing.ai_credit_packs.map((x) => ({
                    ...x,
                    price: money(x.price),
                })),
            },
        });
    const banks = seller.bank_accounts;
    const setBank = (i: number, k: keyof Schemas['BankAccount'], v: string) =>
        setSeller({
            ...seller,
            bank_accounts: banks.map((b, j) =>
                j === i ? { ...b, [k]: v } : b,
            ),
        });
    const packs = pricing.ai_credit_packs;
    const setPack = (i: number, k: keyof Schemas['AICreditPack'], v: string) =>
        setPricing({
            ...pricing,
            ai_credit_packs: packs.map((x, j) =>
                j === i
                    ? { ...x, [k]: k === 'credits' ? Number(v) || 0 : v }
                    : x,
            ),
        });
    return (
        <div className="grid gap-6">
            <Card
                title={t('billing.admin.settings.seller')}
                onSave={saveSeller}
                pending={pending}
                testId="seller-settings"
            >
                <div className="grid gap-3 sm:grid-cols-2">
                    <Field
                        id="s-company"
                        label={t('billing.admin.settings.companyName')}
                        value={seller.company_name}
                        onChange={s('company_name')}
                    />
                    <Field
                        id="s-tax-office"
                        label={t('billing.admin.settings.taxOffice')}
                        value={seller.tax_office}
                        onChange={s('tax_office')}
                    />
                    <Field
                        id="s-tax-number"
                        label={t('billing.admin.settings.taxNumber')}
                        value={seller.tax_number}
                        onChange={s('tax_number')}
                    />
                    <Field
                        id="s-mersis"
                        label={t('billing.admin.settings.mersis')}
                        value={seller.mersis_no ?? ''}
                        onChange={s('mersis_no')}
                    />
                    <Field
                        id="s-email"
                        label={t('billing.admin.settings.email')}
                        value={seller.email}
                        onChange={s('email')}
                    />
                    <Field
                        id="s-phone"
                        label={t('billing.admin.settings.phone')}
                        value={seller.phone}
                        onChange={s('phone')}
                    />
                    <Field
                        id="s-website"
                        label={t('billing.admin.settings.website')}
                        value={seller.website}
                        onChange={s('website')}
                    />
                    <Field
                        id="s-address"
                        label={t('billing.admin.settings.address')}
                        value={seller.address}
                        onChange={s('address')}
                    />
                </div>
                <Field
                    id="s-note"
                    label={t('billing.admin.settings.invoiceNote')}
                    value={seller.invoice_note}
                    onChange={s('invoice_note')}
                />
                <div className="grid gap-3">
                    <h3 className="text-sm font-medium">
                        {t('billing.admin.settings.banks')}
                    </h3>
                    {banks.map((b, i) => (
                        <div
                            key={i}
                            className="grid items-end gap-2 sm:grid-cols-[1fr_1fr_2fr_auto]"
                            data-testid="bank-account"
                        >
                            <Field
                                id={`b-name-${i}`}
                                label={t('billing.admin.settings.bankName')}
                                value={b.bank_name}
                                onChange={(v) => setBank(i, 'bank_name', v)}
                            />
                            <Field
                                id={`b-holder-${i}`}
                                label={t('billing.admin.settings.holder')}
                                value={b.account_holder}
                                onChange={(v) =>
                                    setBank(i, 'account_holder', v)
                                }
                            />
                            <Field
                                id={`b-iban-${i}`}
                                label={t('billing.admin.settings.iban')}
                                value={b.iban}
                                onChange={(v) => setBank(i, 'iban', v)}
                                className="font-mono"
                            />
                            <Button
                                variant="ghost"
                                size="icon"
                                aria-label={t('billing.admin.settings.remove')}
                                onClick={() =>
                                    setSeller({
                                        ...seller,
                                        bank_accounts: banks.filter(
                                            (_, j) => j !== i,
                                        ),
                                    })
                                }
                            >
                                <Trash2 />
                            </Button>
                        </div>
                    ))}
                    {banks.length < 5 && (
                        <div>
                            <Button
                                variant="outline"
                                size="sm"
                                data-testid="add-bank"
                                onClick={() =>
                                    setSeller({
                                        ...seller,
                                        bank_accounts: [
                                            ...banks,
                                            {
                                                bank_name: '',
                                                account_holder:
                                                    seller.company_name,
                                                iban: '',
                                                currency: 'TRY',
                                            },
                                        ],
                                    })
                                }
                            >
                                <Plus /> {t('billing.admin.settings.addBank')}
                            </Button>
                        </div>
                    )}
                </div>
            </Card>

            <Card
                title={t('billing.admin.settings.pricing')}
                onSave={savePricing}
                pending={pending}
                testId="pricing-settings"
            >
                <div className="grid gap-3 sm:grid-cols-3">
                    <Field
                        id="p-vat"
                        label={t('billing.admin.settings.vat')}
                        value={pricing.vat_rate_percent}
                        onChange={p('vat_rate_percent')}
                        inputMode="decimal"
                    />
                    <div className="grid gap-1">
                        <Field
                            id="p-fx"
                            label={t('billing.admin.settings.fx')}
                            value={pricing.fx_usd_try}
                            onChange={p('fx_usd_try')}
                            inputMode="decimal"
                        />
                        {pricing.fx_updated_at && (
                            <span className="text-xs text-muted-foreground">
                                {t('billing.admin.settings.fxUpdated', {
                                    date: fmt(pricing.fx_updated_at),
                                })}
                            </span>
                        )}
                    </div>
                    <Field
                        id="p-mk"
                        label={t('billing.admin.settings.marketingMarkup')}
                        value={pricing.marketing_markup}
                        onChange={p('marketing_markup')}
                        inputMode="decimal"
                    />
                    <Field
                        id="p-other"
                        label={t('billing.admin.settings.otherMarkup')}
                        value={pricing.other_markup_usd}
                        onChange={p('other_markup_usd')}
                        inputMode="decimal"
                    />
                    <Field
                        id="p-min"
                        label={t('billing.admin.settings.minTopup')}
                        value={pricing.min_message_topup}
                        onChange={p('min_message_topup')}
                        inputMode="decimal"
                    />
                    <Field
                        id="p-low"
                        label={t('billing.admin.settings.lowThreshold')}
                        value={pricing.low_balance_threshold}
                        onChange={p('low_balance_threshold')}
                        inputMode="decimal"
                    />
                    <Field
                        id="p-lead"
                        label={t('billing.admin.settings.leadDays')}
                        value={pricing.renewal_lead_days}
                        onChange={n('renewal_lead_days')}
                        type="number"
                    />
                    <Field
                        id="p-due"
                        label={t('billing.admin.settings.dueDays')}
                        value={pricing.due_days}
                        onChange={n('due_days')}
                        type="number"
                    />
                    <Field
                        id="p-grace"
                        label={t('billing.admin.settings.graceDays')}
                        value={pricing.grace_days}
                        onChange={n('grace_days')}
                        type="number"
                    />
                    <Field
                        id="p-validity"
                        label={t('billing.admin.settings.validity')}
                        value={pricing.ai_credit_validity_months}
                        onChange={n('ai_credit_validity_months')}
                        type="number"
                    />
                </div>
                <Field
                    id="p-topups"
                    label={t('billing.admin.settings.topups')}
                    value={topups}
                    onChange={setTopups}
                />
                <div className="grid gap-3">
                    <h3 className="text-sm font-medium">
                        {t('billing.admin.settings.packs')}
                    </h3>
                    {packs.map((x, i) => (
                        <div
                            key={i}
                            className="grid items-end gap-2 sm:grid-cols-[1fr_1fr_1fr_auto]"
                        >
                            <Field
                                id={`k-key-${i}`}
                                label={t('billing.admin.settings.packKey')}
                                value={x.key}
                                onChange={(v) => setPack(i, 'key', v)}
                            />
                            <Field
                                id={`k-credits-${i}`}
                                label={t('billing.admin.settings.packCredits')}
                                value={x.credits}
                                onChange={(v) => setPack(i, 'credits', v)}
                                type="number"
                            />
                            <Field
                                id={`k-price-${i}`}
                                label={t('billing.admin.settings.packPrice')}
                                value={x.price}
                                onChange={(v) => setPack(i, 'price', v)}
                                inputMode="decimal"
                            />
                            <Button
                                variant="ghost"
                                size="icon"
                                aria-label={t('billing.admin.settings.remove')}
                                onClick={() =>
                                    setPricing({
                                        ...pricing,
                                        ai_credit_packs: packs.filter(
                                            (_, j) => j !== i,
                                        ),
                                    })
                                }
                            >
                                <Trash2 />
                            </Button>
                        </div>
                    ))}
                    {packs.length < 10 && (
                        <div>
                            <Button
                                variant="outline"
                                size="sm"
                                onClick={() =>
                                    setPricing({
                                        ...pricing,
                                        ai_credit_packs: [
                                            ...packs,
                                            {
                                                key: `c${packs.length + 1}`,
                                                credits: 1000,
                                                price: '590.00',
                                            },
                                        ],
                                    })
                                }
                            >
                                <Plus /> {t('billing.admin.settings.addPack')}
                            </Button>
                        </div>
                    )}
                </div>
            </Card>
        </div>
    );
}
