'use client';

import { useState, useTransition } from 'react';
import { Copy, KeyRound, Plus } from 'lucide-react';
import { toast } from 'sonner';
import { TextField } from '@/components/forms';
import { newIdempotencyKey, notify } from '@/components/notify';
import { useReauth } from '@/components/reauth';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
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
import { FieldGroup } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Spinner } from '@/components/ui/spinner';
import {
    Table,
    TableBody,
    TableCell,
    TableHead,
    TableHeader,
    TableRow,
} from '@/components/ui/table';
import { useFormatDateTime } from '@/components/display-preferences';
import type { ActionResult, ApiKey, Schemas } from '@/lib/types';
import { createApiKeyAction, revokeApiKeyAction } from '../actions';

export function ApiKeyTable({ keys }: { keys: ApiKey[] }) {
    const formatDateTime = useFormatDateTime();
    const [pending, startTransition] = useTransition();
    if (keys.length === 0) {
        return (
            <Empty className="py-12">
                <EmptyHeader>
                    <EmptyMedia variant="icon">
                        <KeyRound />
                    </EmptyMedia>
                    <EmptyTitle>No API keys yet</EmptyTitle>
                    <EmptyDescription>
                        Create a key to connect another system to Messora.
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
                    <TableHead>Scopes</TableHead>
                    <TableHead>Last used</TableHead>
                    <TableHead>Expires</TableHead>
                    <TableHead />
                </TableRow>
            </TableHeader>
            <TableBody>
                {keys.map((k) => (
                    <TableRow key={k.id}>
                        <TableCell>
                            <div className="font-medium">{k.name}</div>
                            <div className="font-mono text-xs text-muted-foreground">
                                {k.prefix}…
                            </div>
                        </TableCell>
                        <TableCell>
                            <div className="flex max-w-xs flex-wrap gap-1">
                                {k.scopes.map((s) => (
                                    <Badge
                                        key={s}
                                        variant="outline"
                                        className="font-mono text-xs"
                                    >
                                        {s}
                                    </Badge>
                                ))}
                            </div>
                        </TableCell>
                        <TableCell className="text-sm text-muted-foreground">
                            {formatDateTime(k.last_used_at)}
                        </TableCell>
                        <TableCell className="text-sm text-muted-foreground">
                            {k.expires_at
                                ? formatDateTime(k.expires_at)
                                : 'Never'}
                        </TableCell>
                        <TableCell className="text-right">
                            <Button
                                size="sm"
                                variant="outline"
                                disabled={pending}
                                onClick={() =>
                                    startTransition(
                                        async () =>
                                            void notify(
                                                await revokeApiKeyAction(k.id),
                                            ),
                                    )
                                }
                            >
                                Revoke
                            </Button>
                        </TableCell>
                    </TableRow>
                ))}
            </TableBody>
        </Table>
    );
}

export function CreateKeyDialog({
    scopes,
}: {
    scopes: { key: string; description: string }[];
}) {
    const { guard } = useReauth();
    const [open, setOpen] = useState(false);
    const [idem, setIdem] = useState(newIdempotencyKey);
    const [pending, startTransition] = useTransition();
    const [result, setResult] = useState<ActionResult<
        Schemas['ApiKeyCreated']
    > | null>(null);
    const created = result?.ok ? result.data : null;

    const submit = (form: FormData) =>
        startTransition(async () => {
            form.set('idempotency_key', idem);
            const r = await guard(() => createApiKeyAction(null, form));
            setResult(r);
            if (!r.ok) toast.error(r.message);
        });

    return (
        <Dialog
            open={open}
            onOpenChange={(o) => {
                setOpen(o);
                if (!o) {
                    setResult(null);
                    setIdem(newIdempotencyKey());
                }
            }}
        >
            <DialogTrigger asChild>
                <Button>
                    <Plus /> Create key
                </Button>
            </DialogTrigger>
            <DialogContent className="max-h-[90svh] overflow-y-auto">
                <DialogHeader>
                    <DialogTitle>Create an API key</DialogTitle>
                    <DialogDescription>
                        Grant only the scopes the integration needs.
                    </DialogDescription>
                </DialogHeader>
                {created ? (
                    <div className="space-y-3 text-sm">
                        <Alert>
                            <AlertDescription>
                                Copy the key now. For your security it will not
                                be shown again.
                            </AlertDescription>
                        </Alert>
                        <div className="flex gap-2">
                            <Input
                                readOnly
                                value={created.secret}
                                className="font-mono text-xs"
                            />
                            <Button
                                type="button"
                                variant="outline"
                                size="icon"
                                aria-label="Copy key"
                                onClick={() => {
                                    void navigator.clipboard.writeText(
                                        created.secret,
                                    );
                                    toast.success('Key copied');
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
                    <form action={submit} className="space-y-4">
                        <FieldGroup>
                            <TextField
                                label="Name"
                                name="name"
                                required
                                maxLength={100}
                                placeholder="CRM sync"
                                error={
                                    result && !result.ok
                                        ? result.fieldErrors?.name
                                        : undefined
                                }
                            />
                            <TextField
                                label="Expires after (days)"
                                name="expires_in_days"
                                type="number"
                                min={1}
                                max={3650}
                                hint="Leave empty for a key that does not expire."
                            />
                            <fieldset className="space-y-2">
                                <legend className="mb-2 text-sm font-medium">
                                    Scopes
                                </legend>
                                {scopes.map((s) => (
                                    <div
                                        key={s.key}
                                        className="flex items-start gap-2"
                                    >
                                        <Checkbox
                                            id={`scope-${s.key}`}
                                            name="scopes"
                                            value={s.key}
                                        />
                                        <Label
                                            htmlFor={`scope-${s.key}`}
                                            className="grid gap-0.5 font-normal"
                                        >
                                            <span className="font-mono text-xs">
                                                {s.key}
                                            </span>
                                            {s.description !== s.key && (
                                                <span className="text-xs text-muted-foreground">
                                                    {s.description}
                                                </span>
                                            )}
                                        </Label>
                                    </div>
                                ))}
                            </fieldset>
                        </FieldGroup>
                        <Button
                            type="submit"
                            className="w-full"
                            disabled={pending}
                        >
                            {pending && <Spinner />} Create key
                        </Button>
                    </form>
                )}
            </DialogContent>
        </Dialog>
    );
}
