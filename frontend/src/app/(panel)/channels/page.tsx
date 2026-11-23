import type { Metadata } from 'next';
import {
    Card,
    CardContent,
    CardDescription,
    CardHeader,
    CardTitle,
} from '@/components/ui/card';
import { api } from '@/lib/server/api';
import { can, requirePermission } from '@/lib/server/auth';
import type { PhoneNumber, WebhookSetup, WhatsAppAccount } from '@/lib/types';
import { getT } from '@/i18n/server';
import { PageHeading } from '../panel-header';
import {
    AccountList,
    ConnectAccountDialog,
    PhoneNumberTable,
    WebhookCard,
} from './channels-client';

export async function generateMetadata(): Promise<Metadata> {
    const t = await getT();
    return { title: t('channels.title') };
}

export default async function ChannelsPage() {
    const me = await requirePermission('channels.read');
    const manage = can(me, 'channels.manage');
    const t = await getT();
    const [accounts, numbers, webhook] = await Promise.all([
        api<WhatsAppAccount[]>('/channels/accounts').then((r) => r.data),
        api<PhoneNumber[]>('/channels/phone-numbers').then((r) => r.data),
        manage
            ? api<WebhookSetup>('/channels/webhook').then((r) => r.data)
            : Promise.resolve(null),
    ]);

    return (
        <>
            <PageHeading
                title={t('channels.title')}
                description={t('channels.description')}
                actions={manage ? <ConnectAccountDialog /> : undefined}
            />
            <div className="space-y-6">
                <Card>
                    <CardHeader>
                        <CardTitle>{t('channels.accounts.title')}</CardTitle>
                        <CardDescription>
                            {t('channels.accounts.description')}
                        </CardDescription>
                    </CardHeader>
                    <CardContent>
                        <AccountList accounts={accounts} manage={manage} />
                    </CardContent>
                </Card>
                <Card>
                    <CardHeader>
                        <CardTitle>{t('channels.numbers.title')}</CardTitle>
                        <CardDescription>
                            {t('channels.numbers.description')}
                        </CardDescription>
                    </CardHeader>
                    <CardContent className="p-0">
                        <PhoneNumberTable numbers={numbers} manage={manage} />
                    </CardContent>
                </Card>
                {webhook && <WebhookCard setup={webhook} />}
            </div>
        </>
    );
}
