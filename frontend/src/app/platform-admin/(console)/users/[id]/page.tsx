import type { Metadata } from 'next';
import Link from 'next/link';
import { notFound } from 'next/navigation';
import { ArrowLeft, UserRound } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { getT } from '@/i18n/server';
import { ApiError } from '@/lib/server/api';
import { papi } from '@/lib/server/platform';
import type { Schemas } from '@/lib/types';
import { Heading } from '../../heading';
import { UserBadges } from '../user-badges';
import { UserActions } from './user-actions';

export async function generateMetadata(): Promise<Metadata> {
    const t = await getT();
    return { title: t('platform.users.title') };
}

export default async function UserPage({
    params,
}: {
    params: Promise<{ id: string }>;
}) {
    const { id } = await params;
    const [t, detail] = await Promise.all([
        getT(),
        papi<Schemas['PlatformUserDetail']>(`/platform/users/${id}`)
            .then((r) => r.data)
            .catch((err) => {
                if (
                    err instanceof ApiError &&
                    (err.status === 404 || err.status === 422)
                )
                    return null;
                throw err;
            }),
    ]);
    if (!detail) notFound();
    const u = detail.user;
    return (
        <>
            <Link
                href="/users"
                className="mb-4 inline-flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground"
            >
                <ArrowLeft className="size-4" /> {t('platform.user.back')}
            </Link>
            <Heading
                icon={<UserRound />}
                title={u.name}
                description={u.email}
            />
            <div className="mb-6 flex flex-wrap gap-1">
                <UserBadges user={u} t={t} />
            </div>
            <div className="grid items-start gap-6 xl:grid-cols-[minmax(0,1fr)_22rem]">
                <Card>
                    <CardHeader>
                        <CardTitle>{t('platform.user.memberships')}</CardTitle>
                    </CardHeader>
                    <CardContent>
                        {detail.memberships.length === 0 ? (
                            <p className="text-sm text-muted-foreground">
                                {t('platform.user.none')}
                            </p>
                        ) : (
                            <ul className="grid gap-2">
                                {detail.memberships.map((m) => (
                                    <li
                                        key={m.tenant.id}
                                        className="flex flex-wrap items-center gap-2 rounded-lg border p-3 text-sm"
                                    >
                                        <Link
                                            href={`/tenants/${m.tenant.id}`}
                                            className="font-medium hover:underline"
                                        >
                                            {m.tenant.name}
                                        </Link>
                                        <span className="font-mono text-xs text-muted-foreground">
                                            {m.tenant.slug}
                                        </span>
                                        <Badge
                                            variant="outline"
                                            className="ml-auto"
                                        >
                                            {t(
                                                `settings.security.roles.${m.role as 'owner'}`,
                                            )}
                                        </Badge>
                                        {m.status !== 'active' && (
                                            <Badge variant="outline">
                                                {t('platform.users.disabled')}
                                            </Badge>
                                        )}
                                    </li>
                                ))}
                            </ul>
                        )}
                    </CardContent>
                </Card>
                <UserActions user={u} />
            </div>
        </>
    );
}
