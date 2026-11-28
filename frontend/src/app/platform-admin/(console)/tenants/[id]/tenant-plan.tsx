'use client';

import { useState, useTransition } from 'react';
import { useRouter } from 'next/navigation';
import { Coins, Gem } from 'lucide-react';
import { useFormatDateTime } from '@/components/display-preferences';
import { notify } from '@/components/notify';
import {
    PlanFeatureList,
    PlanUsage,
    usePlanName,
} from '@/components/plan-usage';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
    Card,
    CardContent,
    CardDescription,
    CardHeader,
    CardTitle,
} from '@/components/ui/card';
import { DateTimePicker } from '@/components/ui/date-picker';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { NumberField } from '@/components/ui/number-field';
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from '@/components/ui/select';
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
import { formatCredits, parseOverrides } from '@/lib/plan';
import type { Schemas } from '@/lib/types';
import { grantCreditsAction, setTenantPlanAction } from '../../../plan-actions';

/** A company's plan, own limits, usage, credit lots and manual grants. */
export function TenantPlan({
    tenantId,
    data,
    plans,
}: {
    tenantId: string;
    data: Schemas['PlatformTenantPlan'];
    plans: Schemas['PlatformPlan'][];
}) {
    const t = useT();
    const planName = usePlanName();
    return (
        <div className="mt-6 grid items-start gap-6 xl:grid-cols-[minmax(0,1fr)_24rem]">
            <Card data-testid="tenant-plan">
                <CardHeader>
                    <CardTitle className="flex items-center gap-2">
                        <Gem className="size-4 text-primary" />
                        {t('plans.admin.tenant.title')}
                        <Badge variant="outline" data-testid="tenant-plan-key">
                            {planName(data.overview)}
                        </Badge>
                        {data.overview.overridden && (
                            <Badge variant="outline">
                                {t('plans.overridden')}
                            </Badge>
                        )}
                    </CardTitle>
                    <CardDescription>
                        {t('plans.admin.tenant.description')}
                    </CardDescription>
                </CardHeader>
                <CardContent className="grid gap-6">
                    <PlanUsage overview={data.overview} />
                    <PlanFeatureList features={data.overview.features} />
                    <Lots lots={data.lots} />
                </CardContent>
            </Card>
            <div className="grid gap-6">
                <AssignPlan tenantId={tenantId} data={data} plans={plans} />
                <GrantCredits tenantId={tenantId} />
            </div>
        </div>
    );
}

function AssignPlan({
    tenantId,
    data,
    plans,
}: {
    tenantId: string;
    data: Schemas['PlatformTenantPlan'];
    plans: Schemas['PlatformPlan'][];
}) {
    const t = useT();
    const router = useRouter();
    const planName = usePlanName();
    const [plan, setPlan] = useState(data.plan_key ?? '');
    const initial =
        Object.keys(data.overrides ?? {}).length > 0
            ? JSON.stringify(data.overrides, null, 2)
            : '';
    const [overrides, setOverrides] = useState(initial);
    const [error, setError] = useState('');
    const [pending, start] = useTransition();
    const save = () => {
        const o = parseOverrides(overrides);
        if (!o.ok) {
            setError(
                t('plans.admin.tenant.overridesInvalid', { error: o.error }),
            );
            return;
        }
        setError('');
        start(async () => {
            if (
                notify(
                    await setTenantPlanAction(tenantId, {
                        plan_key: plan,
                        overrides: o.value,
                    }),
                )
            )
                router.refresh();
        });
    };
    return (
        <Card>
            <CardHeader>
                <CardTitle>{t('plans.admin.tenant.plan')}</CardTitle>
                <CardDescription>
                    {t('plans.admin.tenant.downgradeHint')}
                </CardDescription>
            </CardHeader>
            <CardContent>
                <form
                    className="grid gap-4"
                    onSubmit={(e) => {
                        e.preventDefault();
                        save();
                    }}
                >
                    <Select value={plan} onValueChange={setPlan}>
                        <SelectTrigger
                            aria-label={t('plans.admin.tenant.plan')}
                            data-testid="tenant-plan-select"
                        >
                            <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                            {plans
                                .filter(
                                    (p) => p.active || p.key === data.plan_key,
                                )
                                .map((p) => (
                                    <SelectItem
                                        key={p.key}
                                        value={p.key}
                                        data-plan={p.key}
                                    >
                                        {planName(p)}
                                    </SelectItem>
                                ))}
                        </SelectContent>
                    </Select>
                    <div className="grid gap-2">
                        <Label htmlFor="tenant-overrides">
                            {t('plans.admin.tenant.overrides')}
                        </Label>
                        <Textarea
                            id="tenant-overrides"
                            value={overrides}
                            rows={5}
                            className="font-mono text-xs"
                            placeholder="{}"
                            onChange={(e) => setOverrides(e.target.value)}
                        />
                        <p className="text-xs text-muted-foreground">
                            {t('plans.admin.tenant.overridesHint')}
                        </p>
                        {error && (
                            <p className="text-xs text-destructive">{error}</p>
                        )}
                    </div>
                    <Button
                        type="submit"
                        disabled={pending || !plan}
                        data-testid="tenant-plan-save"
                    >
                        {t('plans.admin.tenant.assign')}
                    </Button>
                </form>
            </CardContent>
        </Card>
    );
}

