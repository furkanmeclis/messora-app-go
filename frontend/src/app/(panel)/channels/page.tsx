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
import { PageHeading } from '../panel-header';
import {
    AccountList,
    ConnectAccountDialog,
    PhoneNumberTable,
    WebhookCard,
} from './channels-client';

export const metadata: Metadata = { title: 'Channels' };

export default async function ChannelsPage() {
    const me = await requirePermission('channels.read');
    const manage = can(me, 'channels.manage');
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
                title="Channels"
                description="WhatsApp Business accounts, their phone numbers and the webhook Meta calls."
                actions={manage ? <ConnectAccountDialog /> : undefined}
            />
            <div className="space-y-6">
                <Card>
                    <CardHeader>
                        <CardTitle>WhatsApp accounts</CardTitle>
                        <CardDescription>
                            Access tokens and app secrets are stored encrypted
                            and are never shown again.
                        </CardDescription>
                    </CardHeader>
                    <CardContent>
                        <AccountList accounts={accounts} manage={manage} />
                    </CardContent>
                </Card>
                <Card>
                    <CardHeader>
                        <CardTitle>Phone numbers</CardTitle>
                        <CardDescription>
                            The default number sends whenever a request or
                            campaign names no sender.
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
