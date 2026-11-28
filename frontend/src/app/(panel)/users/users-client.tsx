'use client';

import { useActionState, useState, useTransition } from 'react';
import { Ban, Copy, Mail, UserCheck, UserPlus, UserX } from 'lucide-react';
import { toast } from 'sonner';
import { ClientDataTable } from '@/components/client-data-table';
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
import { Spinner } from '@/components/ui/spinner';
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

function copyText(text: string, done: string) {
    void navigator.clipboard.writeText(text).then(
        () => toast.success(done),
        () => undefined,
    );
}

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
    const editable = (u: User) =>
        u.id !== meId && (meIsOwner || (RANK[u.role.key] ?? 0) > myRank);
    const toggle = (u: User) =>
        update(u.id, {
            status: u.status === 'active' ? 'disabled' : 'active',
        });

    return (
        <ClientDataTable
            rows={users}
            rowId={(u) => u.id}
            storageKey="users"
            initialSort="name"
            searchText={(u) => `${u.name} ${u.email}`}
            searchPlaceholder={t('table.search')}
            noMatch={
                <p className="p-8 text-center text-sm text-muted-foreground">
                    {t('table.noMatch')}
                </p>
            }
            columns={[
                {
                    id: 'name',
                    header: t('users.columns.name'),
                    hideable: false,
                    sortValue: (u) => u.name,
                    cell: (u) => (
                        <div className="min-w-0">
                            <div className="flex items-center gap-2 font-medium">
                                {u.name}
                                {u.id === meId && (
                                    <Badge variant="outline">
                                        {t('users.you')}
                                    </Badge>
                                )}
                            </div>
                            <div className="text-xs text-muted-foreground">
                                {u.email}
                            </div>
                        </div>
                    ),
                },
                {
                    id: 'role',
                    header: t('users.columns.role'),
                    sortValue: (u) => RANK[u.role.key] ?? 99,
                    cell: (u) =>
                        editable(u) ? (
                            <Select
                                defaultValue={u.role.key}
                                disabled={pending}
                                onValueChange={(role) => update(u.id, { role })}
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
                                        <SelectItem key={r.key} value={r.key}>
                                            {r.name}
                                        </SelectItem>
                                    ))}
                                </SelectContent>
                            </Select>
                        ) : (
                            <span>
                                {t(`settings.security.roles.${u.role.key}`)}
                            </span>
                        ),
                },
                {
                    id: 'twoFactor',
                    header: t('users.columns.twoFactor'),
                    sortValue: (u) => (u.mfa_enabled ? 1 : 0),
                    cell: (u) => (
                        <Badge
                            variant={u.mfa_enabled ? 'secondary' : 'outline'}
                        >
                            {u.mfa_enabled ? t('users.on') : t('users.off')}
                        </Badge>
                    ),
                },
                {
                    id: 'lastSignIn',
                    header: t('users.columns.lastSignIn'),
                    sortValue: (u) => u.last_login_at,
                    cell: (u) => (
                        <span className="text-sm whitespace-nowrap text-muted-foreground">
                            {formatDateTime(u.last_login_at)}
                        </span>
                    ),
                },
                {
                    id: 'status',
                    header: t('users.columns.status'),
                    sortValue: (u) => u.status,
                    cell: (u) => (
                        <Badge
                            variant={
                                u.status === 'active'
                                    ? 'secondary'
                                    : 'destructive'
                            }
                        >
                            {t(`users.statuses.${u.status}`)}
                        </Badge>
                    ),
                },
            ]}
            rowActions={(u) =>
                editable(u) ? (
                    <Button
                        size="sm"
                        variant="outline"
                        disabled={pending}
                        onClick={() => toggle(u)}
                    >
                        {u.status === 'active'
                            ? t('users.disable')
                            : t('users.enable')}
                    </Button>
                ) : null
            }
            rowMenu={(u) => [
                [
                    {
                        id: 'email',
                        label: t('contextMenu.copyEmail'),
                        icon: Mail,
                        run: () => copyText(u.email, t('contextMenu.copied')),
                    },
                    ...(editable(u)
                        ? [
                              {
                                  id: 'status',
                                  label:
                                      u.status === 'active'
                                          ? t('users.disable')
                                          : t('users.enable'),
                                  icon:
                                      u.status === 'active' ? UserX : UserCheck,
                                  danger: u.status === 'active',
                                  disabled: pending,
                                  run: () => toggle(u),
                              },
                          ]
                        : []),
                ],
            ]}
        />
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
    const revoke = (ids: string[], done?: () => void) =>
        startTransition(async () => {
            if (ids.length === 1) {
                if (notify(await revokeInvitationAction(ids[0]!))) done?.();
                return;
            }
            let revoked = 0;
            for (const id of ids) {
                const r = await revokeInvitationAction(id);
                if (!r.ok) {
                    void notify(r);
                    break;
                }
                revoked++;
            }
            if (revoked > 0) {
                toast.success(
                    t('users.invitation.revokedMany', { count: revoked }),
                );
                done?.();
            }
        });
    return (
        <ClientDataTable
            rows={invitations}
            rowId={(i) => i.id}
            storageKey="invitations"
            initialSort="expires"
            searchText={(i) => `${i.email} ${i.invited_by ?? ''}`}
            searchPlaceholder={t('table.search')}
            noMatch={
                <p className="p-8 text-center text-sm text-muted-foreground">
                    {t('table.noMatch')}
                </p>
            }
            selectable
            bulkActions={(ids, clear) => (
                <Button
                    size="sm"
                    variant="ghost"
                    disabled={pending}
                    onClick={() => revoke(ids, clear)}
                >
                    {pending ? <Spinner /> : <Ban />}
                    {t('users.invitation.revoke')}
                </Button>
            )}
            columns={[
                {
                    id: 'email',
                    header: t('users.invite.email'),
                    hideable: false,
                    sortValue: (i) => i.email,
                    cell: (i) => <span className="font-medium">{i.email}</span>,
                },
                {
                    id: 'role',
                    header: t('users.columns.role'),
                    sortValue: (i) => RANK[i.role.key] ?? 99,
                    cell: (i) => t(`settings.security.roles.${i.role.key}`),
                },
                {
                    id: 'invitedBy',
                    header: t('users.invitation.invitedBy'),
                    sortValue: (i) => i.invited_by,
                    cell: (i) => (
                        <span className="text-sm text-muted-foreground">
                            {i.invited_by ?? t('users.invitation.unknown')}
                        </span>
                    ),
                },
                {
                    id: 'expires',
                    header: t('users.invitation.expires'),
                    sortValue: (i) => i.expires_at,
                    cell: (i) => (
                        <span className="flex items-center gap-2 text-sm whitespace-nowrap text-muted-foreground">
                            {formatDateTime(i.expires_at)}
                            {i.status === 'expired' && (
                                <Badge variant="outline">
                                    {t('users.invitation.expired')}
                                </Badge>
                            )}
                        </span>
                    ),
                },
            ]}
            rowActions={(i) => (
                <Button
                    size="sm"
                    variant="outline"
                    disabled={pending}
                    onClick={() => revoke([i.id])}
                >
                    {t('users.invitation.revoke')}
                </Button>
            )}
            rowMenu={(i) => [
                [
                    {
                        id: 'email',
                        label: t('contextMenu.copyEmail'),
                        icon: Mail,
                        run: () => copyText(i.email, t('contextMenu.copied')),
                    },
                ],
                [
                    {
                        id: 'revoke',
                        label: t('users.invitation.revoke'),
                        icon: Ban,
                        danger: true,
                        disabled: pending,
                        run: () => revoke([i.id]),
                    },
                ],
            ]}
        />
    );
}
