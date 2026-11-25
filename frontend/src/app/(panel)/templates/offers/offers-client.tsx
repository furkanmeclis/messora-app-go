'use client';

import { useEffect, useMemo, useState, useTransition } from 'react';
import {
    CalendarClock,
    CircleCheck,
    CircleDashed,
    CircleSlash,
    Clock,
    Gift,
    Info,
    Pencil,
    Plus,
    TimerOff,
} from 'lucide-react';
import { notify } from '@/components/notify';
import { useReauth } from '@/components/reauth';
import { RealtimeRefresh, useRealtimeChannel } from '@/components/realtime';
import { ClientDataTable } from '@/components/client-data-table';
import { useFormatDateTime } from '@/components/display-preferences';
import { WhatsAppPreview } from '@/components/whatsapp-preview';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { DateTimePicker } from '@/components/ui/date-picker';
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogHeader,
    DialogTitle,
} from '@/components/ui/dialog';
import {
    Empty,
    EmptyDescription,
    EmptyHeader,
    EmptyMedia,
    EmptyTitle,
} from '@/components/ui/empty';
import { Input } from '@/components/ui/input';
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from '@/components/ui/select';
import { Spinner } from '@/components/ui/spinner';
import { Textarea } from '@/components/ui/textarea';
import { useT } from '@/i18n/client';
import type { Translate } from '@/i18n/translate';
import { remaining, shapeOf } from '@/lib/templates';
import type { ActionResult, Offer, OfferStatus, Template } from '@/lib/types';
import { cn } from '@/lib/utils';
import { ConfirmDelete } from '../../contacts/confirm-delete';
import {
    deleteOfferAction,
    endOfferAction,
    saveOfferAction,
    type OfferForm,
} from './actions';

const STATUS: Record<
    OfferStatus,
    {
        icon: typeof Clock;
        variant: 'secondary' | 'outline' | 'destructive' | 'default';
        tone: string;
    }
> = {
    draft: {
        icon: CircleDashed,
        variant: 'outline',
        tone: 'text-muted-foreground',
    },
    scheduled: {
        icon: CalendarClock,
        variant: 'outline',
        tone: 'text-sky-600',
    },
    active: {
        icon: CircleCheck,
        variant: 'secondary',
        tone: 'text-emerald-600',
    },
    expired: {
        icon: CircleSlash,
        variant: 'outline',
        tone: 'text-muted-foreground',
    },
};

export function OfferStatusBadge({ status }: { status: OfferStatus }) {
    const t = useT();
    const s = STATUS[status] ?? STATUS.draft;
    const Icon = s.icon;
    return (
        <Badge variant={s.variant} data-status={status}>
            <Icon className={s.tone} /> {t(`offers.statuses.${status}`)}
        </Badge>
    );
}

/** "2 days 3 hours" until ms, or "ended". */
export function countdown(t: Translate, endsMs: number, now: number): string {
    const r = remaining(endsMs, now);
    if (r.ended) return t('templates.preview.offerEnded');
    const time =
        r.days > 0
            ? t('templates.preview.daysHours', { days: r.days, hours: r.hours })
            : r.hours > 0
              ? t('templates.preview.hoursMinutes', {
                    hours: r.hours,
                    minutes: r.minutes,
                })
              : t('templates.preview.minutes', {
                    minutes: Math.max(1, r.minutes),
                });
    return t('templates.preview.offerEndsIn', { time });
}

function useNow(every = 30_000): number {
    const [now, setNow] = useState(() => Date.now());
    useEffect(() => {
        const id = setInterval(() => setNow(Date.now()), every);
        return () => clearInterval(id);
    }, [every]);
    return now;
}

/** Keeps the list current: offers announce changes on campaign:all. */
export function OffersLive() {
    useRealtimeChannel('campaign:all');
    return <RealtimeRefresh channel="campaign:all" types={['offer.updated']} />;
}

