'use client';

import {
    Fragment,
    useCallback,
    useEffect,
    useState,
    useTransition,
} from 'react';
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
import { ClientDataTable } from '@/components/client-data-table';
import type { MenuGroup } from '@/components/context-menu';
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
import { useFormatDateTime } from '@/components/display-preferences';
import { useCursorRows } from '@/hooks/use-cursor-rows';
import { useT } from '@/i18n/client';
import { NUMBER_LOCALE } from '@/lib/format';
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
                <Button shortcut="n">
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

type Confirm = { kind: 'rotate' | 'delete'; endpoint: Endpoint } | null;

export function EndpointTable({
    endpoints,
    catalogue,
}: {
    endpoints: Endpoint[];
    catalogue: EventInfo[];
}) {
    const t = useT();
    const [pending, startTransition] = useTransition();
    const [busyId, setBusyId] = useState<string | null>(null);
    // The endpoint a dialog shows stays set while the dialog animates out.
    const [editFor, setEditFor] = useState<Endpoint | null>(null);
    const [editing, setEditing] = useState(false);
    const [logFor, setLogFor] = useState<Endpoint | null>(null);
    const [logOpen, setLogOpen] = useState(false);
    const [confirm, setConfirm] = useState<Confirm>(null);
    const [secret, setSecret] = useState<string | null>(null);

    const run = (id: string, fn: () => Promise<ActionResult<unknown>>) =>
        startTransition(async () => {
            setBusyId(id);
            notify(await fn());
            setBusyId(null);
        });
    const edit = (e: Endpoint) => {
        setEditFor(e);
        setEditing(true);
    };
    const openLog = (e: Endpoint) => {
        setLogFor(e);
        setLogOpen(true);
    };

    const actions = (e: Endpoint): MenuGroup[] => [
        [
            {
                id: 'deliveries',
                label: t('webhooks.deliveries'),
                icon: ScrollText,
                run: () => openLog(e),
            },
            {
                id: 'edit',
                label: t('common.edit'),
                icon: Pencil,
                run: () => edit(e),
            },
            {
                id: 'test',
                label: t('webhooks.sendTest'),
                icon: Send,
                disabled: pending,
                run: () => run(e.id, () => sendWebhookTestAction(e.id)),
            },
            {
                id: 'active',
                label: e.active ? t('webhooks.disable') : t('webhooks.enable'),
                icon: Webhook,
                disabled: pending,
                run: () =>
                    run(e.id, () => setWebhookActiveAction(e.id, !e.active)),
            },
            {
                id: 'rotate',
                label: t('webhooks.rotateSecret'),
                icon: RotateCw,
                run: () => setConfirm({ kind: 'rotate', endpoint: e }),
            },
        ],
        [
            {
                id: 'delete',
                label: t('common.delete'),
                icon: Trash2,
                danger: true,
                run: () => setConfirm({ kind: 'delete', endpoint: e }),
            },
        ],
    ];

    return (
        <>
            <ClientDataTable
                rows={endpoints}
                rowId={(e) => e.id}
                storageKey="webhook-endpoints"
                initialSort="endpoint"
                searchText={(e) =>
                    `${e.url} ${e.description ?? ''} ${e.event_types.join(' ')}`
                }
                searchPlaceholder={t('table.search')}
                empty={
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
                }
                noMatch={
                    <p className="p-8 text-center text-sm text-muted-foreground">
                        {t('table.noMatch')}
                    </p>
                }
                columns={[
                    {
                        id: 'endpoint',
                        header: t('webhooks.columns.endpoint'),
                        hideable: false,
                        sortValue: (e) => e.url,
                        className: 'max-w-sm',
                        cell: (e) => (
                            <div className="min-w-0">
                                <div
                                    className="truncate font-mono text-xs"
                                    title={e.url}
                                >
                                    {e.url}
                                </div>
                                {e.description && (
                                    <div className="truncate text-sm text-muted-foreground">
                                        {e.description}
                                    </div>
                                )}
                            </div>
                        ),
                    },
                    {
                        id: 'events',
                        header: t('webhooks.columns.events'),
                        sortValue: (e) => e.event_types.length,
                        cell: (e) => (
                            <div className="flex max-w-xs flex-wrap gap-1">
                                {e.event_types.map((type) => (
                                    <Badge
                                        key={type}
                                        variant="outline"
                                        className="font-mono text-xs"
                                    >
                                        {type}
                                    </Badge>
                                ))}
                            </div>
                        ),
                    },
                    {
                        id: 'status',
                        header: t('webhooks.columns.status'),
                        sortValue: (e) => (e.active ? 0 : 1),
                        cell: (e) =>
                            e.active ? (
                                <Badge variant="secondary">
                                    {t('webhooks.active')}
                                </Badge>
                            ) : (
                                <div className="grid gap-1">
                                    <Badge variant="destructive">
                                        {t('webhooks.disabled')}
                                    </Badge>
                                    {e.disabled_reason && (
                                        <span className="text-xs text-muted-foreground">
                                            {e.disabled_reason}
                                        </span>
                                    )}
                                </div>
                            ),
                    },
                    {
                        id: 'secret',
                        header: t('webhooks.columns.secret'),
                        cell: (e) => (
                            <span className="font-mono text-xs text-muted-foreground">
                                whsec_…{e.secret_last4}
                            </span>
                        ),
                    },
                ]}
                rowActions={(e) => (
                    <span className="whitespace-nowrap">
                        <Button
                            size="sm"
                            variant="outline"
                            onClick={() => openLog(e)}
                        >
                            <ScrollText /> {t('webhooks.deliveries')}
                        </Button>
                        <DropdownMenu>
                            <DropdownMenuTrigger asChild>
                                <Button
                                    size="icon"
                                    variant="ghost"
                                    aria-label={t('webhooks.moreActions')}
                                    disabled={busyId === e.id}
                                    className="ml-1"
                                >
                                    {busyId === e.id ? (
                                        <Spinner />
                                    ) : (
                                        <MoreHorizontal />
                                    )}
                                </Button>
                            </DropdownMenuTrigger>
                            <DropdownMenuContent align="end">
                                {actions(e).map((group, gi) => (
                                    <Fragment key={gi}>
                                        {gi > 0 && <DropdownMenuSeparator />}
                                        {group
                                            .filter(
                                                (i) => i.id !== 'deliveries',
                                            )
                                            .map((i) => (
                                                <DropdownMenuItem
                                                    key={i.id}
                                                    variant={
                                                        i.danger
                                                            ? 'destructive'
                                                            : 'default'
                                                    }
                                                    disabled={i.disabled}
                                                    onSelect={i.run}
                                                >
                                                    {i.icon && <i.icon />}{' '}
                                                    {i.label}
                                                </DropdownMenuItem>
                                            ))}
                                    </Fragment>
                                ))}
                            </DropdownMenuContent>
                        </DropdownMenu>
                    </span>
                )}
                rowMenu={actions}
                onRowClick={openLog}
            />

            {editFor && (
                <EditEndpointDialog
                    key={editFor.id}
                    endpoint={editFor}
                    catalogue={catalogue}
                    open={editing}
                    onOpenChange={setEditing}
                />
            )}
            {logFor && (
                <DeliveryLog
                    key={logFor.id}
                    endpoint={logFor}
                    open={logOpen}
                    onOpenChange={setLogOpen}
                />
            )}
            <AlertDialog
                open={confirm !== null}
                onOpenChange={(o) => !o && setConfirm(null)}
            >
                <AlertDialogContent>
                    <AlertDialogHeader>
                        <AlertDialogTitle>
                            {confirm?.kind === 'rotate'
                                ? t('webhooks.confirmRotateTitle')
                                : t('webhooks.confirmDeleteTitle')}
                        </AlertDialogTitle>
                        <AlertDialogDescription>
                            {confirm?.kind === 'rotate'
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
                                confirm?.kind === 'delete'
                                    ? 'destructive'
                                    : 'default'
                            }
                            onClick={() => {
                                if (!confirm) return;
                                const id = confirm.endpoint.id;
                                if (confirm.kind === 'delete') {
                                    run(id, () => deleteWebhookAction(id));
                                    return;
                                }
                                startTransition(async () => {
                                    setBusyId(id);
                                    const r =
                                        await rotateWebhookSecretAction(id);
                                    setBusyId(null);
                                    if (notify(r)) setSecret(r.data);
                                });
                            }}
                        >
                            {confirm?.kind === 'rotate'
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
        </>
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

const NO_DELIVERIES: Delivery[] = [];

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
    const formatDateTime = useFormatDateTime();
    const [status, setStatus] = useState<DeliveryStatus | 'all'>('all');
    const [loading, startLoading] = useTransition();
    const [redelivering, startRedeliver] = useTransition();
    const [busyId, setBusyId] = useState<string | null>(null);
    const [details, setDetails] = useState<Delivery | null>(null);

    const fetchPage = useCallback(
        async (cursor?: string) => {
            const r = await loadWebhookDeliveries(
                endpoint.id,
                cursor,
                status === 'all' ? undefined : status,
            );
            return notify(r)
                ? { items: r.data.deliveries, next: r.data.next }
                : null;
        },
        [endpoint.id, status],
    );
    const {
        rows: items,
        paging,
        update,
        reset,
    } = useCursorRows<Delivery>({
        initial: NO_DELIVERIES,
        rowId: (d) => d.id,
        fetchPage,
    });
    // The first chunk; later ones follow the table's pager.
    const load = useCallback(
        () =>
            startLoading(async () => {
                const page = await fetchPage();
                if (page) reset(page.items, page.next);
            }),
        [fetchPage, reset],
    );

    useEffect(() => {
        if (open) load();
    }, [open, load]);

    const redeliver = (d: Delivery) =>
        startRedeliver(async () => {
            setBusyId(d.id);
            const r = await redeliverWebhookAction(d.id);
            setBusyId(null);
            if (notify(r)) update((x) => (x.id === r.data.id ? r.data : x));
        });
    const hasDetails = (d: Delivery) =>
        Boolean(d.last_error || d.last_response_body);

    return (
        <Sheet open={open} onOpenChange={onOpenChange}>
            <SheetContent className="w-full overflow-y-auto sm:max-w-4xl">
                <SheetHeader>
                    <SheetTitle>{t('webhooks.log.title')}</SheetTitle>
                    <SheetDescription className="truncate font-mono text-xs">
                        {endpoint.url}
                    </SheetDescription>
                </SheetHeader>
                <div className="space-y-3 px-4 pb-4">
                    <ClientDataTable
                        rows={items}
                        paging={paging}
                        rowId={(d) => d.id}
                        storageKey="webhook-deliveries"
                        initialSort="-when"
                        searchText={(d) =>
                            `${d.event_type} ${d.event_id} ${d.last_status_code ?? ''} ${d.last_error ?? ''}`
                        }
                        searchPlaceholder={t('table.search')}
                        toolbar={
                            <>
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
                            </>
                        }
                        empty={
                            <p className="py-8 text-center text-sm text-muted-foreground">
                                {loading
                                    ? t('table.loading')
                                    : t('webhooks.log.empty')}
                            </p>
                        }
                        noMatch={
                            <p className="p-8 text-center text-sm text-muted-foreground">
                                {t('table.noMatch')}
                            </p>
                        }
                        columns={[
                            {
                                id: 'status',
                                header: t('webhooks.log.status'),
                                sortValue: (d) => d.status,
                                cell: (d) => (
                                    <Badge variant={STATUS_VARIANT[d.status]}>
                                        {t(
                                            `webhooks.deliveryStatuses.${d.status}`,
                                        )}
                                    </Badge>
                                ),
                            },
                            {
                                id: 'event',
                                header: t('webhooks.eventColumn'),
                                hideable: false,
                                sortValue: (d) => d.event_type,
                                cell: (d) => (
                                    <span className="font-mono text-xs">
                                        {d.event_type}
                                    </span>
                                ),
                            },
                            {
                                id: 'attempts',
                                header: t('webhooks.attemptsColumn'),
                                className: 'text-right',
                                sortValue: (d) => d.attempts,
                                cell: (d) => (
                                    <div className="text-xs text-muted-foreground tabular-nums">
                                        {t('webhooks.attempts', {
                                            count: d.attempts,
                                        })}
                                        {d.next_attempt_at && (
                                            <div>
                                                {t('webhooks.nextTry', {
                                                    time: formatDateTime(
                                                        d.next_attempt_at,
                                                    ),
                                                })}
                                            </div>
                                        )}
                                    </div>
                                ),
                            },
                            {
                                id: 'response',
                                header: t('webhooks.responseColumn'),
                                sortValue: (d) => d.last_status_code,
                                cell: (d) => (
                                    <div className="text-xs whitespace-nowrap text-muted-foreground tabular-nums">
                                        {d.last_status_code != null
                                            ? `HTTP ${d.last_status_code}`
                                            : '—'}
                                        {d.last_duration_ms != null && (
                                            <div>
                                                {d.last_duration_ms.toLocaleString(
                                                    NUMBER_LOCALE,
                                                )}{' '}
                                                ms
                                            </div>
                                        )}
                                    </div>
                                ),
                            },
                            {
                                id: 'when',
                                header: t('webhooks.whenColumn'),
                                sortValue: (d) =>
                                    d.last_attempt_at ?? d.created_at,
                                cell: (d) => (
                                    <span className="text-xs whitespace-nowrap text-muted-foreground">
                                        {formatDateTime(
                                            d.last_attempt_at ?? d.created_at,
                                        )}
                                    </span>
                                ),
                            },
                        ]}
                        rowActions={(d) => (
                            <span className="flex justify-end gap-1">
                                {hasDetails(d) && (
                                    <Button
                                        size="sm"
                                        variant="ghost"
                                        onClick={() => setDetails(d)}
                                    >
                                        {t('webhooks.details')}
                                    </Button>
                                )}
                                <Button
                                    size="sm"
                                    variant="outline"
                                    disabled={
                                        redelivering ||
                                        d.status === 'delivering'
                                    }
                                    onClick={() => redeliver(d)}
                                >
                                    {busyId === d.id ? (
                                        <Spinner />
                                    ) : (
                                        <RotateCw />
                                    )}{' '}
                                    {t('webhooks.redeliver')}
                                </Button>
                            </span>
                        )}
                        rowMenu={(d) => [
                            [
                                ...(hasDetails(d)
                                    ? [
                                          {
                                              id: 'details',
                                              label: t('webhooks.details'),
                                              icon: ScrollText,
                                              run: () => setDetails(d),
                                          },
                                      ]
                                    : []),
                                {
                                    id: 'redeliver',
                                    label: t('webhooks.redeliver'),
                                    icon: RotateCw,
                                    disabled:
                                        redelivering ||
                                        d.status === 'delivering',
                                    run: () => redeliver(d),
                                },
                            ],
                        ]}
                    />
                </div>
                <Dialog
                    open={details !== null}
                    onOpenChange={(o) => !o && setDetails(null)}
                >
                    <DialogContent className="max-h-[90svh] overflow-y-auto">
                        <DialogHeader>
                            <DialogTitle>{t('webhooks.details')}</DialogTitle>
                            <DialogDescription className="font-mono text-xs">
                                {details &&
                                    t('webhooks.event', {
                                        id: details.event_id,
                                    })}
                            </DialogDescription>
                        </DialogHeader>
                        {details?.last_error && (
                            <p className="text-sm text-destructive">
                                {details.last_error}
                            </p>
                        )}
                        {details?.last_response_body && (
                            <pre className="max-h-80 overflow-auto rounded-md bg-muted p-2 font-mono text-xs break-all whitespace-pre-wrap">
                                {details.last_response_body}
                            </pre>
                        )}
                    </DialogContent>
                </Dialog>
            </SheetContent>
        </Sheet>
    );
}
