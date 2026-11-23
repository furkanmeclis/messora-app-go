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
import { formatDateTime } from '@/lib/format';
import { api } from '@/lib/server/api';
import { can, requirePermission } from '@/lib/server/auth';
import type { Segment } from '@/lib/types';
import { PageHeading } from '../../panel-header';
import { countRules, fromDefinition } from './segment-tree';

export const metadata: Metadata = { title: 'Segments' };

export default async function SegmentsPage() {
    const me = await requirePermission('segments.read');
    const segments = (await api<Segment[]>('/segments')).data;
    const canManage = can(me, 'segments.manage');
    const canContacts = can(me, 'contacts.read');
    return (
        <>
            <PageHeading
                title="Segments"
                description="Saved rules that pick contacts automatically, e.g. for campaigns."
                actions={
                    canManage && (
                        <Button asChild>
                            <Link href="/contacts/segments/new">
                                <Plus /> New segment
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
                                <EmptyTitle>No segments yet</EmptyTitle>
                                <EmptyDescription>
                                    A segment is a saved filter such as “tagged
                                    VIP and wrote in the last 30 days”. It
                                    always reflects your current contacts.
                                </EmptyDescription>
                            </EmptyHeader>
                        </Empty>
                    ) : (
                        <Table>
                            <TableHeader>
                                <TableRow>
                                    <TableHead>Name</TableHead>
                                    <TableHead className="text-right">
                                        Rules
                                    </TableHead>
                                    <TableHead>Updated</TableHead>
                                    {canContacts && (
                                        <TableHead>
                                            <span className="sr-only">
                                                Contacts
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
                                            {formatDateTime(s.updated_at)}
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
                                                        aria-label={`Show contacts in ${s.name}`}
                                                    >
                                                        Show contacts
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
