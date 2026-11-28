'use client';

import Link from 'next/link';
import { Sparkles } from 'lucide-react';
import { toast } from 'sonner';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { useT } from '@/i18n/client';
import type { MessageKey, Translate } from '@/i18n/translate';
import { PLAN_PAGE, planError, type PlanFeature } from '@/lib/plan';

type ApiErrorLike = {
    code?: string;
    message?: string;
    details?: {
        field?: string | null;
        code?: string | null;
        message?: string | null;
    }[];
};

function label(t: Translate, key: string, fallback: string): string {
    const v = t(key as MessageKey);
    return v === key ? fallback : v;
}

/**
 * The friendly text of a plan error (limit reached, feature not in the
 * plan) from an API error body; null for other errors.
 */
export function planErrorText(
    t: Translate,
    e: ApiErrorLike | null | undefined,
): string | null {
    const p = planError(e?.code, e?.details);
    if (!p) return null;
    if (p.kind === 'feature') {
        return t('plans.upgrade.hint', {
            feature: label(t, `plans.features.${p.feature}.name`, p.feature),
        });
    }
    const resource = label(
        t,
        `plans.resources.${p.resource}`,
        p.resource,
    ).toLowerCase();
    if (p.limit !== null && p.current !== null) {
        return t('plans.upgrade.limit', {
            limit: p.limit,
            current: p.current,
            resource,
        });
    }
    return t('plans.upgrade.limitShort', { resource });
}

/**
 * Shows an API error body as a toast; plan errors get a friendly text and
 * a "see plans" action.
 */
export function toastApiError(
    t: Translate,
    e: ApiErrorLike | null | undefined,
    fallback: string,
) {
    const text = planErrorText(t, e);
    if (text) {
        toast.error(text, {
            action: {
                label: t('plans.upgrade.link'),
                onClick: () => window.location.assign(PLAN_PAGE),
            },
        });
        return;
    }
    toast.error(e?.message ?? fallback);
}

/** An inline note that a feature is not part of the plan. */
export function UpgradeHint({
    feature,
    children,
    testId,
}: {
    feature?: PlanFeature;
    children?: React.ReactNode;
    testId?: string;
}) {
    const t = useT();
    return (
        <Alert data-testid={testId ?? 'upgrade-hint'}>
            <Sparkles />
            <AlertTitle>{t('plans.upgrade.title')}</AlertTitle>
            <AlertDescription>
                <p>
                    {children ??
                        (feature &&
                            t('plans.upgrade.hint', {
                                feature: t(`plans.features.${feature}.name`),
                            }))}
                </p>
                <Link
                    href={PLAN_PAGE}
                    className="font-medium text-primary underline-offset-4 hover:underline"
                >
                    {t('plans.upgrade.link')}
                </Link>
            </AlertDescription>
        </Alert>
    );
}
