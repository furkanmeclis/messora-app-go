import { headers } from 'next/headers';
import { redirect } from 'next/navigation';
import { SidebarInset, SidebarProvider } from '@/components/ui/sidebar';
import { ConfirmProvider } from '@/components/confirm';
import { Shortcuts } from '@/components/shortcuts';
import { Desktop } from '@/components/desktop/desktop';
import { DesktopProvider } from '@/components/desktop/desktop-context';
import { ContextMenuProvider } from '@/components/context-menu';
import { DisplayPreferences } from '@/components/display-preferences';
import { getAppearance } from '@/lib/server/appearance';
import { requireMe } from '@/lib/server/auth';
import { RealtimeProvider } from '@/components/realtime';
import { ReauthProvider } from '@/components/reauth';
import { AppSidebar } from './app-sidebar';
import { api } from '@/lib/server/api';
import type { Campaign, Conversation } from '@/lib/types';
import { Topbar, type RunningCampaign } from './topbar';
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
    const can = (p: string) => me.permissions.includes(p);
    // Starting values for the live top bar; events keep them current. A
    // failure only means the bar starts empty.
    const [unread, running] = me.mfa_required
        ? [[], []]
        : await Promise.all([
              can('conversations.read') || can('conversations.read_all')
                  ? api<Conversation[]>(
                        '/conversations?status=open&unread=true&limit=100',
                    )
                        .then((r) => r.data.map((c) => c.id))
                        .catch(() => [] as string[])
                  : Promise.resolve([] as string[]),
              can('campaigns.read')
                  ? api<Campaign[]>('/campaigns?status=running&limit=20')
                        .then((r) =>
                            r.data.map((c): RunningCampaign => ({
                                id: c.id,
                                name: c.name,
                                total: Number(c.total_recipients),
                                counts: c.counts as Record<string, number>,
                            })),
                        )
                        .catch(() => [] as RunningCampaign[])
                  : Promise.resolve([] as RunningCampaign[]),
          ]);
    // Channels the role may read; the API checks the same rules again when
    // it issues each subscription token.
    const channels: string[] = [];
    // inbox:all names every conversation, so only read_all gets it; agents
    // follow the unassigned pool, and conversations assigned to them arrive
    // on their personal channel (subscribed server-side).
    if (me.permissions.includes('conversations.read_all')) {
        channels.push('inbox:all');
    } else if (me.permissions.includes('conversations.read')) {
        channels.push('inbox:unassigned');
    }
    if (me.permissions.includes('campaigns.read')) {
        channels.push('campaign:all');
    }
    return (
        <RealtimeProvider channels={channels} enabled={!me.mfa_required}>
            <DesktopProvider>
                <ContextMenuProvider>
                    <SidebarProvider>
                        <AppSidebar
                            user={me.user}
                            permissions={me.permissions}
                            organization={appearance.organization_name}
                            logoVersion={appearance.logo?.version}
                            restricted={me.mfa_required}
                        />
                        <SidebarInset>
                            <Shortcuts
                                permissions={me.permissions}
                                restricted={me.mfa_required}
                            />
                            <Topbar
                                userId={me.user.id}
                                permissions={me.permissions}
                                restricted={me.mfa_required}
                                initialUnread={unread}
                                initialRunning={running}
                            />
                            <ReauthProvider mfaEnabled={me.user.mfa_enabled}>
                                <ConfirmProvider>
                                    <div className="w-full flex-1 p-4 md:p-6 xl:px-8">
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
                                            {!me.mfa_required && (
                                                <Desktop
                                                    permissions={me.permissions}
                                                    wallpaper={
                                                        appearance.theme
                                                            .wallpaper
                                                    }
                                                    organization={
                                                        appearance.organization_name
                                                    }
                                                />
                                            )}
                                        </DisplayPreferences>
                                    </div>
                                </ConfirmProvider>
                            </ReauthProvider>
                        </SidebarInset>
                    </SidebarProvider>
                </ContextMenuProvider>
            </DesktopProvider>
        </RealtimeProvider>
    );
}
