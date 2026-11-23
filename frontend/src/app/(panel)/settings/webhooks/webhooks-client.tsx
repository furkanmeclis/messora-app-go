'use client';

import { useCallback, useEffect, useState, useTransition } from 'react';
import {
    Copy,
    MoreHorizontal,
    Pencil,
    Plus,
    RotateCw,
    ScrollText,
    Send,
    Trash2,
    Webhook,
} from 'lucide-react';
import { toast } from 'sonner';
import { TextField } from '@/components/forms';
import { newIdempotencyKey, notify } from '@/components/notify';
import { Alert, AlertDescription } from '@/components/ui/alert';
import {
    AlertDialog,
    AlertDialogAction,
    AlertDialogCancel,
    AlertDialogContent,
    AlertDialogDescription,
    AlertDialogFooter,
    AlertDialogHeader,
    AlertDialogTitle,
} from '@/components/ui/alert-dialog';
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
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuSeparator,
    DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
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
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from '@/components/ui/select';
import {
    Sheet,
    SheetContent,
    SheetDescription,
    SheetHeader,
    SheetTitle,
} from '@/components/ui/sheet';
import { Spinner } from '@/components/ui/spinner';
import { Switch } from '@/components/ui/switch';
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
import type { ActionResult, Schemas } from '@/lib/types';
import {
    createWebhookAction,
    deleteWebhookAction,
    loadWebhookDeliveries,
    redeliverWebhookAction,
    rotateWebhookSecretAction,
    sendWebhookTestAction,
    setWebhookActiveAction,
    updateWebhookAction,
} from './actions';

type Endpoint = Schemas['WebhookEndpoint'];
type EventInfo = Schemas['WebhookEventTypeInfo'];
type Delivery = Schemas['WebhookDelivery'];
type DeliveryStatus = Schemas['WebhookDeliveryStatus'];

/** Shows a signing secret once, with a copy button. */
function SecretNotice({ secret }: { secret: string }) {
    const t = useT();
    return (
        <div className="space-y-3 text-sm">
            <Alert>
                <AlertDescription>
                    {t('webhooks.secret.notice')}
                </AlertDescription>
            </Alert>
            <div className="flex gap-2">
                <Input
                    readOnly
                    value={secret}
                    className="font-mono text-xs"
                    aria-label={t('webhooks.secret.label')}
                />
                <Button
                    type="button"
                    variant="outline"
                    size="icon"
                    aria-label={t('webhooks.secret.copy')}
                    onClick={() => {
                        void navigator.clipboard.writeText(secret);
                        toast.success(t('webhooks.secret.copied'));
                    }}
                >
                    <Copy />
                </Button>
            </div>
        </div>
    );
}

function EndpointFields({
    catalogue,
    endpoint,
    errors,
}: {
    catalogue: EventInfo[];
    endpoint?: Endpoint;
    errors?: Record<string, string>;
}) {
    const t = useT();
    const selected = new Set(endpoint?.event_types ?? []);
    return (
        <FieldGroup>
            <TextField
                label={t('webhooks.fields.url')}
                name="url"
                type="url"
                required
                maxLength={2048}
                placeholder={t('webhooks.fields.urlPlaceholder')}
                defaultValue={endpoint?.url}
                error={errors?.url}
                hint={t('webhooks.fields.urlHint')}
            />
            <TextField
                label={t('webhooks.fields.description')}
                name="description"
                maxLength={500}
                placeholder={t('webhooks.fields.descriptionPlaceholder')}
                defaultValue={endpoint?.description}
                error={errors?.description}
            />
            <fieldset className="space-y-2">
                <legend className="mb-2 text-sm font-medium">
                    {t('webhooks.fields.events')}
                </legend>
                {catalogue.map((e) => (
                    <div key={e.type} className="flex items-start gap-2">
                        <Checkbox
                            id={`event-${e.type}`}
                            name="event_types"
                            value={e.type}
                            defaultChecked={selected.has(e.type)}
                        />
                        <Label
                            htmlFor={`event-${e.type}`}
                            className="grid gap-0.5 font-normal"
                        >
                            <span className="font-mono text-xs">{e.type}</span>
                            <span className="text-xs text-muted-foreground">
                                {e.description}
                            </span>
                        </Label>
                    </div>
                ))}
                {errors?.event_types && (
                    <p role="alert" className="text-sm text-destructive">
                        {errors.event_types}
                    </p>
                )}
            </fieldset>
            <div className="flex items-center gap-2">
                <Switch
                    id="endpoint-active"
                    name="active"
                    defaultChecked={endpoint?.active ?? true}
                />
                <Label htmlFor="endpoint-active">
                    {t('webhooks.fields.active')}
                </Label>
            </div>
        </FieldGroup>
    );
}

