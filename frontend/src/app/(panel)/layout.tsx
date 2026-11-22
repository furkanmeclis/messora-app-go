import { headers } from 'next/headers';
import { redirect } from 'next/navigation';
import { SidebarInset, SidebarProvider } from '@/components/ui/sidebar';
import { api } from '@/lib/server/api';
import { requireMe } from '@/lib/server/auth';
import type { Settings } from '@/lib/types';
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
    return (
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
    );
}
