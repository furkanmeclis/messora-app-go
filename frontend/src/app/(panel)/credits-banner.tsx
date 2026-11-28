import Link from 'next/link';
import { Sparkles } from 'lucide-react';
import { getT } from '@/i18n/server';

/**
 * Shown on every page while the workspace's AI credits of the month are
 * used up: AI with Messora's keys is paused until the reset or a top-up.
 */
export async function CreditsBanner() {
    const t = await getT();
    return (
        <div
            role="status"
            data-testid="credits-banner"
            className="mb-4 flex flex-wrap items-center gap-x-3 gap-y-1.5 rounded-xl border border-warning/50 bg-warning/10 px-4 py-2.5 text-sm"
        >
            <Sparkles className="size-4 shrink-0 text-warning" aria-hidden />
            <strong className="font-semibold">
                {t('plans.banner.credits')}
            </strong>
            <Link
                href="/settings/plan"
                className="ml-auto font-medium text-primary underline-offset-4 hover:underline"
            >
                {t('plans.banner.link')}
            </Link>
        </div>
    );
}