/** Create dialog; the edit variant is EditEndpointDialog. */
export function EndpointDialog({ catalogue }: { catalogue: EventInfo[] }) {
    const t = useT();
    const [open, setOpen] = useState(false);
    const [idem, setIdem] = useState(newIdempotencyKey);
    const [pending, startTransition] = useTransition();
    const [result, setResult] = useState<ActionResult<
        Schemas['WebhookEndpointSecret']
    > | null>(null);
    const created = result?.ok ? result.data : null;

    const submit = (form: FormData) =>
        startTransition(async () => {
            form.set('idempotency_key', idem);
            const r = await createWebhookAction(null, form);
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
                    <Plus /> {t('webhooks.add.button')}
                </Button>
            </DialogTrigger>
            <DialogContent className="max-h-[90svh] overflow-y-auto">
                <DialogHeader>
                    <DialogTitle>{t('webhooks.add.title')}</DialogTitle>
                    <DialogDescription>
                        {t('webhooks.add.description')}
                    </DialogDescription>
                </DialogHeader>
                {created ? (
                    <div className="space-y-3">
                        <SecretNotice secret={created.secret} />
                        <Button
                            className="w-full"
                            onClick={() => setOpen(false)}
                        >
                            {t('webhooks.add.done')}
                        </Button>
                    </div>
                ) : (
                    <form action={submit} className="space-y-4">
                        <EndpointFields
                            catalogue={catalogue}
                            errors={
                                result && !result.ok
                                    ? result.fieldErrors
                                    : undefined
                            }
                        />
                        <Button
                            type="submit"
                            className="w-full"
                            disabled={pending}
                        >
                            {pending && <Spinner />} {t('webhooks.add.button')}
                        </Button>
                    </form>
                )}
            </DialogContent>
        </Dialog>
    );
}

function EditEndpointDialog({
    endpoint,
    catalogue,
    open,
    onOpenChange,
}: {
    endpoint: Endpoint;
    catalogue: EventInfo[];
    open: boolean;
    onOpenChange: (o: boolean) => void;
}) {
    const t = useT();
    const [pending, startTransition] = useTransition();
    const [result, setResult] = useState<ActionResult | null>(null);
    const submit = (form: FormData) =>
        startTransition(async () => {
            const r = await updateWebhookAction(endpoint.id, null, form);
            setResult(r);
            if (notify(r)) onOpenChange(false);
        });
    return (
        <Dialog
            open={open}
            onOpenChange={(o) => {
                onOpenChange(o);
                if (!o) setResult(null);
            }}
        >
            <DialogContent className="max-h-[90svh] overflow-y-auto">
                <DialogHeader>
                    <DialogTitle>{t('webhooks.edit.title')}</DialogTitle>
                    <DialogDescription>
                        {t('webhooks.edit.description')}
                    </DialogDescription>
                </DialogHeader>
                <form action={submit} className="space-y-4">
                    <EndpointFields
                        catalogue={catalogue}
                        endpoint={endpoint}
                        errors={
                            result && !result.ok
                                ? result.fieldErrors
                                : undefined
                        }
                    />
                    <Button type="submit" className="w-full" disabled={pending}>
                        {pending && <Spinner />} {t('common.save')}
                    </Button>
                </form>
            </DialogContent>
        </Dialog>
    );
}

type Confirm = 'rotate' | 'delete' | null;

function EndpointRow({
    endpoint,
    catalogue,
}: {
    endpoint: Endpoint;
    catalogue: EventInfo[];
}) {
    const t = useT();
    const [pending, startTransition] = useTransition();
    const [editing, setEditing] = useState(false);
    const [logOpen, setLogOpen] = useState(false);
    const [confirm, setConfirm] = useState<Confirm>(null);
    const [secret, setSecret] = useState<string | null>(null);

    const run = (fn: () => Promise<ActionResult<unknown>>) =>
        startTransition(async () => void notify(await fn()));

    return (
        <TableRow>
            <TableCell className="max-w-sm">
                <div
                    className="truncate font-mono text-xs"
                    title={endpoint.url}
                >
                    {endpoint.url}
                </div>
                {endpoint.description && (
                    <div className="truncate text-sm text-muted-foreground">
                        {endpoint.description}
                    </div>
                )}
            </TableCell>
            <TableCell>
                <div className="flex max-w-xs flex-wrap gap-1">
                    {endpoint.event_types.map((type) => (
                        <Badge
                            key={type}
                            variant="outline"
                            className="font-mono text-xs"
                        >
                            {type}
                        </Badge>
                    ))}
                </div>
            </TableCell>
            <TableCell>
                {endpoint.active ? (
                    <Badge variant="secondary">{t('webhooks.active')}</Badge>
                ) : (
                    <div className="grid gap-1">
                        <Badge variant="destructive">
                            {t('webhooks.disabled')}
                        </Badge>
                        {endpoint.disabled_reason && (
                            <span className="text-xs text-muted-foreground">
                                {endpoint.disabled_reason}
                            </span>
                        )}
                    </div>
                )}
            </TableCell>
            <TableCell className="font-mono text-xs text-muted-foreground">
                whsec_…{endpoint.secret_last4}
            </TableCell>
            <TableCell className="text-right whitespace-nowrap">
                <Button
                    size="sm"
                    variant="outline"
                    onClick={() => setLogOpen(true)}
                >
                    <ScrollText /> {t('webhooks.deliveries')}
                </Button>
                <DropdownMenu>
                    <DropdownMenuTrigger asChild>
                        <Button
                            size="icon"
                            variant="ghost"
                            aria-label={t('webhooks.moreActions')}
                            disabled={pending}
                            className="ml-1"
                        >
                            {pending ? <Spinner /> : <MoreHorizontal />}
                        </Button>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent align="end">
                        <DropdownMenuItem onSelect={() => setEditing(true)}>
                            <Pencil /> {t('common.edit')}
                        </DropdownMenuItem>
                        <DropdownMenuItem
                            onSelect={() =>
                                run(() => sendWebhookTestAction(endpoint.id))
                            }
                        >
                            <Send /> {t('webhooks.sendTest')}
                        </DropdownMenuItem>
                        <DropdownMenuItem
                            onSelect={() =>
                                run(() =>
                                    setWebhookActiveAction(
                                        endpoint.id,
                                        !endpoint.active,
                                    ),
                                )
                            }
                        >
                            <Webhook />{' '}
                            {endpoint.active
                                ? t('webhooks.disable')
                                : t('webhooks.enable')}
                        </DropdownMenuItem>
                        <DropdownMenuItem onSelect={() => setConfirm('rotate')}>
                            <RotateCw /> {t('webhooks.rotateSecret')}
                        </DropdownMenuItem>
                        <DropdownMenuSeparator />
                        <DropdownMenuItem
                            variant="destructive"
                            onSelect={() => setConfirm('delete')}
                        >
                            <Trash2 /> {t('common.delete')}
                        </DropdownMenuItem>
                    </DropdownMenuContent>
                </DropdownMenu>

                <EditEndpointDialog
                    endpoint={endpoint}
                    catalogue={catalogue}
                    open={editing}
                    onOpenChange={setEditing}
                />
                <DeliveryLog
                    endpoint={endpoint}
                    open={logOpen}
                    onOpenChange={setLogOpen}
                />
                <AlertDialog
                    open={confirm !== null}
                    onOpenChange={(o) => !o && setConfirm(null)}
                >
                    <AlertDialogContent>
                        <AlertDialogHeader>
                            <AlertDialogTitle>
                                {confirm === 'rotate'
                                    ? t('webhooks.confirmRotateTitle')
                                    : t('webhooks.confirmDeleteTitle')}
                            </AlertDialogTitle>
                            <AlertDialogDescription>
                                {confirm === 'rotate'
                                    ? t('webhooks.confirmRotateBody')
                                    : t('webhooks.confirmDeleteBody')}
                            </AlertDialogDescription>
                        </AlertDialogHeader>
                        <AlertDialogFooter>
                            <AlertDialogCancel>
                                {t('common.cancel')}
                            </AlertDialogCancel>
                            <AlertDialogAction
                                variant={
                                    confirm === 'delete'
                                        ? 'destructive'
                                        : 'default'
                                }
                                onClick={() => {
                                    if (confirm === 'delete') {
                                        run(() =>
                                            deleteWebhookAction(endpoint.id),
                                        );
                                        return;
                                    }
                                    startTransition(async () => {
                                        const r =
                                            await rotateWebhookSecretAction(
                                                endpoint.id,
                                            );
                                        if (notify(r)) setSecret(r.data);
                                    });
                                }}
                            >
                                {confirm === 'rotate'
                                    ? t('webhooks.rotate')
                                    : t('common.delete')}
                            </AlertDialogAction>
                        </AlertDialogFooter>
                    </AlertDialogContent>
                </AlertDialog>
                <Dialog
                    open={secret !== null}
                    onOpenChange={(o) => !o && setSecret(null)}
                >
                    <DialogContent>
                        <DialogHeader>
                            <DialogTitle>{t('webhooks.newSecret')}</DialogTitle>
                        </DialogHeader>
                        {secret && <SecretNotice secret={secret} />}
                    </DialogContent>
                </Dialog>
            </TableCell>
        </TableRow>
    );
}

export function EndpointTable({
    endpoints,
    catalogue,
}: {
    endpoints: Endpoint[];
    catalogue: EventInfo[];
}) {
    const t = useT();
    if (endpoints.length === 0) {
        return (
            <Empty className="py-12">
                <EmptyHeader>
                    <EmptyMedia variant="icon">
                        <Webhook />
                    </EmptyMedia>
                    <EmptyTitle>{t('webhooks.empty')}</EmptyTitle>
                    <EmptyDescription>
                        {t('webhooks.emptyHint')}
                    </EmptyDescription>
                </EmptyHeader>
            </Empty>
        );
    }
    return (
        <Table>
            <TableHeader>
                <TableRow>
                    <TableHead>{t('webhooks.columns.endpoint')}</TableHead>
                    <TableHead>{t('webhooks.columns.events')}</TableHead>
                    <TableHead>{t('webhooks.columns.status')}</TableHead>
                    <TableHead>{t('webhooks.columns.secret')}</TableHead>
                    <TableHead />
                </TableRow>
            </TableHeader>
            <TableBody>
                {endpoints.map((e) => (
                    <EndpointRow
                        key={e.id}
                        endpoint={e}
                        catalogue={catalogue}
                    />
                ))}
            </TableBody>
        </Table>
    );
}

const STATUS_VARIANT: Record<
    DeliveryStatus,
    'secondary' | 'destructive' | 'outline' | 'default'
> = {
    succeeded: 'secondary',
    failed: 'destructive',
    retrying: 'outline',
    pending: 'outline',
    delivering: 'default',
};

function DeliveryItem({
    d,
    onChange,
}: {
    d: Delivery;
    onChange: (d: Delivery) => void;
}) {
    const t = useT();
    const formatDateTime = useFormatDateTime();
    const [pending, startTransition] = useTransition();
    const [open, setOpen] = useState(false);
    return (
        <li className="space-y-2 border-b py-3 last:border-b-0">
            <div className="flex flex-wrap items-center gap-2">
                <Badge variant={STATUS_VARIANT[d.status]}>
                    {t(`webhooks.deliveryStatuses.${d.status}`)}
                </Badge>
                <span className="font-mono text-xs">{d.event_type}</span>
                <span className="ml-auto text-xs text-muted-foreground">
                    {formatDateTime(d.last_attempt_at ?? d.created_at)}
                </span>
            </div>
            <div className="flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-muted-foreground">
                <span>{t('webhooks.attempts', { count: d.attempts })}</span>
                {d.last_status_code != null && (
                    <span>HTTP {d.last_status_code}</span>
                )}
                {d.last_duration_ms != null && (
                    <span>{d.last_duration_ms} ms</span>
                )}
                {d.next_attempt_at && (
                    <span>
                        {t('webhooks.nextTry', {
                            time: formatDateTime(d.next_attempt_at),
                        })}
                    </span>
                )}
                <span className="ml-auto flex gap-1">
                    {(d.last_error || d.last_response_body) && (
                        <Button
                            size="sm"
                            variant="ghost"
                            onClick={() => setOpen((o) => !o)}
                        >
                            {open
                                ? t('webhooks.hideDetails')
                                : t('webhooks.details')}
                        </Button>
                    )}
                    <Button
                        size="sm"
                        variant="outline"
                        disabled={pending || d.status === 'delivering'}
                        onClick={() =>
                            startTransition(async () => {
                                const r = await redeliverWebhookAction(d.id);
                                if (notify(r)) onChange(r.data);
                            })
                        }
                    >
                        {pending ? <Spinner /> : <RotateCw />}{' '}
                        {t('webhooks.redeliver')}
                    </Button>
                </span>
            </div>
            {open && (
                <div className="space-y-2 rounded-md bg-muted p-2 text-xs">
                    <div className="font-mono text-muted-foreground">
                        {t('webhooks.event', { id: d.event_id })}
                    </div>
                    {d.last_error && (
                        <div className="text-destructive">{d.last_error}</div>
                    )}
                    {d.last_response_body && (
                        <pre className="max-h-40 overflow-auto font-mono break-all whitespace-pre-wrap">
                            {d.last_response_body}
                        </pre>
                    )}
                </div>
            )}
        </li>
    );
}

function DeliveryLog({
    endpoint,
    open,
    onOpenChange,
}: {
    endpoint: Endpoint;
    open: boolean;
    onOpenChange: (o: boolean) => void;
}) {
    const t = useT();
    const [status, setStatus] = useState<DeliveryStatus | 'all'>('all');
    const [items, setItems] = useState<Delivery[]>([]);
    const [next, setNext] = useState<string | undefined>();
    const [loading, startLoading] = useTransition();

    const load = useCallback(
        (cursor?: string) =>
            startLoading(async () => {
                const r = await loadWebhookDeliveries(
                    endpoint.id,
                    cursor,
                    status === 'all' ? undefined : status,
                );
                if (!notify(r)) return;
                setItems((prev) =>
                    cursor
                        ? [...prev, ...r.data.deliveries]
                        : r.data.deliveries,
                );
                setNext(r.data.next);
            }),
        [endpoint.id, status],
    );

    useEffect(() => {
        if (open) load();
    }, [open, load]);

    return (
        <Sheet open={open} onOpenChange={onOpenChange}>
            <SheetContent className="w-full overflow-y-auto sm:max-w-xl">
                <SheetHeader>
                    <SheetTitle>{t('webhooks.log.title')}</SheetTitle>
                    <SheetDescription className="truncate font-mono text-xs">
                        {endpoint.url}
                    </SheetDescription>
                </SheetHeader>
                <div className="flex items-center gap-2 px-4">
                    <Select
                        value={status}
                        onValueChange={(v) =>
                            setStatus(v as DeliveryStatus | 'all')
                        }
                    >
                        <SelectTrigger
                            className="w-40"
                            aria-label={t('webhooks.log.status')}
                        >
                            <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                            <SelectItem value="all">
                                {t('webhooks.log.allStatuses')}
                            </SelectItem>
                            <SelectItem value="succeeded">
                                {t('webhooks.log.succeeded')}
                            </SelectItem>
                            <SelectItem value="failed">
                                {t('webhooks.log.failed')}
                            </SelectItem>
                            <SelectItem value="retrying">
                                {t('webhooks.log.retrying')}
                            </SelectItem>
                            <SelectItem value="pending">
                                {t('webhooks.log.pending')}
                            </SelectItem>
                        </SelectContent>
                    </Select>
                    <Button
                        variant="outline"
                        size="sm"
                        disabled={loading}
                        onClick={() => load()}
                    >
                        {loading ? <Spinner /> : <RotateCw />}{' '}
                        {t('webhooks.log.refresh')}
                    </Button>
                </div>
                <div className="px-4 pb-4">
                    {items.length === 0 && !loading ? (
                        <p className="py-8 text-center text-sm text-muted-foreground">
                            {t('webhooks.log.empty')}
                        </p>
                    ) : (
                        <ul>
                            {items.map((d) => (
                                <DeliveryItem
                                    key={d.id}
                                    d={d}
                                    onChange={(u) =>
                                        setItems((prev) =>
                                            prev.map((x) =>
                                                x.id === u.id ? u : x,
                                            ),
                                        )
                                    }
                                />
                            ))}
                        </ul>
                    )}
                    {next && (
                        <Button
                            variant="outline"
                            className="mt-3 w-full"
                            disabled={loading}
                            onClick={() => load(next)}
                        >
                            {t('webhooks.log.loadMore')}
                        </Button>
                    )}
                </div>
            </SheetContent>
        </Sheet>
    );
}