function localInput(iso: string | undefined): string {
    if (!iso) return '';
    const d = new Date(iso);
    const pad = (n: number) => String(n).padStart(2, '0');
    return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

function emptyForm(): OfferForm {
    const start = new Date();
    start.setMinutes(0, 0, 0);
    start.setHours(start.getHours() + 1);
    const end = new Date(start.getTime() + 7 * 86_400_000);
    return {
        title: '',
        description: '',
        discount_text: '',
        product: '',
        starts_at: start.toISOString(),
        ends_at: end.toISOString(),
        image_url: '',
        cta_url: '',
        coupon_code: '',
        template_id: '',
        publish: true,
    };
}

function formOf(o: Offer): OfferForm {
    return {
        title: o.title,
        description: o.description,
        discount_text: o.discount_text,
        product: o.product,
        starts_at: o.starts_at,
        ends_at: o.ends_at,
        image_url: o.image_url ?? '',
        cta_url: o.cta_url ?? '',
        coupon_code: o.coupon_code ?? '',
        template_id: o.template_id ?? '',
        publish: o.status !== 'draft',
    };
}

const NONE = 'none';

export function OfferDialog({
    offer,
    templates,
    open,
    onOpenChange,
}: {
    offer?: Offer;
    templates: Template[];
    open: boolean;
    onOpenChange: (o: boolean) => void;
}) {
    const t = useT();
    return (
        <Dialog open={open} onOpenChange={onOpenChange}>
            <DialogContent className="max-h-[92svh] overflow-y-auto sm:max-w-4xl">
                <DialogHeader>
                    <DialogTitle>
                        {offer ? t('offers.edit') : t('offers.new')}
                    </DialogTitle>
                    <DialogDescription>
                        {t('offers.dialogHint')}
                    </DialogDescription>
                </DialogHeader>
                {open && (
                    <OfferFormView
                        offer={offer}
                        templates={templates}
                        onDone={() => onOpenChange(false)}
                    />
                )}
            </DialogContent>
        </Dialog>
    );
}

function OfferFormView({
    offer,
    templates,
    onDone,
}: {
    offer?: Offer;
    templates: Template[];
    onDone: () => void;
}) {
    const t = useT();
    const { guard } = useReauth();
    const now = useNow();
    const [f, setF] = useState<OfferForm>(() =>
        offer ? formOf(offer) : emptyForm(),
    );
    const [state, setState] = useState<ActionResult<Offer> | null>(null);
    const [pending, startTransition] = useTransition();
    const set = <K extends keyof OfferForm>(k: K, v: OfferForm[K]) =>
        setF((p) => ({ ...p, [k]: v }));
    const err = (k: string) =>
        state && !state.ok ? state.fieldErrors?.[k] : undefined;
    const marketing = templates.filter(
        (x) => x.category.toUpperCase() === 'MARKETING',
    );
    const tpl = templates.find((x) => x.id === f.template_id);
    const shape = tpl ? shapeOf(tpl) : undefined;
    const ends = new Date(f.ends_at).getTime();
    const sample = useMemo(
        () => ({
            body: Object.fromEntries(
                (shape?.bodyVars ?? []).map((v, i) => [
                    v,
                    i === 0 ? f.title || '…' : f.discount_text || '…',
                ]),
            ),
        }),
        [shape, f.title, f.discount_text],
    );

    const submit = (e: React.FormEvent) => {
        e.preventDefault();
        startTransition(async () => {
            const r = await guard(() => saveOfferAction(offer?.id ?? null, f));
            setState(r);
            if (notify(r)) onDone();
        });
    };
    const field = (
        k: keyof OfferForm,
        label: string,
        props: React.ComponentProps<typeof Input> = {},
    ) => (
        <label className="block space-y-1 text-sm">
            <span className="font-medium">{label}</span>
            <Input
                id={`offer-${k}`}
                value={String(f[k])}
                aria-invalid={err(k) ? true : undefined}
                onChange={(e) => set(k, e.target.value as never)}
                {...props}
            />
            {err(k) && (
                <span role="alert" className="block text-destructive">
                    {err(k)}
                </span>
            )}
        </label>
    );
    const dateField = (k: 'starts_at' | 'ends_at', label: string) => (
        <label className="block space-y-1 text-sm">
            <span className="font-medium">{label}</span>
            <DateTimePicker
                id={`offer-${k}`}
                value={localInput(f[k])}
                onValueChange={(v) => {
                    const d = new Date(v);
                    if (Number.isFinite(d.getTime())) set(k, d.toISOString());
                }}
            />
            {err(k) && (
                <span role="alert" className="block text-destructive">
                    {err(k)}
                </span>
            )}
        </label>
    );

    return (
        <form
            onSubmit={submit}
            className="grid gap-6 md:grid-cols-[1fr_18rem]"
            data-testid="offer-form"
        >
            <div className="space-y-4">
                {state && !state.ok && !state.fieldErrors?.title && (
                    <Alert variant="destructive">
                        <AlertDescription>{state.message}</AlertDescription>
                    </Alert>
                )}
                {field('title', t('offers.fields.title'), {
                    required: true,
                    maxLength: 120,
                    autoFocus: true,
                })}
                <div className="grid gap-4 sm:grid-cols-2">
                    {field('discount_text', t('offers.fields.discount'), {
                        maxLength: 60,
                        placeholder: t('offers.fields.discountPlaceholder'),
                    })}
                    {field('coupon_code', t('offers.fields.coupon'), {
                        maxLength: 20,
                        className: 'font-mono',
                    })}
                    {dateField('starts_at', t('offers.fields.starts'))}
                    {dateField('ends_at', t('offers.fields.ends'))}
                </div>
                <p className="flex items-center gap-2 text-sm text-muted-foreground">
                    <Clock className="size-4" />
                    {Number.isFinite(ends) ? countdown(t, ends, now) : ''}
                </p>
                {field('product', t('offers.fields.product'), {
                    maxLength: 200,
                })}
                <label className="block space-y-1 text-sm">
                    <span className="font-medium">
                        {t('offers.fields.description')}
                    </span>
                    <Textarea
                        id="offer-description"
                        rows={2}
                        maxLength={2000}
                        value={f.description}
                        onChange={(e) => set('description', e.target.value)}
                    />
                </label>
                <div className="grid gap-4 sm:grid-cols-2">
                    {field('image_url', t('offers.fields.image'), {
                        inputMode: 'url',
                        placeholder: 'https://…',
                    })}
                    {field('cta_url', t('offers.fields.cta'), {
                        inputMode: 'url',
                        placeholder: 'https://…',
                    })}
                </div>
                <label className="block space-y-1 text-sm">
                    <span className="font-medium">
                        {t('offers.fields.template')}
                    </span>
                    <Select
                        value={f.template_id || NONE}
                        onValueChange={(v) =>
                            set('template_id', v === NONE ? '' : v)
                        }
                    >
                        <SelectTrigger
                            id="offer-template"
                            className="w-full"
                            aria-label={t('offers.fields.template')}
                        >
                            <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                            <SelectItem value={NONE}>
                                {t('offers.noTemplate')}
                            </SelectItem>
                            {marketing.map((x) => (
                                <SelectItem key={x.id} value={x.id}>
                                    {x.name} ({x.language})
                                    {shapeOf(x).lto
                                        ? ` · ${t('offers.ltoTag')}`
                                        : ''}
                                </SelectItem>
                            ))}
                        </SelectContent>
                    </Select>
                </label>
                <label className="flex items-center gap-2 text-sm">
                    <Checkbox
                        checked={f.publish}
                        onCheckedChange={(v) => set('publish', v === true)}
                    />
                    {t('offers.publish')}
                </label>
                <Button type="submit" disabled={pending} className="w-full">
                    {pending && <Spinner />}
                    {offer ? t('common.save') : t('offers.create')}
                </Button>
            </div>
            <aside className="space-y-2">
                <p className="text-sm font-medium">{t('offers.preview')}</p>
                {tpl ? (
                    <WhatsAppPreview
                        components={tpl.components}
                        literal={sample}
                        mediaSrc={f.image_url || shape?.exampleMedia}
                        expiresAt={Number.isFinite(ends) ? ends : undefined}
                        coupon={f.coupon_code || undefined}
                    />
                ) : (
                    <OfferCard form={f} ends={ends} now={now} />
                )}
                <Alert>
                    <Info />
                    <AlertDescription>
                        {shape?.lto
                            ? t('offers.ltoNote')
                            : t('offers.fallbackNote')}
                    </AlertDescription>
                </Alert>
            </aside>
        </form>
    );
}

/** A neutral card for offers without a template. */
function OfferCard({
    form,
    ends,
    now,
}: {
    form: OfferForm;
    ends: number;
    now: number;
}) {
    const t = useT();
    return (
        <div className="space-y-2 rounded-xl border p-3">
            <p className="flex items-center gap-2 font-medium">
                <Gift className="size-4 text-muted-foreground" />
                {form.title || t('offers.fields.title')}
            </p>
            {form.discount_text && (
                <p className="text-lg font-semibold">{form.discount_text}</p>
            )}
            <p className="text-sm text-muted-foreground">
                {Number.isFinite(ends) ? countdown(t, ends, now) : ''}
            </p>
            {form.coupon_code && (
                <p className="w-fit rounded border border-dashed px-2 py-0.5 font-mono text-sm">
                    {form.coupon_code}
                </p>
            )}
        </div>
    );
}

export function NewOfferButton({ templates }: { templates: Template[] }) {
    const t = useT();
    const [open, setOpen] = useState(false);
    return (
        <>
            <Button shortcut="n" onClick={() => setOpen(true)}>
                <Plus /> {t('offers.new')}
            </Button>
            <OfferDialog
                templates={templates}
                open={open}
                onOpenChange={setOpen}
            />
        </>
    );
}

export function OffersTable({
    offers,
    templates,
    manage,
}: {
    offers: Offer[];
    templates: Template[];
    manage: boolean;
}) {
    const t = useT();
    const formatDateTime = useFormatDateTime();
    const now = useNow();
    const { guard } = useReauth();
    const [editing, setEditing] = useState<Offer | null>(null);
    const [pending, startTransition] = useTransition();
    const names = new Map(templates.map((x) => [x.id, x]));
    if (offers.length === 0) {
        return (
            <Empty className="py-12">
                <EmptyHeader>
                    <EmptyMedia variant="icon">
                        <Gift />
                    </EmptyMedia>
                    <EmptyTitle>{t('offers.empty')}</EmptyTitle>
                    <EmptyDescription>{t('offers.emptyHint')}</EmptyDescription>
                </EmptyHeader>
            </Empty>
        );
    }
    return (
        <>
            <ClientDataTable
                rows={offers}
                rowId={(o) => o.id}
                storageKey="offers"
                initialSort="-ends"
                rowTestId="offer"
                searchText={(o) =>
                    `${o.title} ${o.discount_text} ${o.coupon_code ?? ''} ${o.product}`
                }
                searchPlaceholder={t('table.search')}
                noMatch={
                    <p className="p-8 text-center text-sm text-muted-foreground">
                        {t('table.noMatch')}
                    </p>
                }
                columns={[
                    {
                        id: 'title',
                        header: t('offers.fields.title'),
                        hideable: false,
                        sortValue: (o) => o.title,
                        cell: (o) => (
                            <div className="min-w-0">
                                <p className="font-medium">{o.title}</p>
                                {(o.discount_text || o.product) && (
                                    <p className="max-w-xs truncate text-xs text-muted-foreground">
                                        {[o.discount_text, o.product]
                                            .filter(Boolean)
                                            .join(' · ')}
                                    </p>
                                )}
                            </div>
                        ),
                    },
                    {
                        id: 'status',
                        header: t('offers.status'),
                        sortValue: (o) => o.status,
                        cell: (o) => <OfferStatusBadge status={o.status} />,
                    },
                    {
                        id: 'starts',
                        header: t('offers.fields.starts'),
                        sortValue: (o) => o.starts_at,
                        cell: (o) => (
                            <span className="text-sm whitespace-nowrap text-muted-foreground">
                                {formatDateTime(o.starts_at)}
                            </span>
                        ),
                    },
                    {
                        id: 'ends',
                        header: t('offers.fields.ends'),
                        sortValue: (o) => o.ends_at,
                        cell: (o) => (
                            <div className="text-sm whitespace-nowrap">
                                <p>{formatDateTime(o.ends_at)}</p>
                                {o.status !== 'expired' &&
                                    o.status !== 'draft' && (
                                        <p
                                            className={cn(
                                                'text-xs text-muted-foreground',
                                                new Date(o.ends_at).getTime() -
                                                    now <
                                                    3_600_000 &&
                                                    'text-destructive',
                                            )}
                                        >
                                            {countdown(
                                                t,
                                                new Date(o.ends_at).getTime(),
                                                now,
                                            )}
                                        </p>
                                    )}
                            </div>
                        ),
                    },
                    {
                        id: 'coupon',
                        header: t('offers.fields.coupon'),
                        sortValue: (o) => o.coupon_code ?? '',
                        cell: (o) =>
                            o.coupon_code ? (
                                <code className="text-sm">{o.coupon_code}</code>
                            ) : (
                                <span className="text-muted-foreground">—</span>
                            ),
                    },
                    {
                        id: 'template',
                        header: t('offers.fields.template'),
                        defaultHidden: false,
                        cell: (o) => {
                            const x = o.template_id
                                ? names.get(o.template_id)
                                : undefined;
                            if (!x)
                                return (
                                    <span className="text-muted-foreground">
                                        —
                                    </span>
                                );
                            return (
                                <span className="font-mono text-xs">
                                    {x.name}
                                    {shapeOf(x).lto && (
                                        <Badge
                                            variant="secondary"
                                            className="ml-1"
                                        >
                                            {t('offers.ltoTag')}
                                        </Badge>
                                    )}
                                </span>
                            );
                        },
                    },
                ]}
                rowActions={
                    manage
                        ? (o) => (
                              <span className="whitespace-nowrap">
                                  {o.status !== 'expired' && (
                                      <>
                                          <Button
                                              size="icon-sm"
                                              variant="ghost"
                                              aria-label={t(
                                                  'offers.editLabel',
                                                  {
                                                      title: o.title,
                                                  },
                                              )}
                                              onClick={() => setEditing(o)}
                                          >
                                              <Pencil />
                                          </Button>
                                          <Button
                                              size="icon-sm"
                                              variant="ghost"
                                              disabled={pending}
                                              aria-label={t('offers.endLabel', {
                                                  title: o.title,
                                              })}
                                              onClick={() =>
                                                  startTransition(async () => {
                                                      notify(
                                                          await guard(() =>
                                                              endOfferAction(
                                                                  o.id,
                                                              ),
                                                          ),
                                                      );
                                                  })
                                              }
                                          >
                                              <TimerOff />
                                          </Button>
                                      </>
                                  )}
                                  <ConfirmDelete
                                      label={t('offers.deleteLabel', {
                                          title: o.title,
                                      })}
                                      title={t('offers.deleteTitle', {
                                          title: o.title,
                                      })}
                                      description={t('offers.deleteBody')}
                                      confirm={t('common.delete')}
                                      action={() => deleteOfferAction(o.id)}
                                  />
                              </span>
                          )
                        : undefined
                }
                onRowClick={
                    manage
                        ? (o) => o.status !== 'expired' && setEditing(o)
                        : undefined
                }
            />
            <OfferDialog
                offer={editing ?? undefined}
                templates={templates}
                open={!!editing}
                onOpenChange={(o) => !o && setEditing(null)}
            />
        </>
    );
}
