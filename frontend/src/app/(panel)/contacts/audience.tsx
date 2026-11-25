import 'server-only';

import Link from 'next/link';
import { ArrowLeft, Lock } from 'lucide-react';
import { AudienceInsightsView } from '@/components/audience-insights';
import { Button } from '@/components/ui/button';
import {
    Empty,
    EmptyDescription,
    EmptyHeader,
    EmptyMedia,
    EmptyTitle,
} from '@/components/ui/empty';
import { getT } from '@/i18n/server';
import { api } from '@/lib/server/api';
import { can } from '@/lib/server/auth';
import type { Me, Schemas } from '@/lib/types';

/** Insights of a group, tag or segment (or why they are not shown). */
export async function AudienceInsightsSection({
    me,
    path,
    contactsHref,
}: {
    me: Me;
    path: string;
    contactsHref?: string;
}) {
    const t = await getT();
    if (!can(me, 'campaigns.read')) {
        return (
            <Empty className="rounded-xl border py-12">
                <EmptyHeader>
                    <EmptyMedia variant="icon">
                        <Lock />
                    </EmptyMedia>
                    <EmptyTitle>{t('insights.noAccess')}</EmptyTitle>
                </EmptyHeader>
            </Empty>
        );
    }
    const tz = encodeURIComponent(me.user.timezone || 'UTC');
    const data = (
        await api<Schemas['AudienceInsights']>(`${path}/insights?tz=${tz}`)
    ).data;
    return <AudienceInsightsView data={data} contactsHref={contactsHref} />;
}

export async function NotFoundAudience({
    title,
    back,
}: {
    title: string;
    back: { href: string; label: string };
}) {
    const t = await getT();
    return (
        <Empty className="py-16">
            <EmptyHeader>
                <EmptyMedia variant="icon">
                    <Lock />
                </EmptyMedia>
                <EmptyTitle>{title}</EmptyTitle>
                <EmptyDescription>
                    {t('contacts.detail.notFoundHint')}
                </EmptyDescription>
            </EmptyHeader>
            <Button variant="outline" asChild>
                <Link href={back.href}>
                    <ArrowLeft /> {back.label}
                </Link>
            </Button>
        </Empty>
    );
}
