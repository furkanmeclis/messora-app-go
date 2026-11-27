import type { Metadata } from 'next';
import { MailCheck } from 'lucide-react';
import { getT } from '@/i18n/server';
import { AuthCard } from '../_components/shell';
import { ConfirmEmailForm } from '../forms';

export async function generateMetadata(): Promise<Metadata> {
    const t = await getT();
    return { title: t('auth.confirmEmail.metaTitle') };
}

/**
 * The link sent to a new e-mail address. Confirming takes a click, so link
 * scanners that only fetch the page do not use the token up.
 */
export default async function ConfirmEmailPage({
    searchParams,
}: {
    searchParams: Promise<Record<string, string | undefined>>;
}) {
    const token = (await searchParams).token ?? '';
    const t = await getT();
    return (
        <AuthCard icon={MailCheck} title={t('auth.confirmEmail.title')}>
            <ConfirmEmailForm token={token} />
        </AuthCard>
    );
}
