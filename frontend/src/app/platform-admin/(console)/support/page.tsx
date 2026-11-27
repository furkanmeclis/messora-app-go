import type { Metadata } from 'next';
import Link from 'next/link';
import { LifeBuoy } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
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
import { EndSupport } from './end-support';

export async function generateMetadata(): Promise<Metadata> {
    const t = await getT();
    return { title: t('platform.support.title') };
}

export default async function SupportPage() {
    const [t, locale, grants] = await Promise.all([
        getT(),
        getLocale(),
        papi<Schemas['SupportGrant'][]>('/platform/support-sessions').then(
            (r) => r.data,
        ),
    ]);
    const fmt = new Intl.DateTimeFormat(intlLocale(locale), {
        dateStyle: 'short',
        timeStyle: 'short',
    });
    return (
        <>
            <Heading
                icon={<LifeBuoy />}
                title={t('platform.support.title')}
                description={t('platform.support.description')}
            />
            <div className="overflow-x-auto rounded-xl border bg-card">
                <Table>
                    <TableHeader>
                        <TableRow>
                            <TableHead>
                                {t('platform.support.columns.company')}
                            </TableHead>
                            <TableHead>
                                {t('platform.support.columns.admin')}
                            </TableHead>
                            <TableHead>
                                {t('platform.support.columns.reason')}
                            </TableHead>
                            <TableHead>
                                {t('platform.support.columns.access')}
                            </TableHead>
                            <TableHead>
                                {t('platform.support.columns.window')}
                            </TableHead>
                            <TableHead />
                        </TableRow>
                    </TableHeader>
                    <TableBody>
                        {grants.length === 0 && (
                            <TableRow>
                                <TableCell
                                    colSpan={6}
                                    className="py-10 text-center text-muted-foreground"
                                >
                                    {t('platform.support.empty')}
                                </TableCell>
                            </TableRow>
                        )}
                        {grants.map((g) => (
                            <TableRow key={g.id} data-testid="support-row">
                                <TableCell>
                                    <Link
                                        href={`/tenants/${g.tenant.id}`}
                                        className="font-medium hover:underline"
                                    >
                                        {g.tenant.name}
                                    </Link>
                                </TableCell>
                                <TableCell className="text-sm">
                                    {g.admin.email}
                                </TableCell>
                                <TableCell className="max-w-72 truncate text-sm">
                                    {g.reason}
                                </TableCell>
                                <TableCell className="space-x-1">
                                    <Badge variant="outline">
                                        {g.read_only
                                            ? t('platform.support.readOnly')
                                            : t('platform.support.write')}
                                    </Badge>
                                    {g.active && (
                                        <Badge>
                                            {t('platform.support.active')}
                                        </Badge>
                                    )}
                                </TableCell>
                                <TableCell className="text-sm whitespace-nowrap text-muted-foreground tabular-nums">
                                    {fmt.format(new Date(g.created_at))} –{' '}
                                    {fmt.format(
                                        new Date(g.ended_at ?? g.expires_at),
                                    )}
                                </TableCell>
                                <TableCell className="text-right">
                                    {g.active && <EndSupport id={g.id} />}
                                </TableCell>
                            </TableRow>
                        ))}
                    </TableBody>
                </Table>
            </div>
        </>
    );
}
