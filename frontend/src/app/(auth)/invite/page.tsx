import type { Metadata } from 'next';
import { AlertCircle, UserPlus } from 'lucide-react';
import { getLocale, getT } from '@/i18n/server';
import { intlLocale } from '@/i18n/config';
import { api, ApiError } from '@/lib/server/api';
import { errorMessage } from '@/lib/server/actions';
import { getAppearance } from '@/lib/server/appearance';
import type { InvitationPreviewExtras } from '@/lib/auth/contracts';
import type { Schemas } from '@/lib/types';
import { FormAlert } from '../_components/fields';
import { AuthCard } from '../_components/shell';
import { AcceptInvitationForm } from '../forms';

export async function generateMetadata(): Promise<Metadata> {
    const t = await getT();
    return { title: t('auth.invite.metaTitle') };
}

type Preview = Schemas['InvitationPreview'] & InvitationPreviewExtras;

export default async function InvitePage({
    searchParams,
}: {
    searchParams: Promise<Record<string, string | undefined>>;
}) {
    const [t, locale, appearance] = await Promise.all([
        getT(),
        getLocale(),
        getAppearance(),
    ]);
    const token = (await searchParams).token ?? '';
    let invitation: Preview | null = null;
    let error = t('auth.invite.incomplete');
    if (token) {
        try {
            invitation = (
                await api<Preview>('/invitations/lookup', {
                    method: 'POST',
                    auth: 'none',
                    body: { token },
                })
            ).data;
        } catch (err) {
            if (!(err instanceof ApiError)) throw err;
            error = await errorMessage(err);
        }
    }

    if (!invitation) {
        return (
            <AuthCard
                icon={AlertCircle}
                tone="danger"
                title={t('auth.invite.invalidTitle')}
            >
                <FormAlert>
                    <AlertCircle
                        className="mt-0.5 size-4 shrink-0"
                        aria-hidden
                    />
                    <span>
                        {error} {t('auth.invite.askAdmin')}
                    </span>
                </FormAlert>
            </AuthCard>
        );
    }

    const workspace =
        invitation.tenant?.name ||
        (appearance.organization_name !== 'Messora'
            ? appearance.organization_name
            : null);
    const logo =
        invitation.tenant?.logo_url ??
        (appearance.logo
            ? `/bff/logo?v=${encodeURIComponent(appearance.logo.version ?? '')}`
            : null);
    const expires = new Intl.DateTimeFormat(intlLocale(locale), {
        dateStyle: 'medium',
        timeStyle: 'short',
    }).format(new Date(invitation.expires_at));

    return (
        <AuthCard
            icon={UserPlus}
            logoUrl={logo}
            title={
                workspace
                    ? t('auth.invite.titleWorkspace', { workspace })
                    : t('auth.invite.title')
            }
            description={
                <>
                    {invitation.invited_by?.name && (
                        <>
                            {t('auth.invite.invitedBy', {
                                name: invitation.invited_by.name,
                            })}
                            .{' '}
                        </>
                    )}
                    {t('auth.invite.invitedAs', { role: invitation.role.name })}
                </>
            }
        >
            <div className="flex flex-col gap-6">
                <dl className="grid grid-cols-2 gap-px overflow-hidden rounded-xl border bg-border text-sm">
                    {workspace && (
                        <div className="col-span-2 bg-card px-4 py-3">
                            <dt className="text-xs text-muted-foreground">
                                {t('auth.invite.workspace')}
                            </dt>
                            <dd className="mt-0.5 font-medium">{workspace}</dd>
                        </div>
                    )}
                    <div className="bg-card px-4 py-3">
                        <dt className="text-xs text-muted-foreground">
                            {t('auth.invite.role')}
                        </dt>
                        <dd className="mt-0.5 font-medium">
                            {invitation.role.name}
                        </dd>
                    </div>
                    <div className="bg-card px-4 py-3">
                        <dt className="text-xs text-muted-foreground">
                            {t('auth.invite.expires')}
                        </dt>
                        <dd className="mt-0.5 font-medium tabular-nums">
                            {expires}
                        </dd>
                    </div>
                </dl>
                <AcceptInvitationForm token={token} email={invitation.email} />
            </div>
        </AuthCard>
    );
}
