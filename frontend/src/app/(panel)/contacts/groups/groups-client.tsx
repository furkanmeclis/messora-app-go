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
import {
    Table,
    TableBody,
    TableCell,
    TableHead,
    TableHeader,
    TableRow,
} from '@/components/ui/table';
import { Textarea } from '@/components/ui/textarea';
import { useFormatDateTime } from '@/components/display-preferences';
import type { ActionResult, ContactGroup } from '@/lib/types';
import { deleteGroupAction, saveGroupAction } from '../actions';
import { ConfirmDelete } from '../confirm-delete';
import { useSubmit } from '../use-submit';

export function GroupDialog({ group }: { group?: ContactGroup }) {
    const [open, setOpen] = useState(false);
    return (
        <Dialog open={open} onOpenChange={setOpen}>
            <DialogTrigger asChild>
                {group ? (
                    <Button
                        variant="ghost"
                        size="icon-sm"
                        aria-label={`Edit group ${group.name}`}
                    >
                        <Pencil />
                    </Button>
                ) : (
                    <Button>
                        <FolderPlus /> New group
                    </Button>
                )}
            </DialogTrigger>
            <DialogContent>
                <DialogHeader>
                    <DialogTitle>
                        {group ? 'Edit group' : 'New group'}
                    </DialogTitle>
                    <DialogDescription>
                        Add contacts from the contact list or a contact’s page.
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
                    label="Name"
                    name="name"
                    required
                    autoFocus
                    maxLength={120}
                    defaultValue={group?.name}
                    error={fieldError(state, 'name')}
                />
                <Field data-invalid={descError ? true : undefined}>
                    <FieldLabel htmlFor="group-description">
                        Description
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
                {group ? 'Save group' : 'Create group'}
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
    const formatDateTime = useFormatDateTime();
    if (groups.length === 0) {
        return (
            <Empty className="py-12">
                <EmptyHeader>
                    <EmptyMedia variant="icon">
                        <UsersRound />
                    </EmptyMedia>
                    <EmptyTitle>No groups yet</EmptyTitle>
                    <EmptyDescription>
                        {canManage
                            ? 'Create a group, then add contacts to it from the contact list or while importing a file.'
                            : 'Groups your team creates appear here.'}
                    </EmptyDescription>
                </EmptyHeader>
            </Empty>
        );
    }
    return (
        <Table>
            <TableHeader>
                <TableRow>
                    <TableHead>Name</TableHead>
                    <TableHead className="text-right">Members</TableHead>
                    <TableHead>Created</TableHead>
                    {canManage && (
                        <TableHead>
                            <span className="sr-only">Actions</span>
                        </TableHead>
                    )}
                </TableRow>
            </TableHeader>
            <TableBody>
                {groups.map((g) => (
                    <TableRow key={g.id}>
                        <TableCell>
                            <div className="font-medium">{g.name}</div>
                            {g.description && (
                                <div className="max-w-md truncate text-xs text-muted-foreground">
                                    {g.description}
                                </div>
                            )}
                        </TableCell>
                        <TableCell className="text-right tabular-nums">
                            <Link
                                href={`/contacts?group_id=${g.id}`}
                                className="hover:underline"
                                aria-label={`${g.member_count} members of ${g.name}; show them`}
                            >
                                {g.member_count.toLocaleString()}
                            </Link>
                        </TableCell>
                        <TableCell className="text-sm text-muted-foreground">
                            {formatDateTime(g.created_at)}
                        </TableCell>
                        {canManage && (
                            <TableCell className="text-right whitespace-nowrap">
                                <GroupDialog group={g} />
                                <ConfirmDelete
                                    label={`Delete group ${g.name}`}
                                    title={`Delete ${g.name}?`}
                                    description="The group is removed. Its contacts stay, and campaigns already sent are not affected."
                                    confirm="Delete group"
                                    action={() => deleteGroupAction(g.id)}
                                />
                            </TableCell>
                        )}
                    </TableRow>
                ))}
            </TableBody>
        </Table>
    );
}
