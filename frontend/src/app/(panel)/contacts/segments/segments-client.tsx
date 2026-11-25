'use client';

import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { Filter, UsersRound } from 'lucide-react';
import { ClientDataTable } from '@/components/client-data-table';
import { useFormatDateTime } from '@/components/display-preferences';
import { Button } from '@/components/ui/button';
import {
    Empty,
    EmptyDescription,
    EmptyHeader,
    EmptyMedia,
    EmptyTitle,
} from '@/components/ui/empty';
import { useT } from '@/i18n/client';
import type { Segment } from '@/lib/types';
import { countRules, fromDefinition } from './segment-tree';

export function SegmentTable({
    segments,
    canContacts,
}: {
    segments: Segment[];
    canContacts: boolean;
}) {
    const t = useT();
    const router = useRouter();
    const formatDateTime = useFormatDateTime();
    if (segments.length === 0) {
        return (
            <Empty className="border py-12">
                <EmptyHeader>
                    <EmptyMedia variant="icon">
                        <Filter />
                    </EmptyMedia>
                    <EmptyTitle>{t('contacts.segments.empty')}</EmptyTitle>
                    <EmptyDescription>
                        {t('contacts.segments.emptyHint')}
                    </EmptyDescription>
                </EmptyHeader>
            </Empty>
        );
    }
    const rules = new Map(
        segments.map((s) => [s.id, countRules(fromDefinition(s.definition))]),
    );
    return (
        <ClientDataTable
            rows={segments}
            rowId={(s) => s.id}
            storageKey="segments"
            initialSort="name"
            searchText={(s) => `${s.name} ${s.description}`}
            searchPlaceholder={t('table.search')}
            noMatch={
                <p className="p-8 text-center text-sm text-muted-foreground">
                    {t('table.noMatch')}
                </p>
            }
            columns={[
                {
                    id: 'name',
                    header: t('contacts.fields.name'),
                    hideable: false,
                    sortValue: (s) => s.name,
                    cell: (s) => (
                        <div className="min-w-0">
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
                        </div>
                    ),
                },
                {
                    id: 'rules',
                    header: t('contacts.segments.rules'),
                    className: 'text-right',
                    sortValue: (s) => rules.get(s.id) ?? 0,
                    cell: (s) => (
                        <span className="tabular-nums">{rules.get(s.id)}</span>
                    ),
                },
                {
                    id: 'updated',
                    header: t('contacts.segments.updated'),
                    sortValue: (s) => s.updated_at,
                    cell: (s) => (
                        <span className="text-sm whitespace-nowrap text-muted-foreground">
                            {formatDateTime(s.updated_at)}
                        </span>
                    ),
                },
            ]}
            rowActions={
                canContacts
                    ? (s) => (
                          <Button size="sm" variant="outline" asChild>
                              <Link
                                  href={`/contacts?segment_id=${s.id}`}
                                  aria-label={t('contacts.segments.showIn', {
                                      name: s.name,
                                  })}
                              >
                                  {t('contacts.segments.show')}
                              </Link>
                          </Button>
                      )
                    : undefined
            }
            rowMenu={(s) => [
                [
                    {
                        id: 'open',
                        label: s.name,
                        icon: Filter,
                        run: () => router.push(`/contacts/segments/${s.id}`),
                    },
                    ...(canContacts
                        ? [
                              {
                                  id: 'contacts',
                                  label: t('contacts.segments.showIn', {
                                      name: s.name,
                                  }),
                                  icon: UsersRound,
                                  run: () =>
                                      router.push(
                                          `/contacts?segment_id=${s.id}`,
                                      ),
                              },
                          ]
                        : []),
                ],
            ]}
            onRowClick={(s) => router.push(`/contacts/segments/${s.id}`)}
        />
    );
}
