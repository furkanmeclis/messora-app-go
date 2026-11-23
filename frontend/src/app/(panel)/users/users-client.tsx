'use client';

import { useActionState, useState, useTransition } from 'react';
import { Copy, UserPlus } from 'lucide-react';
import { toast } from 'sonner';
import {
    FormStatus,
    SubmitButton,
    TextField,
    fieldError,
} from '@/components/forms';
import { newIdempotencyKey, notify } from '@/components/notify';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogHeader,
    DialogTitle,
    DialogTrigger,
} from '@/components/ui/dialog';
import { Field, FieldGroup, FieldLabel } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from '@/components/ui/select';
import {
    Table,
    TableBody,
    TableCell,
    TableHead,
    TableHeader,
    TableRow,
} from '@/components/ui/table';
import { useFormatDateTime } from '@/components/display-preferences';
import { useT } from '@/i18n/client';
import type {
    ActionResult,
    Invitation,
    RoleKey,
    Schemas,
    User,
} from '@/lib/types';
import {
    inviteAction,
    revokeInvitationAction,
    updateUserAction,
} from '../actions';

type RoleOption = { key: RoleKey; name: string };
const RANK: Record<string, number> = {
    owner: 0,
    admin: 10,
    manager: 20,
    agent: 30,
};

export function UserTable({
    users,
    meId,
    meIsOwner,
    myRank,
    roles,
}: {
    users: User[];
    meId: string;
    meIsOwner: boolean;
    myRank: number;
    roles: RoleOption[];
}) {
    const t = useT();
    const formatDateTime = useFormatDateTime();
    const [pending, startTransition] = useTransition();
    const update = (
        id: string,
        patch: Parameters<typeof updateUserAction>[1],
    ) =>
        startTransition(
            async () => void notify(await updateUserAction(id, patch)),
        );

    return (
        <Table>
            <TableHeader>
                <TableRow>
                    <TableHead>{t('users.columns.name')}</TableHead>
                    <TableHead>{t('users.columns.role')}</TableHead>
                    <TableHead>{t('users.columns.twoFactor')}</TableHead>
                    <TableHead>{t('users.columns.lastSignIn')}</TableHead>
                    <TableHead className="text-right">
                        {t('users.columns.status')}
                    </TableHead>
                </TableRow>
            </TableHeader>
            <TableBody>
                {users.map((u) => {
                    const editable =
                        u.id !== meId &&
                        (meIsOwner || (RANK[u.role.key] ?? 0) > myRank);
                    return (
                        <TableRow key={u.id}>
                            <TableCell>
                                <div className="font-medium">{u.name}</div>
                                <div className="text-xs text-muted-foreground">
                                    {u.email}
                                </div>
                            </TableCell>
                            <TableCell>
                                {editable ? (
                                    <Select
                                        defaultValue={u.role.key}
                                        disabled={pending}
                                        onValueChange={(role) =>
                                            update(u.id, { role })
                                        }
                                    >
                                        <SelectTrigger
                                            className="w-32"
                                            aria-label={t('users.roleOf', {
                                                name: u.name,
                                            })}
                                        >
                                            <SelectValue />
                                        </SelectTrigger>
                                        <SelectContent>
                                            {roles.map((r) => (
                                                <SelectItem
                                                    key={r.key}
                                                    value={r.key}
                                                >
                                                    {r.name}
                                                </SelectItem>
                                            ))}
                                        </SelectContent>
                                    </Select>
                                ) : (
                                    <span>
                                        {t(
                                            `settings.security.roles.${u.role.key}`,
                                        )}
                                    </span>
                                )}
                            </TableCell>
                            <TableCell>
                                <Badge
                                    variant={
                                        u.mfa_enabled ? 'secondary' : 'outline'
                                    }
                                >
                                    {u.mfa_enabled
                                        ? t('users.on')
                                        : t('users.off')}
                                </Badge>
                            </TableCell>
                            <TableCell className="text-sm text-muted-foreground">
                                {formatDateTime(u.last_login_at)}
                            </TableCell>
                            <TableCell className="text-right">
                                {editable ? (
                                    <Button
                                        size="sm"
                                        variant="outline"
                                        disabled={pending}
                                        onClick={() =>
                                            update(u.id, {
                                                status:
                                                    u.status === 'active'
                                                        ? 'disabled'
                                                        : 'active',
                                            })
                                        }
                                    >
                                        {u.status === 'active'
                                            ? t('users.disable')
                                            : t('users.enable')}
                                    </Button>
                                ) : (
                                    <Badge
                                        variant={
                                            u.status === 'active'
                                                ? 'secondary'
                                                : 'destructive'
                                        }
                                    >
                                        {u.id === meId
                                            ? t('users.you')
                                            : t(`users.statuses.${u.status}`)}
                                    </Badge>
                                )}
                            </TableCell>
                        </TableRow>
                    );
                })}
            </TableBody>
        </Table>
    );
}

