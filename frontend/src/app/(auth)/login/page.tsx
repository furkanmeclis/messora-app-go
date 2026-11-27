import type { Metadata } from 'next';
import { redirect } from 'next/navigation';
import { CheckCircle2, Clock } from 'lucide-react';
import { getT } from '@/i18n/server';
import { safeNext } from '@/lib/server/actions';
import { getAuthContext, getCapabilities } from '@/lib/server/auth-context';
import { FormAlert } from '../_components/fields';
import { AuthCard, AuthLink } from '../_components/shell';
import { LoginForm } from '../forms';

export async function generateMetadata(): Promise<Metadata> {
    const [t, { tenant }] = await Promise.all([getT(), getAuthContext()]);
    return {
        title: tenant
            ? t('auth.login.titleTenant', { name: tenant.name })
            : t('auth.login.title'),
    };
}

export default async function LoginPage({
    searchParams,
}: {
    searchParams: Promise<Record<string, string | undefined>>;
}) {
    const sp = await searchParams;
    const [caps, { tenant }, t] = await Promise.all([
        getCapabilities(),
        getAuthContext(),
        getT(),
    ]);
    if (caps.setup_required) redirect('/setup');

    const notice = sp.reset
        ? { icon: CheckCircle2, text: t('auth.login.passwordChanged') }
        : sp.verified
          ? { icon: CheckCircle2, text: t('auth.login.verified') }
          : sp.expired
            ? { icon: Clock, text: t('auth.login.expired') }
            : null;

    return (
        <AuthCard
            title={
                tenant
                    ? t('auth.login.titleTenant', { name: tenant.name })
                    : t('auth.login.title')
            }
            description={
                tenant
                    ? t('auth.login.descriptionTenant')
                    : t('auth.login.description')
            }
            logoUrl={tenant?.logoUrl}
            footer={
                // Workspaces invite their people; sign-up lives on the
                // platform address.
                tenant ? undefined : (
                    <>
                        {t('auth.login.noAccount')}{' '}
                        <AuthLink href="/signup">
                            {t('auth.login.signup')}
                        </AuthLink>
                    </>
                )
            }
        >
            <div className="flex flex-col gap-5">
                {notice && (
                    <FormAlert tone={sp.expired ? 'info' : 'success'}>
                        <notice.icon
                            className="mt-0.5 size-4 shrink-0"
                            aria-hidden
                        />
                        <span>{notice.text}</span>
                    </FormAlert>
                )}
                <LoginForm
                    next={safeNext(sp.next)}
                    canReset={caps.password_reset}
                />
            </div>
        </AuthCard>
    );
}
