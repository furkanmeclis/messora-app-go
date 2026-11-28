// The brand typefaces, bundled (no third-party request; the CSP allows
// fonts from this origin only).
import '@fontsource-variable/inter/wght.css';
import '@fontsource-variable/plus-jakarta-sans/wght.css';
import { PauseCircle } from 'lucide-react';
import { getT } from '@/i18n/server';
import { getAuthContext, getWorkspaceState } from '@/lib/server/auth-context';
import { AuthCard, AuthShell } from './_components/shell';

export default async function AuthLayout({
    children,
}: {
    children: React.ReactNode;
}) {
    // On a workspace address only: a suspended company shows why instead
    // of a sign-in form that cannot work.
    const { tenant } = await getAuthContext();
    const state = tenant ? await getWorkspaceState() : 'ok';
    if (state === 'suspended') {
        const t = await getT();
        return (
            <AuthShell>
                <AuthCard
                    icon={PauseCircle}
                    tone="danger"
                    title={t('auth.suspended.title')}
                    description={t('auth.suspended.body')}
                />
            </AuthShell>
        );
    }
    return <AuthShell>{children}</AuthShell>;
}
