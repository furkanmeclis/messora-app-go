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
import { Segmented } from '@/components/ui/segmented';
import { useT } from '@/i18n/client';
import type { MessageKey } from '@/i18n/translate';
import type { ActionResult, Tag } from '@/lib/types';
import { deleteTagAction, saveTagAction } from '../actions';
import { ConfirmDelete } from '../confirm-delete';
import { TAG_COLORS, TagChip } from '../tag-chip';
import { useSubmit } from '../use-submit';
import { NUMBER_LOCALE } from '@/lib/format';

export function TagDialog({ tag }: { tag?: Tag }) {
    const t = useT();
    const [open, setOpen] = useState(false);
    return (
        <Dialog open={open} onOpenChange={setOpen}>
            <DialogTrigger asChild>
                {tag ? (
                    <Button
                        variant="ghost"
                        size="icon-sm"
                        aria-label={t('contacts.tags.editLabel', {
                            name: tag.name,
                        })}
                    >
                        <Pencil />
                    </Button>
                ) : (
                    <Button>
                        <Plus /> {t('contacts.tags.new')}
                    </Button>
                )}
            </DialogTrigger>
            <DialogContent>
                <DialogHeader>
                    <DialogTitle>
                        {tag ? t('contacts.tags.edit') : t('contacts.tags.new')}
                    </DialogTitle>
                    <DialogDescription>
                        {tag
                            ? t('contacts.tags.renameHint')
                            : t('contacts.tags.createHint')}
                    </DialogDescription>
                </DialogHeader>
                {open && <TagForm tag={tag} onDone={() => setOpen(false)} />}
            </DialogContent>
        </Dialog>
    );
}

function TagForm({ tag, onDone }: { tag?: Tag; onDone: () => void }) {
    const t = useT();
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
                    label={t('contacts.fields.name')}
                    name="name"
                    required
                    autoFocus
                    maxLength={60}
                    defaultValue={tag?.name}
                    error={fieldError(state, 'name')}
                />
                <FieldSet>
                    <FieldLegend variant="label">
                        {t('contacts.tags.colour')}
                    </FieldLegend>
                    <Segmented
                        name="color"
                        aria-label={t('contacts.tags.colour')}
                        value={color}
                        onValueChange={setColor}
                        options={Object.entries(TAG_COLORS).map(
                            ([name, cls]) => ({
                                value: name,
                                label: t(
                                    `contacts.tags.colors.${name}` as MessageKey,
                                ),
                                icon: (
                                    <span
                                        aria-hidden
                                        className={cn(
                                            'size-3 rounded-full',
                                            cls,
                                        )}
                                    />
                                ),
                            }),
                        )}
                    />
                    {colorError && (
                        <p role="alert" className="text-sm text-destructive">
                            {colorError}
                        </p>
                    )}
                </FieldSet>
            </FieldGroup>
            <Button type="submit" className="w-full" disabled={pending}>
                {pending && <Spinner />}
                {tag ? t('contacts.tags.save') : t('contacts.tags.create')}
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
    const t = useT();
    if (tags.length === 0) {
        return (
            <Empty className="py-12">
                <EmptyHeader>
                    <EmptyMedia variant="icon">
                        <Tags />
                    </EmptyMedia>
                    <EmptyTitle>{t('contacts.tags.empty')}</EmptyTitle>
                    <EmptyDescription>
                        {t('contacts.tags.emptyHint')}
                    </EmptyDescription>
                </EmptyHeader>
            </Empty>
        );
    }
    return (
        <Table>
            <TableHeader>
                <TableRow>
                    <TableHead>{t('contacts.filters.tag')}</TableHead>
                    <TableHead className="text-right">
                        {t('contacts.title')}
                    </TableHead>
                    {canManage && (
                        <TableHead>
                            <span className="sr-only">
                                {t('contacts.actions')}
                            </span>
                        </TableHead>
                    )}
                </TableRow>
            </TableHeader>
            <TableBody>
                {tags.map((tag) => (
                    <TableRow key={tag.id}>
                        <TableCell>
                            <TagChip name={tag.name} color={tag.color} />
                        </TableCell>
                        <TableCell className="text-right tabular-nums">
                            <Link
                                href={`/contacts?tag_id=${tag.id}`}
                                className="hover:underline"
                                aria-label={t('contacts.tags.showContacts', {
                                    count: tag.contact_count,
                                    name: tag.name,
                                })}
                            >
                                {tag.contact_count.toLocaleString(
                                    NUMBER_LOCALE,
                                )}
                            </Link>
                        </TableCell>
                        {canManage && (
                            <TableCell className="text-right whitespace-nowrap">
                                <TagDialog tag={tag} />
                                <ConfirmDelete
                                    label={t('contacts.tags.deleteLabel', {
                                        name: tag.name,
                                    })}
                                    title={t('contacts.tags.deleteTitle', {
                                        name: tag.name,
                                    })}
                                    description={t('contacts.tags.deleteBody', {
                                        count: tag.contact_count,
                                        formatted:
                                            tag.contact_count.toLocaleString(
                                                NUMBER_LOCALE,
                                            ),
                                    })}
                                    confirm={t('contacts.tags.deleteConfirm')}
                                    action={() => deleteTagAction(tag.id)}
                                />
                            </TableCell>
                        )}
                    </TableRow>
                ))}
            </TableBody>
        </Table>
    );
}
