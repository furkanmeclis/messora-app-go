'use client';

import Link from 'next/link';
import { useActionState, useEffect, useState } from 'react';
import { FolderPlus, Pencil, UsersRound } from 'lucide-react';
import { FormStatus, TextField, fieldError } from '@/components/forms';
import { Button } from '@/components/ui/button';
import { Spinner } from '@/components/ui/spinner';
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogHeader,
    DialogTitle,
    DialogTrigger,
} from '@/components/ui/dialog';
import {
    Empty,
    EmptyDescription,
    EmptyHeader,
    EmptyMedia,
    EmptyTitle,
} from '@/components/ui/empty';
import { Field, FieldGroup, FieldLabel } from '@/components/ui/field';
import { Textarea } from '@/components/ui/textarea';
import { useFormatDateTime } from '@/components/display-preferences';
import { useT } from '@/i18n/client';
import type { ActionResult, ContactGroup } from '@/lib/types';
import { deleteGroupAction, saveGroupAction } from '../actions';
import { ConfirmDelete } from '../confirm-delete';
import { useSubmit } from '../use-submit';
import { NUMBER_LOCALE } from '@/lib/format';
import { useRouter } from 'next/navigation';
import { ClientDataTable } from '@/components/client-data-table';

export function GroupDialog({ group }: { group?: ContactGroup }) {
    const t = useT();
    const [open, setOpen] = useState(false);
    return (
        <Dialog open={open} onOpenChange={setOpen}>
            <DialogTrigger asChild>
                {group ? (
                    <Button
                        variant="ghost"
                        size="icon-sm"
                        aria-label={t('contacts.groups.editLabel', {
                            name: group.name,
                        })}
                    >
                        <Pencil />
                    </Button>
                ) : (
                    <Button shortcut="n">
                        <FolderPlus /> {t('contacts.groups.new')}
                    </Button>
                )}
            </DialogTrigger>
            <DialogContent>
                <DialogHeader>
                    <DialogTitle>
                        {group
                            ? t('contacts.groups.edit')
                            : t('contacts.groups.new')}
                    </DialogTitle>
                    <DialogDescription>
                        {t('contacts.groups.dialogHint')}
                    </DialogDescription>
                </DialogHeader>
                {open && (
                    <GroupForm group={group} onDone={() => setOpen(false)} />
                )}
            </DialogContent>
        </Dialog>
    );
}

function GroupForm({
    group,
    onDone,
}: {
    group?: ContactGroup;
    onDone: () => void;
}) {
    const t = useT();
    const [state, action, pending] = useActionState<
        ActionResult<ContactGroup> | null,
        FormData
    >(saveGroupAction.bind(null, group?.id ?? null), null);
    const submit = useSubmit(action);
    useEffect(() => {
        if (state?.ok) onDone();
    }, [state, onDone]);
    const descError = fieldError(state, 'description');
    return (
        <form onSubmit={submit} className="space-y-4">
            <FormStatus state={state?.ok ? null : state} />
            <FieldGroup>
                <TextField
                    label={t('contacts.fields.name')}
                    name="name"
                    required
                    autoFocus
                    maxLength={120}
                    defaultValue={group?.name}
                    error={fieldError(state, 'name')}
                />
                <Field data-invalid={descError ? true : undefined}>
                    <FieldLabel htmlFor="group-description">
                        {t('contacts.fields.description')}
                    </FieldLabel>
                    <Textarea
                        id="group-description"
                        name="description"
                        rows={2}
                        maxLength={500}
                        defaultValue={group?.description}
                        aria-invalid={descError ? true : undefined}
                    />
                    {descError && (
                        <p role="alert" className="text-sm text-destructive">
                            {descError}
                        </p>
                    )}
                </Field>
            </FieldGroup>
            <Button type="submit" className="w-full" disabled={pending}>
                {pending && <Spinner />}
                {group
                    ? t('contacts.groups.save')
                    : t('contacts.groups.create')}
            </Button>
        </form>
    );
}

export function GroupTable({
    groups,
    canManage,
}: {
    groups: ContactGroup[];
    canManage: boolean;
}) {
    const t = useT();
    const router = useRouter();
    const formatDateTime = useFormatDateTime();
    if (groups.length === 0) {
        return (
            <Empty className="py-12">
                <EmptyHeader>
                    <EmptyMedia variant="icon">
                        <UsersRound />
                    </EmptyMedia>
                    <EmptyTitle>{t('contacts.groups.empty')}</EmptyTitle>
                    <EmptyDescription>
                        {canManage
                            ? t('contacts.groups.emptyHintManage')
                            : t('contacts.groups.emptyHint')}
                    </EmptyDescription>
                </EmptyHeader>
            </Empty>
        );
    }
    return (
        <ClientDataTable
            rows={groups}
            rowId={(g) => g.id}
            storageKey="groups"
            initialSort="name"
            searchText={(g) => `${g.name} ${g.description}`}
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
                    sortValue: (g) => g.name,
                    cell: (g) => (
                        <div className="min-w-0">
                            <Link
                                href={`/contacts/groups/${g.id}`}
                                className="font-medium hover:underline"
                            >
                                {g.name}
                            </Link>
                            {g.description && (
                                <div className="max-w-md truncate text-xs text-muted-foreground">
                                    {g.description}
                                </div>
                            )}
                        </div>
                    ),
                },
                {
                    id: 'members',
                    header: t('contacts.groups.members'),
                    className: 'text-right',
                    sortValue: (g) => g.member_count,
                    cell: (g) => (
                        <Link
                            href={`/contacts?group_id=${g.id}`}
                            className="tabular-nums hover:underline"
                            aria-label={t('contacts.groups.showMembers', {
                                count: g.member_count,
                                name: g.name,
                            })}
                        >
                            {g.member_count.toLocaleString(NUMBER_LOCALE)}
                        </Link>
                    ),
                },
                {
                    id: 'created',
                    header: t('contacts.groups.created'),
                    sortValue: (g) => g.created_at,
                    cell: (g) => (
                        <span className="text-sm whitespace-nowrap text-muted-foreground">
                            {formatDateTime(g.created_at)}
                        </span>
                    ),
                },
            ]}
            rowActions={
                canManage
                    ? (g) => (
                          <span className="whitespace-nowrap">
                              <GroupDialog group={g} />
                              <ConfirmDelete
                                  label={t('contacts.groups.deleteLabel', {
                                      name: g.name,
                                  })}
                                  title={t('contacts.groups.deleteTitle', {
                                      name: g.name,
                                  })}
                                  description={t('contacts.groups.deleteBody')}
                                  confirm={t('contacts.groups.deleteConfirm')}
                                  action={() => deleteGroupAction(g.id)}
                              />
                          </span>
                      )
                    : undefined
            }
            rowMenu={(g) => [
                [
                    {
                        id: 'open',
                        label: g.name,
                        icon: UsersRound,
                        run: () => router.push(`/contacts/groups/${g.id}`),
                    },
                ],
            ]}
            onRowClick={(g) => router.push(`/contacts/groups/${g.id}`)}
        />
    );
}
