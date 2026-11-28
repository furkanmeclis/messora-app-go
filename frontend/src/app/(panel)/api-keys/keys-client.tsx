'use client';

import { useState, useTransition } from 'react';
import { Ban, Copy, KeyRound, Plus } from 'lucide-react';
import { toast } from 'sonner';
import { ClientDataTable } from '@/components/client-data-table';
import { useConfirm } from '@/components/confirm';
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
import { useFormatDateTime } from '@/components/display-preferences';
import { useT } from '@/i18n/client';
import type { ActionResult, ApiKey, Schemas } from '@/lib/types';
import { createApiKeyAction, revokeApiKeyAction } from '../actions';

/** The workspace's API base URL with a copy button and a curl sample. */
export function ApiBaseCard({ baseUrl }: { baseUrl: string }) {
    const t = useT();
    const copy = async () => {
        try {
            await navigator.clipboard.writeText(baseUrl);
            toast.success(t('apiKeys.base.copied'));
        } catch {
            // Clipboard access can be refused; the URL stays selectable.
        }
    };
    return (
        <section
            aria-labelledby="api-base-title"
            className="mb-4 grid gap-2 rounded-lg border bg-card p-4"
            data-testid="api-base"
        >
            <div className="flex flex-wrap items-center justify-between gap-2">
                <h2 id="api-base-title" className="text-sm font-medium">
                    {t('apiKeys.base.title')}
                </h2>
                <Button size="sm" variant="outline" onClick={copy}>
                    <Copy /> {t('apiKeys.base.copy')}
                </Button>
            </div>
            <code
                className="rounded bg-muted px-2 py-1 font-mono text-sm break-all select-all"
                data-testid="api-base-url"
            >
                {baseUrl}
            </code>
            <p className="text-xs text-muted-foreground">
                {t('apiKeys.base.description')}
            </p>
            <p className="text-xs font-medium">{t('apiKeys.base.sample')}</p>
            <pre className="overflow-x-auto rounded bg-muted px-2 py-1 font-mono text-xs">
                {`curl ${baseUrl}/api-keys/whoami \\\n  -H "Authorization: Bearer msk_live_…"`}
            </pre>
        </section>
    );
}

export function ApiKeyTable({ keys }: { keys: ApiKey[] }) {
    const t = useT();
    const ask = useConfirm();
    const formatDateTime = useFormatDateTime();
    const [pending, startTransition] = useTransition();

    const revokeOne = (id: string) =>
        startTransition(async () => void notify(await revokeApiKeyAction(id)));
    const revokeMany = async (ids: string[], clear: () => void) => {
        const ok = await ask({
            title: t('apiKeys.revokeManyTitle', { count: ids.length }),
            confirmLabel: t('apiKeys.revoke'),
            destructive: true,
        });
        if (!ok) return;
        startTransition(async () => {
            let revoked = 0;
            for (const id of ids) {
                const r = await revokeApiKeyAction(id);
                if (!r.ok) {
                    toast.error(r.message);
                    break;
                }
                revoked++;
            }
            if (revoked > 0) {
                toast.success(t('apiKeys.revokedMany', { count: revoked }));
                clear();
            }
        });
    };

    return (
        <ClientDataTable
            rows={keys}
            rowId={(k) => k.id}
            storageKey="api-keys"
            initialSort="name"
            searchText={(k) => `${k.name} ${k.prefix} ${k.scopes.join(' ')}`}
            searchPlaceholder={t('table.search')}
            empty={
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
            }
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
                    onClick={() => void revokeMany(ids, clear)}
                >
                    {pending ? <Spinner /> : <Ban />}
                    {t('apiKeys.revoke')}
                </Button>
            )}
            columns={[
                {
                    id: 'name',
                    header: t('apiKeys.columns.name'),
                    hideable: false,
                    sortValue: (k) => k.name,
                    cell: (k) => (
                        <div className="min-w-0">
                            <div className="font-medium">{k.name}</div>
                            <div className="font-mono text-xs text-muted-foreground">
                                {k.prefix}…
                            </div>
                        </div>
                    ),
                },
                {
                    id: 'scopes',
                    header: t('apiKeys.columns.scopes'),
                    sortValue: (k) => k.scopes.length,
                    cell: (k) => (
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
                    ),
                },
                {
                    id: 'lastUsed',
                    header: t('apiKeys.columns.lastUsed'),
                    sortValue: (k) => k.last_used_at,
                    cell: (k) => (
                        <span className="text-sm whitespace-nowrap text-muted-foreground">
                            {formatDateTime(k.last_used_at)}
                        </span>
                    ),
                },
                {
                    id: 'expires',
                    header: t('apiKeys.columns.expires'),
                    sortValue: (k) => k.expires_at,
                    cell: (k) => (
                        <span className="text-sm whitespace-nowrap text-muted-foreground">
                            {k.expires_at
                                ? formatDateTime(k.expires_at)
                                : t('apiKeys.never')}
                        </span>
                    ),
                },
            ]}
            rowActions={(k) => (
                <Button
                    size="sm"
                    variant="outline"
                    disabled={pending}
                    onClick={() => revokeOne(k.id)}
                >
                    {t('apiKeys.revoke')}
                </Button>
            )}
            rowMenu={(k) => [
                [
                    {
                        id: 'revoke',
                        label: t('apiKeys.revoke'),
                        icon: Ban,
                        danger: true,
                        disabled: pending,
                        run: () => revokeOne(k.id),
                    },
                ],
            ]}
        />
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
