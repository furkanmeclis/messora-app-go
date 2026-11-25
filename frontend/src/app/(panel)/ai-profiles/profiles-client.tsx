'use client';

import { useTransition } from 'react';
import { useRouter } from 'next/navigation';
import { Bot, Pencil, Trash2, TriangleAlert } from 'lucide-react';
import { ClientDataTable } from '@/components/client-data-table';
import { useConfirm } from '@/components/confirm';
import { notify } from '@/components/notify';
import { Badge } from '@/components/ui/badge';
import {
    Empty,
    EmptyDescription,
    EmptyHeader,
    EmptyMedia,
    EmptyTitle,
} from '@/components/ui/empty';
import { useT } from '@/i18n/client';
import type { Schemas } from '@/lib/types';
import { deleteProfileAction } from './actions';

type Profile = Schemas['AIProfile'];

/** AI profiles; a row opens the full-page editor. */
export function ProfilesTable({
    profiles,
    providers,
    settings,
    canManage,
}: {
    profiles: Profile[];
    providers: Schemas['AIProvider'][];
    settings: Schemas['AISettings'];
    canManage: boolean;
}) {
    const t = useT();
    const router = useRouter();
    const ask = useConfirm();
    const [, start] = useTransition();
    const providerName = (id: string | null) =>
        providers.find((p) => p.id === id)?.name;

    const model = (p: Profile) => {
        if (p.provider_id)
            return `${providerName(p.provider_id) ?? '?'} · ${p.model ?? '—'}`;
        const base = settings.default_provider_id
            ? `${providerName(settings.default_provider_id) ?? '?'} · ${p.model ?? settings.default_model}`
            : '—';
        return p.model ? base : `${t('ai.profiles.companyDefault')} (${base})`;
    };

    const remove = async (p: Profile) => {
        const ok = await ask({
            title: t('ai.profiles.deleteTitle', { name: p.name }),
            description: t('ai.profiles.deleteBody'),
            confirmLabel: t('ai.profiles.delete'),
            destructive: true,
        });
        if (ok)
            start(async () => {
                if (notify(await deleteProfileAction(p.id))) router.refresh();
            });
    };

    return (
        <ClientDataTable
            rows={profiles}
            rowId={(p) => p.id}
            storageKey="ai-profiles"
            initialSort="name"
            rowTestId="ai-profile-row"
            searchText={(p) => `${p.name} ${p.phone_display}`}
            searchPlaceholder={t('table.search')}
            onRowClick={(p) => router.push(`/ai-profiles/${p.id}`)}
            empty={
                <Empty className="py-12">
                    <EmptyHeader>
                        <EmptyMedia variant="icon">
                            <Bot />
                        </EmptyMedia>
                        <EmptyTitle>{t('ai.profiles.empty')}</EmptyTitle>
                        <EmptyDescription>
                            {t('ai.profiles.emptyHint')}
                        </EmptyDescription>
                    </EmptyHeader>
                </Empty>
            }
            noMatch={
                <p className="p-8 text-center text-sm text-muted-foreground">
                    {t('table.noMatch')}
                </p>
            }
            columns={[
                {
                    id: 'name',
                    header: t('ai.profiles.columns.name'),
                    hideable: false,
                    sortValue: (p) => p.name,
                    cell: (p) => (
                        <div className="min-w-0">
                            <div className="flex items-center gap-2 font-medium">
                                {p.name}
                                {p.warnings.includes('disclosure_off') && (
                                    <TriangleAlert
                                        className="size-4 text-warning"
                                        aria-label={t(
                                            'ai.profiles.warnings.disclosure_off',
                                        )}
                                    />
                                )}
                            </div>
                        </div>
                    ),
                },
                {
                    id: 'number',
                    header: t('ai.profiles.columns.number'),
                    sortValue: (p) => p.phone_display,
                    cell: (p) => (
                        <span className="text-sm whitespace-nowrap">
                            {p.phone_display || (
                                <span className="text-muted-foreground">
                                    {t('ai.profiles.noNumber')}
                                </span>
                            )}
                        </span>
                    ),
                },
                {
                    id: 'model',
                    header: t('ai.profiles.columns.model'),
                    cell: (p) => (
                        <span className="text-sm text-muted-foreground">
                            {model(p)}
                        </span>
                    ),
                },
                {
                    id: 'autoReply',
                    header: t('ai.profiles.columns.autoReply'),
                    sortValue: (p) => (p.auto_reply ? 1 : 0),
                    cell: (p) => (
                        <span className="text-sm">
                            {p.auto_reply ? t('ai.test.yes') : t('ai.test.no')}
                        </span>
                    ),
                },
                {
                    id: 'status',
                    header: t('ai.profiles.columns.status'),
                    sortValue: (p) => p.status,
                    cell: (p) => (
                        <Badge
                            variant="outline"
                            className={
                                p.status === 'active'
                                    ? 'border-success/40 bg-success/10 text-success'
                                    : ''
                            }
                        >
                            {t(`ai.profiles.status.${p.status}`)}
                        </Badge>
                    ),
                },
            ]}
            rowMenu={(p) => [
                [
                    {
                        id: 'edit',
                        label: t('common.edit'),
                        icon: Pencil,
                        run: () => router.push(`/ai-profiles/${p.id}`),
                    },
                ],
                ...(canManage
                    ? [
                          [
                              {
                                  id: 'delete',
                                  label: t('ai.profiles.delete'),
                                  icon: Trash2,
                                  danger: true,
                                  run: () => void remove(p),
                              },
                          ],
                      ]
                    : []),
            ]}
        />
    );
}
