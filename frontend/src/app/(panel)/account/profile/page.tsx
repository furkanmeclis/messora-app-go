import type { Metadata } from 'next';
import {
    Card,
    CardContent,
    CardDescription,
    CardHeader,
    CardTitle,
} from '@/components/ui/card';
import { getT } from '@/i18n/server';
import { requireMe } from '@/lib/server/auth';
import { PageHeading } from '../../panel-header';
import { ProfileForm } from './profile-form';

export async function generateMetadata(): Promise<Metadata> {
    const t = await getT();
    return { title: t('account.profile.title') };
}

export default async function ProfilePage() {
    const [me, t] = await Promise.all([requireMe(), getT()]);
    return (
        <>
            <PageHeading
                title={t('account.profile.title')}
                description={t('account.profile.description')}
            />
            <Card className="max-w-xl">
                <CardHeader>
                    <CardTitle>
                        {t('account.profile.personalDetails')}
                    </CardTitle>
                    <CardDescription>
                        {t('account.profile.signedInAs', {
                            email: me.user.email,
                            role: t(
                                `settings.security.roles.${me.user.role.key}`,
                            ),
                        })}
                    </CardDescription>
                </CardHeader>
                <CardContent>
                    <ProfileForm user={me.user} />
                </CardContent>
            </Card>
        </>
    );
}
