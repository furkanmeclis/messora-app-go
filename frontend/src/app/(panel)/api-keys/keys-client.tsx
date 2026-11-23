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
import {
    Field,
    FieldDescription,
    FieldGroup,
    FieldLabel,
} from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { NumberField } from '@/components/ui/number-field';
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
import { useT } from '@/i18n/client';
import type { ActionResult, ApiKey, Schemas } from '@/lib/types';
import { createApiKeyAction, revokeApiKeyAction } from '../actions';

export function ApiKeyTable({ keys }: { keys: ApiKey[] }) {
    const t = useT();
    const formatDateTime = useFormatDateTime();
    const [pending, startTransition] = useTransition();
    if (keys.length === 0) {
        return (
            <Empty className="py-12">
                <EmptyHeader>
                    <EmptyMedia variant="icon">
                        <KeyRound />
                    </EmptyMedia>
                    <EmptyTitle>{t('apiKeys.empty')}</EmptyTitle>
                    <EmptyDescription>
                        {t('apiKeys.emptyHint')}
                    </EmptyDescription>
                </EmptyHeader>
            </Empty>
        );
    }
    return (
        <Table>
            <TableHeader>
                <TableRow>
                    <TableHead>{t('apiKeys.columns.name')}</TableHead>
                    <TableHead>{t('apiKeys.columns.scopes')}</TableHead>
                    <TableHead>{t('apiKeys.columns.lastUsed')}</TableHead>
                    <TableHead>{t('apiKeys.columns.expires')}</TableHead>
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
                                : t('apiKeys.never')}
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
                                {t('apiKeys.revoke')}
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
    const t = useT();
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
                <Button shortcut="n">
                    <Plus /> {t('apiKeys.create.button')}
                </Button>
            </DialogTrigger>
            <DialogContent className="max-h-[90svh] overflow-y-auto">
                <DialogHeader>
                    <DialogTitle>{t('apiKeys.create.title')}</DialogTitle>
                    <DialogDescription>
                        {t('apiKeys.create.description')}
                    </DialogDescription>
                </DialogHeader>
                {created ? (
                    <div className="space-y-3 text-sm">
                        <Alert>
                            <AlertDescription>
                                {t('apiKeys.create.copyNow')}
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
                                aria-label={t('apiKeys.create.copyKey')}
                                onClick={() => {
                                    void navigator.clipboard.writeText(
                                        created.secret,
                                    );
                                    toast.success(
                                        t('apiKeys.create.keyCopied'),
                                    );
                                }}
                            >
                                <Copy />
                            </Button>
                        </div>
                        <Button
                            className="w-full"
                            onClick={() => setOpen(false)}
                        >
                            {t('apiKeys.create.done')}
                        </Button>
                    </div>
                ) : (
                    <form action={submit} className="space-y-4">
                        <FieldGroup>
                            <TextField
                                label={t('apiKeys.create.name')}
                                name="name"
                                required
                                maxLength={100}
                                placeholder={t(
                                    'apiKeys.create.namePlaceholder',
                                )}
                                error={
                                    result && !result.ok
                                        ? result.fieldErrors?.name
                                        : undefined
                                }
                            />
                            <Field>
                                <FieldLabel htmlFor="field-expires_in_days">
                                    {t('apiKeys.create.expiresAfter')}
                                </FieldLabel>
                                <NumberField
                                    id="field-expires_in_days"
                                    name="expires_in_days"
                                    min={1}
                                    max={3650}
                                    allowEmpty
                                />
                                <FieldDescription>
                                    {t('apiKeys.create.expiresHint')}
                                </FieldDescription>
                            </Field>
                            <fieldset className="space-y-2">
                                <legend className="mb-2 text-sm font-medium">
                                    {t('apiKeys.create.scopes')}
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
                            {pending && <Spinner />}{' '}
                            {t('apiKeys.create.button')}
                        </Button>
                    </form>
                )}
            </DialogContent>
        </Dialog>
    );
}
