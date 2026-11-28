import type { Metadata } from 'next';
import Link from 'next/link';
import { Users } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import {
    Table,
    TableBody,
    TableCell,
    TableHead,
    TableHeader,
    TableRow,
} from '@/components/ui/table';
import { getLocale, getT } from '@/i18n/server';
import { intlLocale } from '@/i18n/config';
import { papi } from '@/lib/server/platform';
import type { Schemas } from '@/lib/types';
import { Heading } from '../heading';
import { UserBadges } from './user-badges';

export async function generateMetadata(): Promise<Metadata> {
    const t = await getT();
    return { title: t('platform.users.title') };
}

export default async function UsersPage({
    searchParams,
}: {
    searchParams: Promise<Record<string, string | undefined>>;
}) {
    const sp = await searchParams;
    const q = new URLSearchParams({ limit: '50' });
    if (sp.q) q.set('q', sp.q);
    if (sp.cursor) q.set('cursor', sp.cursor);
    const [t, locale, page] = await Promise.all([
        getT(),
        getLocale(),
        papi<Schemas['PlatformUser'][]>(`/platform/users?${q}`),
    ]);
    const fmt = new Intl.DateTimeFormat(intlLocale(locale), {
        dateStyle: 'medium',
        timeStyle: 'short',
    });
    const next = new URLSearchParams();
    if (sp.q) next.set('q', sp.q);
    if (page.nextCursor) next.set('cursor', page.nextCursor);
    return (
        <>
            <Heading
                icon={<Users />}
                title={t('platform.users.title')}
                description={t('platform.users.description')}
            />
            <form className="mb-4 flex gap-2" action="/users">
                <Input
                    name="q"
                    defaultValue={sp.q}
                    placeholder={t('platform.users.search')}
                    aria-label={t('platform.users.search')}
                    className="max-w-md"
                />
                <Button type="submit" variant="outline">
                    {t('common.search')}
                </Button>
            </form>
            <div className="overflow-x-auto rounded-xl border bg-card">
                <Table>
                    <TableHeader>
                        <TableRow>
                            <TableHead>
                                {t('platform.users.columns.name')}
                            </TableHead>
                            <TableHead>
                                {t('platform.users.columns.status')}
                            </TableHead>
                            <TableHead className="text-right">
                                {t('platform.users.columns.memberships')}
                            </TableHead>
                            <TableHead>
                                {t('platform.users.columns.lastLogin')}
                            </TableHead>
                        </TableRow>
                    </TableHeader>
                    <TableBody>
                        {page.data.length === 0 && (
                            <TableRow>
                                <TableCell
                                    colSpan={4}
                                    className="py-10 text-center text-muted-foreground"
                                >
                                    {t('platform.users.empty')}
                                </TableCell>
                            </TableRow>
                        )}
                        {page.data.map((u) => (
                            <TableRow key={u.id} data-testid="user-row">
                                <TableCell>
                                    <Link
                                        href={`/users/${u.id}`}
                                        className="font-medium hover:underline"
                                    >
                                        {u.name}
                                    </Link>
                                    <div className="text-xs text-muted-foreground">
                                        {u.email}
                                    </div>
                                </TableCell>
                                <TableCell className="space-x-1">
                                    <UserBadges user={u} t={t} />
                                </TableCell>
                                <TableCell className="text-right tabular-nums">
                                    {u.membership_count}
                                </TableCell>
                                <TableCell className="text-sm whitespace-nowrap text-muted-foreground">
                                    {u.last_login_at
                                        ? fmt.format(new Date(u.last_login_at))
                                        : t('platform.users.never')}
                                </TableCell>
                            </TableRow>
                        ))}
                    </TableBody>
                </Table>
            </div>
            {page.nextCursor && (
                <Button asChild variant="outline" className="mt-4">
                    <Link href={`/users?${next}`}>
                        {t('platform.tenants.more')}
                    </Link>
                </Button>
            )}
        </>
    );
}
