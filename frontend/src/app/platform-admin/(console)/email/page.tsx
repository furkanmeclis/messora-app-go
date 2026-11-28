import type { Metadata } from 'next';
import { Mail } from 'lucide-react';
import { getT } from '@/i18n/server';
import { papi, requirePlatformMe } from '@/lib/server/platform';
import type { Schemas } from '@/lib/types';
import { Heading } from '../heading';
import { EmailSenderForm } from './email-form';

export async function generateMetadata(): Promise<Metadata> {
    const t = await getT();
    return { title: t('plans.admin.email.title') };
}

/** The platform's e-mail sender (Resend over SMTP). */
export default async function PlatformEmailPage() {
    const [t, me, email] = await Promise.all([
        getT(),
        requirePlatformMe(),
        papi<Schemas['PlatformEmail']>('/platform/email').then((r) => r.data),
    ]);
    return (
        <>
            <Heading
                icon={<Mail />}
                title={t('plans.admin.email.title')}
                description={t('plans.admin.email.description')}
            />
            <EmailSenderForm email={email} defaultTestTo={me.admin.email} />
        </>
    );
}
