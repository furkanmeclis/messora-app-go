import { headers } from 'next/headers';
import { redirect } from 'next/navigation';
import { SidebarInset, SidebarProvider } from '@/components/ui/sidebar';
import { ConfirmProvider } from '@/components/confirm';
import { DisplayPreferences } from '@/components/display-preferences';
import { getAppearance } from '@/lib/server/appearance';
import { requireMe } from '@/lib/server/auth';
import { RealtimeProvider } from '@/components/realtime';
import { ReauthProvider } from '@/components/reauth';
import { AppSidebar } from './app-sidebar';
import { PanelHeader } from './panel-header';
import { SetupBanner } from './setup-banner';

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
    // Name, logo and formats are public appearance settings (never fail:
    // getAppearance falls back to defaults).
    const appearance = await getAppearance();
    const isOwner = me.user.role.key === 'owner';
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
                    organization={appearance.organization_name}
                    logoVersion={appearance.logo?.version}
                    restricted={me.mfa_required}
                />
                <SidebarInset>
                    <PanelHeader />
                    <ReauthProvider mfaEnabled={me.user.mfa_enabled}>
                        <ConfirmProvider>
                            <div className="mx-auto w-full max-w-6xl flex-1 p-4 md:p-6">
                                {isOwner &&
                                    !appearance.setup_completed &&
                                    !me.mfa_required && (
                                        <SetupBanner
                                            organization={
                                                appearance.organization_name
                                            }
                                        />
                                    )}
                                <DisplayPreferences
                                    timeZone={me.user.timezone}
                                    locale={me.user.locale}
                                    dateFormat={appearance.date_format}
                                    timeFormat={appearance.time_format}
                                >
                                    {children}
                                </DisplayPreferences>
                            </div>
                        </ConfirmProvider>
                    </ReauthProvider>
                </SidebarInset>
            </SidebarProvider>
        </RealtimeProvider>
    );
}