function GrantCredits({ tenantId }: { tenantId: string }) {
    const t = useT();
    const router = useRouter();
    const [credits, setCredits] = useState<number | null>(1000);
    const [expires, setExpires] = useState('');
    const [note, setNote] = useState('');
    const [pending, start] = useTransition();
    const submit = () =>
        start(async () => {
            const r = await grantCreditsAction(tenantId, {
                credits: credits ?? 0,
                note: note.trim(),
                expires_at: expires ? new Date(expires).toISOString() : null,
            });
            if (notify(r)) {
                setNote('');
                router.refresh();
            }
        });
    return (
        <Card>
            <CardHeader>
                <CardTitle className="flex items-center gap-2">
                    <Coins className="size-4 text-primary" />
                    {t('plans.admin.tenant.grant')}
                </CardTitle>
                <CardDescription>
                    {t('plans.admin.tenant.grantHint')}
                </CardDescription>
            </CardHeader>
            <CardContent>
                <form
                    className="grid gap-4"
                    onSubmit={(e) => {
                        e.preventDefault();
                        submit();
                    }}
                >
                    <div className="grid gap-2">
                        <Label htmlFor="grant-credits">
                            {t('plans.admin.tenant.credits')}
                        </Label>
                        <NumberField
                            id="grant-credits"
                            aria-label={t('plans.admin.tenant.credits')}
                            value={credits}
                            min={1}
                            max={100000000}
                            onValueChange={setCredits}
                        />
                    </div>
                    <div className="grid gap-2">
                        <Label htmlFor="grant-expires">
                            {t('plans.admin.tenant.expiresAt')}
                        </Label>
                        <DateTimePicker
                            id="grant-expires"
                            value={expires}
                            onValueChange={setExpires}
                        />
                    </div>
                    <div className="grid gap-2">
                        <Label htmlFor="grant-note">
                            {t('plans.admin.tenant.grantNote')}
                        </Label>
                        <Input
                            id="grant-note"
                            value={note}
                            minLength={3}
                            maxLength={500}
                            required
                            onChange={(e) => setNote(e.target.value)}
                        />
                    </div>
                    <Button
                        type="submit"
                        variant="outline"
                        disabled={pending || !credits || note.trim().length < 3}
                        data-testid="grant-submit"
                    >
                        {t('plans.admin.tenant.submitGrant')}
                    </Button>
                </form>
            </CardContent>
        </Card>
    );
}

function Lots({ lots }: { lots: Schemas['AICreditLot'][] }) {
    const t = useT();
    const locale = useLocale();
    const fmt = useFormatDateTime();
    const c = (n: number) => formatCredits(n, locale);
    return (
        <div className="grid gap-2">
            <h3 className="text-sm font-semibold">
                {t('plans.admin.tenant.lots')}
            </h3>
            {lots.length === 0 ? (
                <p className="text-sm text-muted-foreground">
                    {t('plans.admin.tenant.lotsEmpty')}
                </p>
            ) : (
                <div className="overflow-x-auto rounded-lg border">
                    <Table data-testid="credit-lots">
                        <TableHeader>
                            <TableRow>
                                <TableHead />
                                <TableHead className="text-right">
                                    {t('plans.admin.tenant.granted')}
                                </TableHead>
                                <TableHead className="text-right">
                                    {t('plans.admin.tenant.used')}
                                </TableHead>
                                <TableHead className="text-right">
                                    {t('plans.admin.tenant.remaining')}
                                </TableHead>
                                <TableHead>
                                    {t('plans.admin.tenant.expires')}
                                </TableHead>
                                <TableHead>
                                    {t('plans.admin.tenant.note')}
                                </TableHead>
                            </TableRow>
                        </TableHeader>
                        <TableBody>
                            {lots.map((l) => (
                                <TableRow key={l.id}>
                                    <TableCell>
                                        <Badge variant="outline">
                                            {t(
                                                `plans.admin.tenant.source.${l.source}`,
                                            )}
                                        </Badge>
                                        {l.period_start && (
                                            <div className="mt-1 text-xs text-muted-foreground">
                                                {l.period_start.slice(0, 7)}
                                            </div>
                                        )}
                                    </TableCell>
                                    <TableCell className="text-right tabular-nums">
                                        {c(l.granted)}
                                    </TableCell>
                                    <TableCell className="text-right tabular-nums">
                                        {c(l.used)}
                                    </TableCell>
                                    <TableCell className="text-right tabular-nums">
                                        {c(l.remaining)}
                                    </TableCell>
                                    <TableCell className="text-sm whitespace-nowrap text-muted-foreground">
                                        {l.expires_at
                                            ? fmt(l.expires_at)
                                            : t('plans.admin.tenant.never')}
                                    </TableCell>
                                    <TableCell
                                        className="max-w-56 truncate text-sm"
                                        title={l.note}
                                    >
                                        {l.note}
                                    </TableCell>
                                </TableRow>
                            ))}
                        </TableBody>
                    </Table>
                </div>
            )}
        </div>
    );
}
