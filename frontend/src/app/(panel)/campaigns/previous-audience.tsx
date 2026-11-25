'use client';

import { ShieldBan, TriangleAlert, Users } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { Checkbox } from '@/components/ui/checkbox';
import {
    Field,
    FieldDescription,
    FieldLabel,
    FieldLegend,
    FieldSet,
} from '@/components/ui/field';
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from '@/components/ui/select';
import { Spinner } from '@/components/ui/spinner';
import { useT } from '@/i18n/client';
import { NUMBER_LOCALE } from '@/lib/format';
import type { ActionResult } from '@/lib/types';
import type { Preflight } from './actions';
import {
    BLOCKED_CLASSES,
    SELECTABLE_CLASSES,
    unreachedLabel,
} from './campaign-ui';

export type FinishedCampaign = { id: string; name: string };

const fmt = (n: number) => n.toLocaleString(NUMBER_LOCALE);

/**
 * The "previous campaign's unreached customers" audience: an earlier,
 * finished campaign and the reasons to target again, with a live summary
 * from POST /campaigns/preflight (by_failure_class and eligible).
 */
export function PreviousAudience({
    campaigns,
    campaignId,
    classes,
    onCampaign,
    onClasses,
    summary,
    error,
}: {
    campaigns: FinishedCampaign[];
    campaignId: string;
    classes: string[];
    onCampaign: (id: string) => void;
    onClasses: (c: string[]) => void;
    /** undefined while counting; null before a campaign is chosen. */
    summary: ActionResult<Preflight> | undefined | null;
    error?: string;
}) {
    const t = useT();
    const counts = summary && summary.ok ? summary.data.by_failure_class : null;
    const toggle = (c: string, on: boolean) =>
        onClasses(
            on
                ? [...classes.filter((x) => x !== c), c]
                : classes.filter((x) => x !== c),
        );
    return (
        <div className="space-y-4" data-testid="previous-audience">
            <Field data-invalid={!!error && !campaignId}>
                <FieldLabel htmlFor="cw-previous">
                    {t('campaigns.engine.previous.campaign')}
                </FieldLabel>
                {campaigns.length === 0 ? (
                    <p className="text-sm text-muted-foreground">
                        {t('campaigns.engine.previous.none')}
                    </p>
                ) : (
                    <Select value={campaignId} onValueChange={onCampaign}>
                        <SelectTrigger id="cw-previous" className="w-full">
                            <SelectValue
                                placeholder={t(
                                    'campaigns.engine.previous.choose',
                                )}
                            />
                        </SelectTrigger>
                        <SelectContent>
                            {campaigns.map((c) => (
                                <SelectItem key={c.id} value={c.id}>
                                    {c.name}
                                </SelectItem>
                            ))}
                        </SelectContent>
                    </Select>
                )}
            </Field>
            <FieldSet>
                <FieldLegend variant="label">
                    {t('campaigns.engine.previous.classes')}
                </FieldLegend>
                <ul className="space-y-3">
                    {SELECTABLE_CLASSES.map((c) => {
                        const id = `cw-class-${c}`;
                        return (
                            <li key={c} className="flex items-start gap-3">
                                <Checkbox
                                    id={id}
                                    data-testid={`class-${c}`}
                                    checked={classes.includes(c)}
                                    onCheckedChange={(v) =>
                                        toggle(c, v === true)
                                    }
                                    className="mt-0.5"
                                />
                                <div className="space-y-0.5">
                                    <label
                                        htmlFor={id}
                                        className="flex flex-wrap items-center gap-2 text-sm font-medium"
                                    >
                                        {unreachedLabel(t, c)}
                                        {counts && (
                                            <Badge
                                                variant="outline"
                                                className="tabular-nums"
                                                data-testid={`class-count-${c}`}
                                            >
                                                {fmt(counts[c] ?? 0)}
                                            </Badge>
                                        )}
                                        {c === 'undeliverable' && (
                                            <span className="inline-flex items-center gap-1 text-xs text-amber-600 dark:text-amber-400">
                                                <TriangleAlert className="size-3" />
                                                {t(
                                                    'campaigns.engine.previous.undeliverableWarning',
                                                )}
                                            </span>
                                        )}
                                    </label>
                                    <FieldDescription>
                                        {t(`campaigns.engine.classHints.${c}`)}
                                    </FieldDescription>
                                </div>
                            </li>
                        );
                    })}
                </ul>
            </FieldSet>
            <div className="rounded-lg border border-dashed p-3 text-sm">
                <p className="mb-2 flex items-center gap-2 font-medium">
                    <ShieldBan className="size-4" />
                    {t('campaigns.engine.previous.hardBlocks')}
                </p>
                <ul className="space-y-1 text-muted-foreground">
                    {BLOCKED_CLASSES.map((c) => (
                        <li key={c}>
                            {unreachedLabel(t, c)}
                            {counts ? ` (${fmt(counts[c] ?? 0)})` : ''} —{' '}
                            {t(`campaigns.engine.classHints.${c}`)}
                        </li>
                    ))}
                </ul>
            </div>
            {error && (
                <p role="alert" className="text-sm text-destructive">
                    {error}
                </p>
            )}
            {summary !== null && (
                <p
                    className="flex items-center gap-2 text-sm text-muted-foreground"
                    role="status"
                    data-testid="previous-summary"
                >
                    {summary === undefined ? (
                        <>
                            <Spinner /> {t('campaigns.engine.previous.loading')}
                        </>
                    ) : summary.ok ? (
                        <>
                            <Users className="size-4" />
                            {t('campaigns.engine.previous.eligible', {
                                count: fmt(summary.data.eligible),
                            })}
                        </>
                    ) : (
                        summary.message
                    )}
                </p>
            )}
        </div>
    );
}
