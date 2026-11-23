import { redirect } from 'next/navigation';
import { ConfirmProvider } from '@/components/confirm';
import { requireMe } from '@/lib/server/auth';

/** Full-screen first-run wizard, for the owner only. */
export default async function WizardLayout({
    children,
}: {
    children: React.ReactNode;
}) {
    const me = await requireMe({ allowMfaPending: true });
    if (me.mfa_required) redirect('/account/security?enroll=1');
    if (me.user.role.key !== 'owner') redirect('/');
    return <ConfirmProvider>{children}</ConfirmProvider>;
}
