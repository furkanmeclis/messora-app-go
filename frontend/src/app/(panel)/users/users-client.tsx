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
                    <TableHead>Name</TableHead>
                    <TableHead>Role</TableHead>
                    <TableHead>2FA</TableHead>
                    <TableHead>Last sign-in</TableHead>
                    <TableHead className="text-right">Status</TableHead>
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
                                            aria-label={`Role of ${u.name}`}
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
                                    <span>{u.role.name}</span>
                                )}
                            </TableCell>
                            <TableCell>
                                <Badge
                                    variant={
                                        u.mfa_enabled ? 'secondary' : 'outline'
                                    }
                                >
                                    {u.mfa_enabled ? 'On' : 'Off'}
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
                                            ? 'Disable'
                                            : 'Enable'}
                                    </Button>
                                ) : (
                                    <Badge
                                        variant={
                                            u.status === 'active'
                                                ? 'secondary'
                                                : 'destructive'
                                        }
                                    >
                                        {u.id === meId ? 'You' : u.status}
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
                <Button>
                    <UserPlus /> Invite user
                </Button>
            </DialogTrigger>
            <DialogContent>
                <DialogHeader>
                    <DialogTitle>Invite a user</DialogTitle>
                    <DialogDescription>
                        They receive a link to create their account.
                    </DialogDescription>
                </DialogHeader>
                {created && created.invitation.id && state?.ok ? (
                    <div className="space-y-3 text-sm">
                        <p>
                            {created.email_sent
                                ? `We e-mailed an invitation to ${created.invitation.email}. You can also share this link:`
                                : `E-mail is not configured. Share this link with ${created.invitation.email} — it is shown only once:`}
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
                                aria-label="Copy link"
                                onClick={() => {
                                    void navigator.clipboard.writeText(
                                        created.accept_url,
                                    );
                                    toast.success('Link copied');
                                }}
                            >
                                <Copy />
                            </Button>
                        </div>
                        <Button
                            className="w-full"
                            onClick={() => setOpen(false)}
                        >
                            Done
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
                                label="E-mail"
                                name="email"
                                type="email"
                                required
                                autoFocus
                                error={fieldError(state, 'email')}
                            />
                            <Field>
                                <FieldLabel htmlFor="field-role">
                                    Role
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
                            Send invitation
                        </SubmitButton>
                    </form>
                )}
            </DialogContent>
        </Dialog>
    );
}

export function InvitationList({ invitations }: { invitations: Invitation[] }) {
    const formatDateTime = useFormatDateTime();
    const [pending, startTransition] = useTransition();
    return (
        <ul className="divide-y rounded-md border">
            {invitations.map((i) => (
                <li key={i.id} className="flex items-center gap-3 p-3 text-sm">
                    <div className="min-w-0 flex-1">
                        <div className="font-medium">{i.email}</div>
                        <div className="text-muted-foreground">
                            {i.role.name} · invited by{' '}
                            {i.invited_by ?? 'unknown'} · expires{' '}
                            {formatDateTime(i.expires_at)}
                        </div>
                    </div>
                    {i.status === 'expired' && (
                        <Badge variant="outline">Expired</Badge>
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
                        Revoke
                    </Button>
                </li>
            ))}
        </ul>
    );
}