export function InviteDialog({ roles }: { roles: RoleOption[] }) {
    const t = useT();
    const [open, setOpen] = useState(false);
    const [key, setKey] = useState(newIdempotencyKey);
    const [state, action] = useActionState<
        ActionResult<Schemas['InvitationCreated']> | null,
        FormData
    >(inviteAction, null);
    const created = state?.ok ? state.data : null;

    return (
        <Dialog
            open={open}
            onOpenChange={(o) => {
                setOpen(o);
                if (!o) setKey(newIdempotencyKey());
            }}
        >
            <DialogTrigger asChild>
                <Button shortcut="n">
                    <UserPlus /> {t('users.invite.button')}
                </Button>
            </DialogTrigger>
            <DialogContent>
                <DialogHeader>
                    <DialogTitle>{t('users.invite.title')}</DialogTitle>
                    <DialogDescription>
                        {t('users.invite.description')}
                    </DialogDescription>
                </DialogHeader>
                {created && created.invitation.id && state?.ok ? (
                    <div className="space-y-3 text-sm">
                        <p>
                            {created.email_sent
                                ? t('users.invite.emailed', {
                                      email: created.invitation.email,
                                  })
                                : t('users.invite.noEmail', {
                                      email: created.invitation.email,
                                  })}
                        </p>
                        <div className="flex gap-2">
                            <Input
                                readOnly
                                value={created.accept_url}
                                className="font-mono text-xs"
                            />
                            <Button
                                type="button"
                                variant="outline"
                                size="icon"
                                aria-label={t('users.invite.copyLink')}
                                onClick={() => {
                                    void navigator.clipboard.writeText(
                                        created.accept_url,
                                    );
                                    toast.success(t('users.invite.linkCopied'));
                                }}
                            >
                                <Copy />
                            </Button>
                        </div>
                        <Button
                            className="w-full"
                            onClick={() => setOpen(false)}
                        >
                            {t('users.invite.done')}
                        </Button>
                    </div>
                ) : (
                    <form action={action} className="space-y-4">
                        <input
                            type="hidden"
                            name="idempotency_key"
                            value={key}
                        />
                        <FormStatus state={state} />
                        <FieldGroup>
                            <TextField
                                label={t('users.invite.email')}
                                name="email"
                                type="email"
                                required
                                autoFocus
                                error={fieldError(state, 'email')}
                            />
                            <Field>
                                <FieldLabel htmlFor="field-role">
                                    {t('users.invite.role')}
                                </FieldLabel>
                                <Select
                                    name="role"
                                    defaultValue={roles.at(-1)?.key}
                                >
                                    <SelectTrigger
                                        id="field-role"
                                        className="w-full"
                                    >
                                        <SelectValue />
                                    </SelectTrigger>
                                    <SelectContent>
                                        {roles.map((r) => (
                                            <SelectItem
                                                key={r.key}
                                                value={r.key}
                                            >
                                                {r.name}
                                            </SelectItem>
                                        ))}
                                    </SelectContent>
                                </Select>
                            </Field>
                        </FieldGroup>
                        <SubmitButton className="w-full">
                            {t('users.invite.send')}
                        </SubmitButton>
                    </form>
                )}
            </DialogContent>
        </Dialog>
    );
}

export function InvitationList({ invitations }: { invitations: Invitation[] }) {
    const t = useT();
    const formatDateTime = useFormatDateTime();
    const [pending, startTransition] = useTransition();
    return (
        <ul className="divide-y rounded-md border">
            {invitations.map((i) => (
                <li key={i.id} className="flex items-center gap-3 p-3 text-sm">
                    <div className="min-w-0 flex-1">
                        <div className="font-medium">{i.email}</div>
                        <div className="text-muted-foreground">
                            {t('users.invitation.details', {
                                role: t(
                                    `settings.security.roles.${i.role.key}`,
                                ),
                                by:
                                    i.invited_by ??
                                    t('users.invitation.unknown'),
                                expires: formatDateTime(i.expires_at),
                            })}
                        </div>
                    </div>
                    {i.status === 'expired' && (
                        <Badge variant="outline">
                            {t('users.invitation.expired')}
                        </Badge>
                    )}
                    <Button
                        size="sm"
                        variant="outline"
                        disabled={pending}
                        onClick={() =>
                            startTransition(
                                async () =>
                                    void notify(
                                        await revokeInvitationAction(i.id),
                                    ),
                            )
                        }
                    >
                        {t('users.invitation.revoke')}
                    </Button>
                </li>
            ))}
        </ul>
    );
}
