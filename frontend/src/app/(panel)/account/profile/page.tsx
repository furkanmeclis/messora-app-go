import type { Metadata } from 'next';
import {
    Card,
    CardContent,
    CardDescription,
    CardHeader,
    CardTitle,
} from '@/components/ui/card';
import { requireMe } from '@/lib/server/auth';
import { PageHeading } from '../../panel-header';
import { ProfileForm } from './profile-form';

export const metadata: Metadata = { title: 'Profile' };

export default async function ProfilePage() {
    const me = await requireMe();
    return (
        <>
            <PageHeading
                title="Profile"
                description="How you appear to your team."
            />
            <Card className="max-w-xl">
                <CardHeader>
                    <CardTitle>Personal details</CardTitle>
                    <CardDescription>
                        Signed in as {me.user.email} ({me.user.role.name}).
                    </CardDescription>
                </CardHeader>
                <CardContent>
                    <ProfileForm user={me.user} />
                </CardContent>
            </Card>
        </>
    );
}
