import type { Metadata } from 'next';
import Link from 'next/link';
import { Filter, Plus } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Card, CardContent } from '@/components/ui/card';
import {
    Empty,
    EmptyDescription,
    EmptyHeader,
    EmptyMedia,
    EmptyTitle,
} from '@/components/ui/empty';
import {
    Table,
    TableBody,
    TableCell,
    TableHead,
    TableHeader,
    TableRow,
} from '@/components/ui/table';
import { getT } from '@/i18n/server';
import { formatDateTime } from '@/lib/format';
import { getDisplayPrefs } from '@/lib/server/display';
import { api } from '@/lib/server/api';
import { can, requirePermission } from '@/lib/server/auth';
import type { Segment } from '@/lib/types';
import { PageHeading } from '../../panel-header';
import { countRules, fromDefinition } from './segment-tree';

export async function generateMetadata(): Promise<Metadata> {
    const t = await getT();
    return { title: t('contacts.nav.segments') };
}

export default async function SegmentsPage() {
    const me = await requirePermission('segments.read');
    const prefs = await getDisplayPrefs();
    const segments = (await api<Segment[]>('/segments')).data;
    const canManage = can(me, 'segments.manage');
    const canContacts = can(me, 'contacts.read');
    const t = await getT();
    return (
        <>
            <PageHeading
                title={t('contacts.nav.segments')}
                description={t('contacts.segments.description')}
                actions={
                    canManage && (
                        <Button asChild shortcut="n">
                            <Link href="/contacts/segments/new">
                                <Plus /> {t('contacts.segments.new')}
                            </Link>
                        </Button>
                    )
                }
            />
            <Card>
                <CardContent className="p-0">
                    {segments.length === 0 ? (
                        <Empty className="py-12">
                            <EmptyHeader>
                                <EmptyMedia variant="icon">
                                    <Filter />
                                </EmptyMedia>
                                <EmptyTitle>
                                    {t('contacts.segments.empty')}
                                </EmptyTitle>
                                <EmptyDescription>
                                    {t('contacts.segments.emptyHint')}
                                </EmptyDescription>
                            </EmptyHeader>
                        </Empty>
                    ) : (
                        <Table>
                            <TableHeader>
                                <TableRow>
                                    <TableHead>
                                        {t('contacts.fields.name')}
                                    </TableHead>
                                    <TableHead className="text-right">
                                        {t('contacts.segments.rules')}
                                    </TableHead>
                                    <TableHead>
                                        {t('contacts.segments.updated')}
                                    </TableHead>
                                    {canContacts && (
                                        <TableHead>
                                            <span className="sr-only">
                                                {t('contacts.title')}
                                            </span>
                                        </TableHead>
                                    )}
                                </TableRow>
                            </TableHeader>
                            <TableBody>
                                {segments.map((s) => (
                                    <TableRow key={s.id}>
                                        <TableCell>
                                            <Link
                                                href={`/contacts/segments/${s.id}`}
                                                className="font-medium hover:underline"
                                            >
                                                {s.name}
                                            </Link>
                                            {s.description && (
                                                <div className="max-w-md truncate text-xs text-muted-foreground">
                                                    {s.description}
                                                </div>
                                            )}
                                        </TableCell>
                                        <TableCell className="text-right tabular-nums">
                                            {countRules(
                                                fromDefinition(s.definition),
                                            )}
                                        </TableCell>
                                        <TableCell className="text-sm text-muted-foreground">
                                            {formatDateTime(
                                                s.updated_at,
                                                prefs,
                                            )}
                                        </TableCell>
                                        {canContacts && (
                                            <TableCell className="text-right">
                                                <Button
                                                    size="sm"
                                                    variant="outline"
                                                    asChild
                                                >
                                                    <Link
                                                        href={`/contacts?segment_id=${s.id}`}
                                                        aria-label={t(
                                                            'contacts.segments.showIn',
                                                            { name: s.name },
                                                        )}
                                                    >
                                                        {t(
                                                            'contacts.segments.show',
                                                        )}
                                                    </Link>
                                                </Button>
                                            </TableCell>
                                        )}
                                    </TableRow>
                                ))}
                            </TableBody>
                        </Table>
                    )}
                </CardContent>
            </Card>
        </>
    );
}
