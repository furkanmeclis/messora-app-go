import { headers } from 'next/headers';
import { redirect } from 'next/navigation';
import { SidebarInset, SidebarProvider } from '@/components/ui/sidebar';
import { api } from '@/lib/server/api';
import { requireMe } from '@/lib/server/auth';
import type { Settings } from '@/lib/types';
import { RealtimeProvider } from '@/components/realtime';
import { ReauthProvider } from '@/components/reauth';
import { AppSidebar } from './app-sidebar';
import { PanelHeader } from './panel-header';

export default async function PanelLayout({
    children,
}: {
    children: React.ReactNode;
}) {
    const me = await requireMe({ allowMfaPending: true });
    const path = (await headers()).get('x-pathname') ?? '/';
    // Roles with mandatory two-factor authentication must enrol before
    // anything else; the API enforces the same rule.
    if (me.mfa_required && path !== '/account/security') {
        redirect('/account/security?enroll=1');
    }
    // While a mandatory second factor is pending the API only allows
    // enrolment, so nothing else is fetched. A failure here must never take
    // the whole panel down: the name is cosmetic.
    let organization = 'Messora';
    if (!me.mfa_required && me.permissions.includes('settings.read')) {
        organization = await api<Settings>('/settings')
            .then((r) => r.data.organization_name)
            .catch(() => organization);
    }
    // Channels the role may read; the API checks the same rules again when
    // it issues each subscription token.
    const channels: string[] = [];
    if (
        me.permissions.includes('conversations.read') ||
        me.permissions.includes('conversations.read_all')
    ) {
        channels.push('inbox:all');
    }
    if (me.permissions.includes('campaigns.read')) {
        channels.push('campaign:all');
    }
    return (
        <RealtimeProvider channels={channels} enabled={!me.mfa_required}>
            <SidebarProvider>
                <AppSidebar
                    user={me.user}
                    permissions={me.permissions}
                    organization={organization}
                    restricted={me.mfa_required}
                />
                <SidebarInset>
                    <PanelHeader />
                    <ReauthProvider mfaEnabled={me.user.mfa_enabled}>
                        <div className="mx-auto w-full max-w-6xl flex-1 p-4 md:p-6">
                            {children}
                        </div>
                    </ReauthProvider>
                </SidebarInset>
            </SidebarProvider>
        </RealtimeProvider>
    );
}
