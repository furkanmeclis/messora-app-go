import type { Metadata } from 'next';
import { getT } from '@/i18n/server';
import { api } from '@/lib/server/api';
import { requirePermission } from '@/lib/server/auth';
import type { Schemas } from '@/lib/types';
import { MetaRates } from './meta-rates-client';

export async function generateMetadata(): Promise<Metadata> {
    const t = await getT();
    return { title: t('settings.tabs.metaRates') };
}

export default async function MetaRatesPage() {
    const me = await requirePermission('settings.read');
    const [rates, t] = await Promise.all([
        api<Schemas['MetaRate'][]>('/settings/meta-rates'),
        getT(),
    ]);
    const publicUrl = `${me.tenant?.url ?? ''}/api/v1/meta-rates`;
    return (
        <div className="max-w-6xl space-y-4">
            <p
                className="text-sm text-muted-foreground"
                data-testid="meta-rates-public"
            >
                {t('plans.metaRates.publicJson', { url: '' })}
                <a
                    href={publicUrl}
                    target="_blank"
                    rel="noreferrer"
                    className="font-mono break-all text-primary underline-offset-4 hover:underline"
                >
                    {publicUrl}
                </a>
            </p>
            <MetaRates rates={rates.data} readOnly />
        </div>
    );
}
