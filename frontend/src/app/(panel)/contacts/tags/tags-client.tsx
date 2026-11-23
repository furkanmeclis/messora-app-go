'use client';

import Link from 'next/link';
import { useActionState, useEffect, useState } from 'react';
import { Pencil, Plus, Tags } from 'lucide-react';
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
import { FieldGroup, FieldLegend, FieldSet } from '@/components/ui/field';
import {
    Table,
    TableBody,
    TableCell,
    TableHead,
    TableHeader,
    TableRow,
} from '@/components/ui/table';
import { cn } from '@/lib/utils';
import type { ActionResult, Tag } from '@/lib/types';
import { deleteTagAction, saveTagAction } from '../actions';
import { ConfirmDelete } from '../confirm-delete';
import { TAG_COLORS, TagChip } from '../tag-chip';
import { useSubmit } from '../use-submit';
import { NUMBER_LOCALE } from '@/lib/format';

export function TagDialog({ tag }: { tag?: Tag }) {
    const [open, setOpen] = useState(false);
    return (
        <Dialog open={open} onOpenChange={setOpen}>
            <DialogTrigger asChild>
                {tag ? (
                    <Button
                        variant="ghost"
                        size="icon-sm"
                        aria-label={`Edit tag ${tag.name}`}
                    >
                        <Pencil />
                    </Button>
                ) : (
                    <Button>
                        <Plus /> New tag
                    </Button>
                )}
            </DialogTrigger>
            <DialogContent>
                <DialogHeader>
                    <DialogTitle>{tag ? 'Edit tag' : 'New tag'}</DialogTitle>
                    <DialogDescription>
                        {tag
                            ? 'Renaming changes the tag on every contact that has it.'
                            : 'Tags can also be created by typing a new name on a contact.'}
                    </DialogDescription>
                </DialogHeader>
                {open && <TagForm tag={tag} onDone={() => setOpen(false)} />}
            </DialogContent>
        </Dialog>
    );
}

function TagForm({ tag, onDone }: { tag?: Tag; onDone: () => void }) {
    const [state, action, pending] = useActionState<
        ActionResult<Tag> | null,
        FormData
    >(saveTagAction.bind(null, tag?.id ?? null), null);
    const [color, setColor] = useState(
        tag && tag.color in TAG_COLORS ? tag.color : 'gray',
    );
    const submit = useSubmit(action);
    useEffect(() => {
        if (state?.ok) onDone();
    }, [state, onDone]);
    const colorError = fieldError(state, 'color');
    return (
        <form onSubmit={submit} className="space-y-4">
            <FormStatus state={state?.ok ? null : state} />
            <FieldGroup>
                <TextField
                    label="Name"
                    name="name"
                    required
                    autoFocus
                    maxLength={60}
                    defaultValue={tag?.name}
                    error={fieldError(state, 'name')}
                />
                <FieldSet>
                    <FieldLegend variant="label">Colour</FieldLegend>
                    <div
                        role="radiogroup"
                        aria-label="Colour"
                        className="flex flex-wrap gap-2"
                    >
                        {Object.entries(TAG_COLORS).map(([name, cls]) => (
                            <label
                                key={name}
                                className={cn(
                                    'flex cursor-pointer items-center gap-1.5 rounded-md border px-2 py-1 text-xs capitalize has-focus-visible:ring-2 has-focus-visible:ring-ring',
                                    color === name &&
                                        'border-primary bg-muted font-medium',
                                )}
                            >
                                <input
                                    type="radio"
                                    name="color"
                                    value={name}
                                    checked={color === name}
                                    onChange={() => setColor(name)}
                                    className="sr-only"
                                />
                                <span
                                    aria-hidden
                                    className={cn('size-3 rounded-full', cls)}
                                />
                                {name}
                            </label>
                        ))}
                    </div>
                    {colorError && (
                        <p role="alert" className="text-sm text-destructive">
                            {colorError}
                        </p>
                    )}
                </FieldSet>
            </FieldGroup>
            <Button type="submit" className="w-full" disabled={pending}>
                {pending && <Spinner />}
                {tag ? 'Save tag' : 'Create tag'}
            </Button>
        </form>
    );
}

export function TagTable({
    tags,
    canManage,
}: {
    tags: Tag[];
    canManage: boolean;
}) {
    if (tags.length === 0) {
        return (
            <Empty className="py-12">
                <EmptyHeader>
                    <EmptyMedia variant="icon">
                        <Tags />
                    </EmptyMedia>
                    <EmptyTitle>No tags yet</EmptyTitle>
                    <EmptyDescription>
                        Tags appear here once you add them to contacts, import a
                        file with tags or create one.
                    </EmptyDescription>
                </EmptyHeader>
            </Empty>
        );
    }
    return (
        <Table>
            <TableHeader>
                <TableRow>
                    <TableHead>Tag</TableHead>
                    <TableHead className="text-right">Contacts</TableHead>
                    {canManage && (
                        <TableHead>
                            <span className="sr-only">Actions</span>
                        </TableHead>
                    )}
                </TableRow>
            </TableHeader>
            <TableBody>
                {tags.map((t) => (
                    <TableRow key={t.id}>
                        <TableCell>
                            <TagChip name={t.name} color={t.color} />
                        </TableCell>
                        <TableCell className="text-right tabular-nums">
                            <Link
                                href={`/contacts?tag_id=${t.id}`}
                                className="hover:underline"
                                aria-label={`${t.contact_count} contacts tagged ${t.name}; show them`}
                            >
                                {t.contact_count.toLocaleString(NUMBER_LOCALE)}
                            </Link>
                        </TableCell>
                        {canManage && (
                            <TableCell className="text-right whitespace-nowrap">
                                <TagDialog tag={t} />
                                <ConfirmDelete
                                    label={`Delete tag ${t.name}`}
                                    title={`Delete the tag ${t.name}?`}
                                    description={`It is removed from ${t.contact_count.toLocaleString(NUMBER_LOCALE)} contact${t.contact_count === 1 ? '' : 's'}. The contacts themselves stay.`}
                                    confirm="Delete tag"
                                    action={() => deleteTagAction(t.id)}
                                />
                            </TableCell>
                        )}
                    </TableRow>
                ))}
            </TableBody>
        </Table>
    );
}
