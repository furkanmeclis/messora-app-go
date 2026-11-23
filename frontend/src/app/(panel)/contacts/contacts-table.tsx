'use client';

import { useState, useTransition } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { toast } from 'sonner';
import {
    BellOff,
    BellRing,
    Ellipsis,
    ExternalLink,
    FolderMinus,
    FolderPlus,
    Tag as TagIcon,
    TagsIcon,
    Trash2,
    UserX,
} from 'lucide-react';
import { useConfirm } from '@/components/confirm';
import {
    DataTable,
    selectionCount,
    type Column,
    type Selection,
} from '@/components/data-table';
import { useFormatDateTime } from '@/components/display-preferences';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Combobox } from '@/components/ui/combobox';
import {
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuSeparator,
    DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import {
    Empty,
    EmptyDescription,
    EmptyHeader,
    EmptyMedia,
    EmptyTitle,
} from '@/components/ui/empty';
import {
    Popover,
    PopoverContent,
    PopoverTrigger,
} from '@/components/ui/popover';
import { useT } from '@/i18n/client';
import type { Contact } from '@/lib/types';
import { bulkContactsAction, type BulkBody } from './actions';
import { contactName } from './shared';
import { TagChip } from './tag-chip';

type Option = { id: string; name: string };
type Filters = Record<
    'q' | 'tag_id' | 'group_id' | 'segment_id' | 'opted_out',
    string
>;

/** The selection as the bulk API expects it. */
function target(
    sel: Selection,
    filters: Filters,
): Pick<BulkBody, 'ids' | 'filter' | 'exclude_ids'> {
    if (sel.mode === 'ids') return { ids: sel.ids };
    const filter: NonNullable<BulkBody['filter']> = {};
    if (filters.q) filter.q = filters.q;
    if (filters.tag_id) filter.tag_id = filters.tag_id;
    if (filters.group_id) filter.group_id = filters.group_id;
    if (filters.segment_id) filter.segment_id = filters.segment_id;
    if (filters.opted_out) filter.opted_out = filters.opted_out === 'true';
    return {
        filter,
        ...(sel.exclude.length ? { exclude_ids: sel.exclude } : {}),
    };
}

export function ContactsTable({
    contacts,
    total,
    page,
    pageSize,
    sort,
    filters,
    tags,
    groups,
    canUpdate,
    canDelete,
    filtered,
}: {
    contacts: Contact[];
    total: number;
    page: number;
    pageSize: number;
    sort: string;
    filters: Filters;
    tags: Option[];
    groups: Option[];
    canUpdate: boolean;
    canDelete: boolean;
    filtered: boolean;
}) {
    const t = useT();
    const router = useRouter();
    const formatDateTime = useFormatDateTime();
    const unnamed = t('contacts.unnamed');

    const columns: Column<Contact>[] = [
        {
            id: 'name',
            header: t('contacts.fields.name'),
            hideable: false,
            sort: { asc: 'name_asc', desc: 'name_desc' },
            cell: (c) => (
                <div
                    className="flex min-w-0 items-center gap-3"
                    data-testid="contact-row-name"
                >
                    <span className="flex size-8 shrink-0 items-center justify-center rounded-full bg-primary/10 text-xs font-semibold text-primary">
                        {(/[\p{L}\p{N}]/u.exec(contactName(c, unnamed))?.[0] ?? '#').toLocaleUpperCase()}
                    </span>
                    <div className="min-w-0">
                        <Link
                            href={`/contacts/${c.id}`}
                            className="block truncate font-medium hover:underline"
                        >
                            {contactName(c, unnamed)}
                        </Link>
                        {c.email && (
                            <div className="truncate text-xs text-muted-foreground">
                                {c.email}
                            </div>
                        )}
                    </div>
                </div>
            ),
        },
        {
            id: 'phone',
            header: t('contacts.table.phone'),
            cell: (c) => <span className="font-mono text-sm">{c.phone}</span>,
        },
        {
            id: 'tags',
            header: t('contacts.fields.tags'),
            cell: (c) => (
                <div className="flex max-w-xs flex-wrap gap-1">
                    {c.tags.map((tag) => (
                        <TagChip
                            key={tag.id}
                            name={tag.name}
                            color={tag.color}
                        />
                    ))}
                </div>
            ),
        },
        {
            id: 'consent',
            header: t('contacts.consent.label'),
            cell: (c) =>
                c.opted_out ? (
                    <Badge variant="destructive" className="gap-1">
                        <BellOff className="size-3" />
                        {t('contacts.consent.optedOut')}
                    </Badge>
                ) : (
                    <Badge variant="secondary" className="gap-1">
                        <BellRing className="size-3" />
                        {t('contacts.consent.subscribed')}
                    </Badge>
                ),
        },
        {
            id: 'last_message',
            header: t('contacts.bulk.lastMessage'),
            sort: { asc: 'last_message', desc: 'last_message' },
            cell: (c) => (
                <span className="text-sm whitespace-nowrap text-muted-foreground">
                    {formatDateTime(c.last_inbound_at)}
                </span>
            ),
        },
        {
            id: 'source',
            header: t('contacts.bulk.source'),
            defaultHidden: true,
            cell: (c) => <Badge variant="outline">{c.source}</Badge>,
        },
        {
            id: 'added',
            header: t('contacts.table.added'),
            sort: { asc: 'oldest', desc: 'newest' },
            cell: (c) => (
                <span className="text-sm whitespace-nowrap text-muted-foreground">
                    {formatDateTime(c.created_at)}
                </span>
            ),
        },
    ];

    return (
        <DataTable
            storageKey="contacts"
            rowTestId="contact-row"
            rows={contacts}
            rowId={(c) => c.id}
            columns={columns}
            total={total}
            page={page}
            pageSize={pageSize}
            sort={sort}
            selectable={canUpdate || canDelete}
            onRowClick={(c) => router.push(`/contacts/${c.id}`)}
            rowActions={(c) => (
                <DropdownMenu>
                    <DropdownMenuTrigger asChild>
                        <Button
                            variant="ghost"
                            size="icon-sm"
                            aria-label={t('table.actions')}
                        >
                            <Ellipsis />
                        </Button>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent align="end">
                        <DropdownMenuItem asChild>
                            <Link href={`/contacts/${c.id}`}>
                                <ExternalLink />
                                {t('contacts.bulk.open')}
                            </Link>
                        </DropdownMenuItem>
                    </DropdownMenuContent>
                </DropdownMenu>
            )}
            empty={
                <Empty className="border-0 py-14">
                    <EmptyHeader>
                        <EmptyMedia variant="icon">
                            <UserX />
                        </EmptyMedia>
                        <EmptyTitle>
                            {filtered
                                ? t('contacts.table.noMatches')
                                : t('contacts.table.empty')}
                        </EmptyTitle>
                        <EmptyDescription>
                            {filtered
                                ? t('contacts.table.noMatchesHint')
                                : t('contacts.table.emptyHint')}
                        </EmptyDescription>
                    </EmptyHeader>
                </Empty>
            }
            bulkActions={(sel, clear) => (
                <BulkBar
                    selection={sel}
                    clear={clear}
                    filters={filters}
                    tags={tags}
                    groups={groups}
                    canUpdate={canUpdate}
                    canDelete={canDelete}
                />
            )}
        />
    );
}

function BulkBar({
    selection,
    clear,
    filters,
    tags,
    groups,
    canUpdate,
    canDelete,
}: {
    selection: Selection;
    clear: () => void;
    filters: Filters;
    tags: Option[];
    groups: Option[];
    canUpdate: boolean;
    canDelete: boolean;
}) {
    const t = useT();
    const router = useRouter();
    const ask = useConfirm();
    const [busy, start] = useTransition();
    const count = selectionCount(selection);

    const run = (
        body: Omit<BulkBody, 'ids' | 'filter' | 'exclude_ids'>,
        done: (affected: number) => string,
    ) =>
        start(async () => {
            const r = await bulkContactsAction({
                ...body,
                ...target(selection, filters),
            });
            if (!r.ok) {
                toast.error(r.message);
                return;
            }
            if (r.data.affected === 0) toast.info(t('table.bulkNothing'));
            else toast.success(done(r.data.affected));
            clear();
            router.refresh();
        });

    const nameOf = (list: Option[], id: string) =>
        list.find((o) => o.id === id)?.name ?? '';

    return (
        <div className="flex flex-wrap items-center gap-1">
            {canUpdate && (
                <>
                    <PickPopover
                        icon={<TagIcon />}
                        label={t('contacts.bulk.addTag')}
                        fieldLabel={t('contacts.bulk.tag')}
                        options={tags}
                        allowCreate
                        busy={busy}
                        onApply={(v, created) =>
                            run(
                                created
                                    ? { action: 'add_tag', tag_name: v }
                                    : { action: 'add_tag', tag_id: v },
                                (n) =>
                                    t('contacts.bulk.tagged', {
                                        count: n,
                                        tag: created ? v : nameOf(tags, v),
                                    }),
                            )
                        }
                    />
                    <PickPopover
                        icon={<FolderPlus />}
                        label={t('contacts.bulk.addToGroup')}
                        fieldLabel={t('contacts.bulk.group')}
                        options={groups}
                        busy={busy}
                        onApply={(v) =>
                            run({ action: 'add_to_group', group_id: v }, (n) =>
                                t('contacts.table.addedToGroup', {
                                    count: n,
                                    group: nameOf(groups, v),
                                }),
                            )
                        }
                    />
                </>
            )}
            <DropdownMenu>
                <DropdownMenuTrigger asChild>
                    <Button variant="ghost" size="sm" disabled={busy}>
                        <Ellipsis />
                        {t('contacts.bulk.more')}
                    </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end" side="top" className="w-56">
                    {canUpdate && (
                        <>
                            <RemoveMenuItem
                                icon={<TagsIcon />}
                                label={t('contacts.bulk.removeTag')}
                                fieldLabel={t('contacts.bulk.tag')}
                                options={tags}
                                onApply={(v) =>
                                    run(
                                        { action: 'remove_tag', tag_id: v },
                                        (n) =>
                                            t('contacts.bulk.untagged', {
                                                count: n,
                                                tag: nameOf(tags, v),
                                            }),
                                    )
                                }
                            />
                            <RemoveMenuItem
                                icon={<FolderMinus />}
                                label={t('contacts.bulk.removeFromGroup')}
                                fieldLabel={t('contacts.bulk.group')}
                                options={groups}
                                onApply={(v) =>
                                    run(
                                        {
                                            action: 'remove_from_group',
                                            group_id: v,
                                        },
                                        (n) =>
                                            t(
                                                'contacts.bulk.removedFromGroup',
                                                {
                                                    count: n,
                                                    group: nameOf(groups, v),
                                                },
                                            ),
                                    )
                                }
                            />
                            <DropdownMenuSeparator />
                            <DropdownMenuItem
                                onSelect={() =>
                                    run({ action: 'opt_in' }, (n) =>
                                        t('contacts.bulk.optedIn', {
                                            count: n,
                                        }),
                                    )
                                }
                            >
                                <BellRing />
                                {t('contacts.bulk.optIn')}
                            </DropdownMenuItem>
                            <DropdownMenuItem
                                onSelect={async () => {
                                    if (
                                        await ask({
                                            title: t(
                                                'contacts.bulk.optOutTitle',
                                                { count },
                                            ),
                                            description: t(
                                                'contacts.bulk.optOutBody',
                                            ),
                                            confirmLabel: t(
                                                'contacts.bulk.optOut',
                                            ),
                                        })
                                    )
                                        run({ action: 'opt_out' }, (n) =>
                                            t('contacts.bulk.optedOut', {
                                                count: n,
                                            }),
                                        );
                                }}
                            >
                                <BellOff />
                                {t('contacts.bulk.optOut')}
                            </DropdownMenuItem>
                        </>
                    )}
                    {canDelete && (
                        <>
                            <DropdownMenuSeparator />
                            <DropdownMenuItem
                                variant="destructive"
                                onSelect={async () => {
                                    if (
                                        await ask({
                                            destructive: true,
                                            title: t(
                                                'contacts.bulk.deleteTitle',
                                                { count },
                                            ),
                                            description: t(
                                                'contacts.bulk.deleteBody',
                                            ),
                                            confirmLabel: t(
                                                'contacts.bulk.delete',
                                            ),
                                        })
                                    )
                                        run({ action: 'delete' }, (n) =>
                                            t('contacts.bulk.deleted', {
                                                count: n,
                                            }),
                                        );
                                }}
                            >
                                <Trash2 />
                                {t('contacts.bulk.delete')}
                            </DropdownMenuItem>
                        </>
                    )}
                </DropdownMenuContent>
            </DropdownMenu>
        </div>
    );
}

/** A bar button that opens a picker (tag or group) and applies it. */
function PickPopover({
    icon,
    label,
    fieldLabel,
    options,
    allowCreate,
    busy,
    onApply,
}: {
    icon: React.ReactNode;
    label: string;
    fieldLabel: string;
    options: Option[];
    allowCreate?: boolean;
    busy: boolean;
    onApply: (value: string, created: boolean) => void;
}) {
    const t = useT();
    const [open, setOpen] = useState(false);
    const [value, setValue] = useState('');
    const [typed, setTyped] = useState('');
    const created = allowCreate && !value && typed.trim().length > 0;
    return (
        <Popover open={open} onOpenChange={setOpen}>
            <PopoverTrigger asChild>
                <Button variant="ghost" size="sm" disabled={busy}>
                    {icon}
                    {label}
                </Button>
            </PopoverTrigger>
            <PopoverContent side="top" align="start" className="w-72 space-y-3">
                <p className="text-sm font-medium">{label}</p>
                <Combobox
                    aria-label={fieldLabel}
                    options={options.map((o) => ({
                        value: o.id,
                        label: o.name,
                    }))}
                    value={value}
                    onValueChange={(v) => {
                        setValue(v);
                        setTyped('');
                    }}
                    placeholder={
                        allowCreate
                            ? t('contacts.bulk.tagPlaceholder')
                            : fieldLabel
                    }
                    searchPlaceholder={t('common.searchPlaceholder')}
                    emptyText={t('common.noResults')}
                />
                {allowCreate && (
                    <input
                        value={typed}
                        onChange={(e) => {
                            setTyped(e.target.value);
                            setValue('');
                        }}
                        maxLength={50}
                        placeholder={t('contacts.bulk.createTag', {
                            name: '…',
                        })}
                        className="h-8 w-full rounded-lg border border-input bg-transparent px-2.5 text-sm outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50"
                    />
                )}
                <Button
                    className="w-full"
                    size="sm"
                    disabled={busy || (!value && !created)}
                    onClick={() => {
                        onApply(created ? typed.trim() : value, !!created);
                        setOpen(false);
                        setValue('');
                        setTyped('');
                    }}
                >
                    {created
                        ? t('contacts.bulk.createTag', { name: typed.trim() })
                        : t('contacts.bulk.apply')}
                </Button>
            </PopoverContent>
        </Popover>
    );
}

/** A menu entry that asks which tag or group to remove. */
function RemoveMenuItem({
    icon,
    label,
    fieldLabel,
    options,
    onApply,
}: {
    icon: React.ReactNode;
    label: string;
    fieldLabel: string;
    options: Option[];
    onApply: (value: string) => void;
}) {
    const t = useT();
    const [value, setValue] = useState('');
    return (
        <div className="space-y-2 px-2 py-1.5">
            <p className="flex items-center gap-2 text-sm [&_svg]:size-4 [&_svg]:text-muted-foreground">
                {icon}
                {label}
            </p>
            <div className="flex gap-1.5">
                <Combobox
                    aria-label={fieldLabel}
                    options={options.map((o) => ({
                        value: o.id,
                        label: o.name,
                    }))}
                    value={value}
                    onValueChange={setValue}
                    placeholder={fieldLabel}
                    searchPlaceholder={t('common.searchPlaceholder')}
                    emptyText={t('common.noResults')}
                    className="h-7"
                />
                <Button
                    size="sm"
                    variant="secondary"
                    disabled={!value}
                    onClick={() => onApply(value)}
                >
                    {t('contacts.bulk.apply')}
                </Button>
            </div>
        </div>
    );
}
