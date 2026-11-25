'use client';

import { useEffect, useMemo, useState } from 'react';
import {
    AlertTriangle,
    CheckCircle2,
    Clock,
    FileText,
    GalleryHorizontal,
    Image as ImageIcon,
    Link2,
    MapPin,
    Play,
    Ticket,
    Type,
    Upload,
    UserRound,
    Workflow,
    X,
} from 'lucide-react';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import { Combobox, type ComboboxOption } from '@/components/ui/combobox';
import { DateTimePicker } from '@/components/ui/date-picker';
import { FileDrop } from '@/components/ui/file-drop';
import { Input } from '@/components/ui/input';
import { Segmented } from '@/components/ui/segmented';
import { Spinner } from '@/components/ui/spinner';
import { Textarea } from '@/components/ui/textarea';
import { useT } from '@/i18n/client';
import type { Translate } from '@/i18n/translate';
import {
    CONTACT_FIELDS,
    fieldOf,
    fieldToken,
    flowData,
    isMediaHeader,
    issuesOf,
    placeholdersIn,
    type CardValues,
    type HeaderMediaValue,
    type MediaHeaderKind,
    type OfferFills,
    type ParamValues,
    type TemplateShape,
} from '@/lib/templates';
import { cn } from '@/lib/utils';

const ACCEPT: Record<MediaHeaderKind, string> = {
    image: 'image/jpeg,image/png',
    video: 'video/mp4,video/3gpp',
    document:
        'application/pdf,application/msword,application/vnd.openxmlformats-officedocument.wordprocessingml.document,application/vnd.ms-excel,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet,application/vnd.ms-powerpoint,application/vnd.openxmlformats-officedocument.presentationml.presentation,text/plain',
};

const MEDIA_ICON = { image: ImageIcon, video: Play, document: FileText };

/** A value the editor fills: where it lives in ParamValues and its label. */
type Slot = {
    key: string;
    label: string;
    get: (v: ParamValues) => string;
    set: (v: ParamValues, x: string) => ParamValues;
    /** Contact fields make no sense for some values (a code typed once). */
    fieldMapping?: boolean;
    maxLength?: number;
};

function card(v: ParamValues, k: number): CardValues {
    return v.cards[k] ?? { body: {}, buttons: {} };
}

function withCard(v: ParamValues, k: number, c: CardValues): ParamValues {
    const cards = [...v.cards];
    while (cards.length <= k) cards.push({ body: {}, buttons: {} });
    cards[k] = c;
    return { ...v, cards };
}

/** The label of an issue slot ("body:1", "card:0:button:1", "coupon" …). */
export function slotLabel(t: Translate, slot: string): string {
    const parts = slot.split(':');
    switch (parts[0]) {
        case 'header':
            return t('params.header', { name: parts[1]! });
        case 'body':
            return t('params.body', { name: parts[1]! });
        case 'button':
            return t('params.button', { n: Number(parts[1]) + 1 });
        case 'coupon':
            return t('params.coupon');
        case 'expiration':
            return t('params.expiration');
        case 'otp':
            return t('params.otp');
        case 'card': {
            const n = Number(parts[1]) + 1;
            const inner =
                parts[2] === 'body'
                    ? t('params.body', { name: parts[3]! })
                    : t('params.cardButton', { n: Number(parts[3]) + 1 });
            return `${t('params.card', { n })} · ${inner}`;
        }
    }
    return slot;
}

function slotsOf(t: Translate, shape: TemplateShape): Slot[] {
    if (shape.otp.length > 0) {
        return [
            {
                key: 'otp',
                label: t('params.otp'),
                get: (v) => v.otp,
                set: (v, x) => ({ ...v, otp: x }),
                maxLength: 1024,
            },
        ];
    }
    const out: Slot[] = [
        ...shape.headerVars.map((name) => ({
            key: `header:${name}`,
            label: t('params.header', { name }),
            get: (v: ParamValues) => v.header[name] ?? '',
            set: (v: ParamValues, x: string) => ({
                ...v,
                header: { ...v.header, [name]: x },
            }),
        })),
        ...shape.bodyVars.map((name) => ({
            key: `body:${name}`,
            label: t('params.body', { name }),
            get: (v: ParamValues) => v.body[name] ?? '',
            set: (v: ParamValues, x: string) => ({
                ...v,
                body: { ...v.body, [name]: x },
            }),
        })),
        ...shape.urlButtons.map((i) => ({
            key: `button:${i}`,
            label: t('params.button', { n: i + 1 }),
            get: (v: ParamValues) => v.buttons[i] ?? '',
            set: (v: ParamValues, x: string) => ({
                ...v,
                buttons: { ...v.buttons, [i]: x },
            }),
        })),
    ];
    if (shape.copyCode.length > 0) {
        out.push({
            key: 'coupon',
            label: t('params.coupon'),
            get: (v) => v.coupon,
            set: (v, x) => ({ ...v, coupon: x }),
            maxLength: shape.lto ? 15 : 20,
        });
    }
    return out;
}

function cardSlots(t: Translate, shape: TemplateShape, k: number): Slot[] {
    const cs = shape.cards[k]!;
    return [
        ...cs.bodyVars.map((name) => ({
            key: `card:${k}:body:${name}`,
            label: t('params.body', { name }),
            get: (v: ParamValues) => card(v, k).body[name] ?? '',
            set: (v: ParamValues, x: string) =>
                withCard(v, k, {
                    ...card(v, k),
                    body: { ...card(v, k).body, [name]: x },
                }),
        })),
        ...[...cs.urlButtons, ...cs.quickReplies]
            .sort((a, b) => a - b)
            .map((i) => ({
                key: `card:${k}:button:${i}`,
                label: cs.quickReplies.includes(i)
                    ? t('params.cardPayload', { n: i + 1 })
                    : t('params.cardButton', { n: i + 1 }),
                get: (v: ParamValues) => card(v, k).buttons[i] ?? '',
                set: (v: ParamValues, x: string) =>
                    withCard(v, k, {
                        ...card(v, k),
                        buttons: { ...card(v, k).buttons, [i]: x },
                    }),
            })),
    ];
}

/**
 * Fills a template's variables, header media and the special components
 * (location, copy code, offer end, carousel cards, one-time code, Flow):
 * each variable is a contact field (with a value for contacts that lack
 * it) or fixed text. Used by campaigns, the inbox and automations.
 */
export function TemplateParamsEditor({
    shape,
    value,
    onChange,
    attributeKeys = [],
    phoneNumberId,
    fieldMapping = true,
    disabled,
    fills = {},
}: {
    shape: TemplateShape;
    value: ParamValues;
    onChange: (v: ParamValues) => void;
    attributeKeys?: string[];
    /** Sender whose account stores uploads; uploads are off without it. */
    phoneNumberId?: string;
    /** Inbox replies to one person may prefer fixed text only. */
    fieldMapping?: boolean;
    disabled?: boolean;
    /** What a linked offer fills in (campaigns). */
    fills?: OfferFills;
}) {
    const t = useT();
    const slots = slotsOf(t, shape);
    const cardSlotList = shape.cards.flatMap((_, k) => cardSlots(t, shape, k));
    const fieldLabel = (f: string) =>
        f.startsWith('attr:')
            ? t('params.attribute', { key: f.slice(5) })
            : f.startsWith('offer_')
              ? t('params.offerField', { field: f.slice(6) })
              : t(`params.fields.${f as 'name'}`);
    const options: ComboboxOption[] = useMemo(
        () => [
            ...CONTACT_FIELDS.map((f) => ({
                value: f,
                label: fieldLabel(f),
                keywords: f,
            })),
            ...attributeKeys.map((k) => ({
                value: `attr:${k}`,
                label: fieldLabel(`attr:${k}`),
                keywords: k,
            })),
        ],
        // eslint-disable-next-line react-hooks/exhaustive-deps -- labels depend on the locale only
        [attributeKeys],
    );

    const used = [
        ...new Set(
            [...slots, ...cardSlotList]
                .flatMap((s) => placeholdersIn(s.get(value)))
                .filter((f) => !f.startsWith('offer_')),
        ),
    ];
    const hasSpecial =
        shape.header === 'location' ||
        shape.lto ||
        shape.cards.length > 0 ||
        shape.flow.length > 0;
    const offerImage = fills.media && shape.header === 'image';

    return (
        <div className="space-y-5">
            {isMediaHeader(shape.header) &&
                (offerImage && !value.media ? (
                    <FillNote
                        icon={<ImageIcon />}
                        text={t('params.offer.image')}
                    />
                ) : (
                    <MediaPicker
                        kind={shape.header}
                        media={value.media}
                        onMedia={(media, preview) =>
                            onChange({
                                ...value,
                                media,
                                mediaPreview: preview,
                            })
                        }
                        phoneNumberId={phoneNumberId}
                        disabled={disabled}
                        testId="param-media"
                    />
                ))}
            {shape.header === 'location' && (
                <LocationEditor
                    value={value}
                    onChange={onChange}
                    disabled={disabled}
                />
            )}
            {slots.length === 0 &&
                !isMediaHeader(shape.header) &&
                !hasSpecial && (
                    <p className="text-sm text-muted-foreground">
                        {t('params.noVariables')}
                    </p>
                )}
            {slots.map((s) =>
                s.key === 'coupon' && fills.coupon && !value.coupon ? (
                    <FillNote
                        key={s.key}
                        icon={<Ticket />}
                        text={t('params.offer.coupon')}
                    />
                ) : (
                    <SlotRow
                        key={s.key}
                        label={s.label}
                        example={shape.examples[s.key]}
                        value={s.get(value)}
                        onChange={(x) => onChange(s.set(value, x))}
                        options={options}
                        fieldMapping={fieldMapping && s.fieldMapping !== false}
                        disabled={disabled}
                        maxLength={s.maxLength}
                        testId={`param-${s.key}`}
                    />
                ),
            )}
            {shape.lto &&
                (fills.expiration ? (
                    <FillNote icon={<Clock />} text={t('params.offer.ends')} />
                ) : (
                    <ExpirationEditor
                        value={value}
                        onChange={onChange}
                        disabled={disabled}
                    />
                ))}
            {shape.cards.map((cs, k) => (
                <fieldset
                    key={k}
                    className="space-y-3 rounded-lg border p-3"
                    data-testid={`param-card-${k}`}
                >
                    <legend className="flex items-center gap-2 px-1 text-sm font-medium">
                        <GalleryHorizontal className="size-4 text-muted-foreground" />
                        {t('params.card', { n: k + 1 })}
                    </legend>
                    <MediaPicker
                        kind={cs.header}
                        media={card(value, k).media}
                        onMedia={(media, preview) =>
                            onChange(
                                withCard(value, k, {
                                    ...card(value, k),
                                    media,
                                    mediaPreview: preview,
                                }),
                            )
                        }
                        phoneNumberId={phoneNumberId}
                        disabled={disabled}
                        testId={`param-card-${k}-media`}
                    />
                    {cardSlots(t, shape, k).map((s) => (
                        <SlotRow
                            key={s.key}
                            label={s.label}
                            example={shape.examples[s.key]}
                            value={s.get(value)}
                            onChange={(x) => onChange(s.set(value, x))}
                            options={options}
                            fieldMapping={fieldMapping}
                            disabled={disabled}
                            testId={`param-${s.key}`}
                        />
                    ))}
                </fieldset>
            ))}
            {shape.flow.length > 0 && (
                <FlowEditor
                    value={value}
                    onChange={onChange}
                    disabled={disabled}
                />
            )}
            {used.length > 0 && (
                <div className="space-y-2 rounded-lg border bg-muted/20 p-3">
                    {used.map((f) => (
                        <div
                            key={f}
                            className="grid items-center gap-2 sm:grid-cols-[10rem_1fr]"
                        >
                            <label
                                htmlFor={`fb-${f}`}
                                className="text-sm text-muted-foreground"
                            >
                                {t('params.fallback')}:{' '}
                                <span className="font-medium text-foreground">
                                    {fieldLabel(f)}
                                </span>
                            </label>
                            <Input
                                id={`fb-${f}`}
                                value={value.fallback[f] ?? ''}
                                placeholder={t('params.fallbackPlaceholder')}
                                disabled={disabled}
                                onChange={(e) =>
                                    onChange({
                                        ...value,
                                        fallback: {
                                            ...value.fallback,
                                            [f]: e.target.value,
                                        },
                                    })
                                }
                            />
                        </div>
                    ))}
                </div>
            )}
        </div>
    );
}

function FillNote({ icon, text }: { icon: React.ReactNode; text: string }) {
    return (
        <p className="flex items-center gap-2 rounded-lg border border-dashed px-3 py-2 text-sm text-muted-foreground [&>svg]:size-4 [&>svg]:shrink-0">
            {icon}
            {text}
        </p>
    );
}

function SlotRow({
    label,
    example,
    value,
    onChange,
    options,
    fieldMapping,
    disabled,
    maxLength = 1024,
    testId,
}: {
    label: string;
    example?: string;
    value: string;
    onChange: (v: string) => void;
    options: ComboboxOption[];
    fieldMapping: boolean;
    disabled?: boolean;
    maxLength?: number;
    testId: string;
}) {
    const t = useT();
    const mapped = fieldOf(value);
    const [mode, setMode] = useState<'field' | 'text'>(
        fieldMapping && (mapped !== undefined || value === '')
            ? 'field'
            : 'text',
    );
    return (
        <div className="@container space-y-2" data-testid={testId}>
            <div className="flex flex-col items-start gap-2 @md:flex-row @md:items-center @md:justify-between">
                <div>
                    <p className="text-sm font-medium">{label}</p>
                    {example && (
                        <p className="text-xs text-muted-foreground">
                            {t('params.example', { value: example })}
                        </p>
                    )}
                </div>
                {fieldMapping && (
                    <Segmented
                        value={mode}
                        disabled={disabled}
                        onValueChange={(m) => {
                            setMode(m);
                            // Switching keeps nothing that would be wrong in the new mode.
                            if (m === 'field' && mapped === undefined)
                                onChange('');
                            if (m === 'text' && mapped !== undefined)
                                onChange('');
                        }}
                        options={[
                            {
                                value: 'field',
                                label: t('params.modes.field'),
                                icon: <UserRound />,
                            },
                            {
                                value: 'text',
                                label: t('params.modes.text'),
                                icon: <Type />,
                            },
                        ]}
                    />
                )}
            </div>
            {mode === 'field' && fieldMapping ? (
                <Combobox
                    aria-label={label}
                    options={options}
                    value={mapped ?? ''}
                    onValueChange={(f) => onChange(f ? fieldToken(f) : '')}
                    placeholder={t('params.chooseField')}
                    searchPlaceholder={t('params.searchField')}
                    emptyText={t('params.noField')}
                    disabled={disabled}
                />
            ) : (
                <Input
                    aria-label={label}
                    value={value}
                    placeholder={t('params.textPlaceholder')}
                    disabled={disabled}
                    maxLength={maxLength}
                    onChange={(e) => onChange(e.target.value)}
                />
            )}
        </div>
    );
}

function LocationEditor({
    value,
    onChange,
    disabled,
}: {
    value: ParamValues;
    onChange: (v: ParamValues) => void;
    disabled?: boolean;
}) {
    const t = useT();
    const l = value.location ?? {
        latitude: '',
        longitude: '',
        name: '',
        address: '',
    };
    const set = (k: keyof typeof l, x: string) =>
        onChange({ ...value, location: { ...l, [k]: x } });
    const field = (k: keyof typeof l, mode?: 'decimal') => (
        <label className="space-y-1 text-sm">
            <span className="text-muted-foreground">
                {t(`params.location.${k}`)}
            </span>
            <Input
                value={l[k]}
                inputMode={mode}
                disabled={disabled}
                maxLength={k === 'latitude' || k === 'longitude' ? 32 : 1024}
                onChange={(e) => set(k, e.target.value)}
            />
        </label>
    );
    return (
        <fieldset
            className="space-y-3 rounded-lg border p-3"
            data-testid="param-location"
        >
            <legend className="flex items-center gap-2 px-1 text-sm font-medium">
                <MapPin className="size-4 text-muted-foreground" />
                {t('params.location.title')}
            </legend>
            <div className="grid gap-3 sm:grid-cols-2">
                {field('latitude', 'decimal')}
                {field('longitude', 'decimal')}
                {field('name')}
                {field('address')}
            </div>
            <p className="text-xs text-muted-foreground">
                {t('params.location.hint')}
            </p>
        </fieldset>
    );
}

/** datetime-local value of a Unix ms time, in the browser's zone. */
function localInput(ms: number | undefined): string {
    if (!ms) return '';
    const d = new Date(ms);
    const pad = (n: number) => String(n).padStart(2, '0');
    return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

function ExpirationEditor({
    value,
    onChange,
    disabled,
}: {
    value: ParamValues;
    onChange: (v: ParamValues) => void;
    disabled?: boolean;
}) {
    const t = useT();
    return (
        <label
            className="block space-y-1 rounded-lg border p-3 text-sm"
            data-testid="param-expiration"
        >
            <span className="flex items-center gap-2 font-medium">
                <Clock className="size-4 text-muted-foreground" />
                {t('params.expiration')}
            </span>
            <DateTimePicker
                value={localInput(value.expirationMs)}
                disabled={disabled}
                onValueChange={(v) => {
                    const ms = v ? new Date(v).getTime() : undefined;
                    onChange({
                        ...value,
                        expirationMs:
                            ms && Number.isFinite(ms) ? ms : undefined,
                    });
                }}
            />
            <span className="block text-xs text-muted-foreground">
                {t('params.expirationHint')}
            </span>
        </label>
    );
}

function FlowEditor({
    value,
    onChange,
    disabled,
}: {
    value: ParamValues;
    onChange: (v: ParamValues) => void;
    disabled?: boolean;
}) {
    const t = useT();
    const bad = value.flowData.trim() !== '' && !flowData(value.flowData);
    return (
        <fieldset
            className="space-y-3 rounded-lg border p-3"
            data-testid="param-flow"
        >
            <legend className="flex items-center gap-2 px-1 text-sm font-medium">
                <Workflow className="size-4 text-muted-foreground" />
                {t('params.flow.title')}
            </legend>
            <label className="block space-y-1 text-sm">
                <span className="text-muted-foreground">
                    {t('params.flow.token')}
                </span>
                <Input
                    value={value.flowToken}
                    placeholder="unused"
                    maxLength={1024}
                    disabled={disabled}
                    onChange={(e) =>
                        onChange({ ...value, flowToken: e.target.value })
                    }
                />
            </label>
            <label className="block space-y-1 text-sm">
                <span className="text-muted-foreground">
                    {t('params.flow.data')}
                </span>
                <Textarea
                    value={value.flowData}
                    rows={3}
                    className="font-mono text-xs"
                    placeholder='{"product_id": "42"}'
                    aria-invalid={bad || undefined}
                    disabled={disabled}
                    onChange={(e) =>
                        onChange({ ...value, flowData: e.target.value })
                    }
                />
                {bad && (
                    <span className="block text-xs text-destructive">
                        {t('params.flow.invalid')}
                    </span>
                )}
            </label>
        </fieldset>
    );
}

function MediaPicker({
    kind,
    media: current,
    onMedia,
    phoneNumberId,
    disabled,
    testId,
}: {
    kind: MediaHeaderKind;
    media?: HeaderMediaValue;
    onMedia: (media?: HeaderMediaValue, preview?: string) => void;
    phoneNumberId?: string;
    disabled?: boolean;
    testId: string;
}) {
    const t = useT();
    const media = current?.type === kind ? current : undefined;
    const [mode, setMode] = useState<'upload' | 'link'>(
        media?.link ? 'link' : 'upload',
    );
    const [link, setLink] = useState(media?.link ?? '');
    const [busy, setBusy] = useState(false);
    const [error, setError] = useState<string | null>(null);
    const [preview, setPreview] = useState<string | undefined>();
    const Icon = MEDIA_ICON[kind];
    const kindLabel = t(`params.media.${kind}`);

    // Free the object URL of a replaced upload.
    useEffect(() => {
        return () => {
            if (preview?.startsWith('blob:')) URL.revokeObjectURL(preview);
        };
    }, [preview]);

    const upload = async (file: File | null) => {
        if (!file) return;
        if (!phoneNumberId) {
            setError(t('params.media.needSender'));
            return;
        }
        setBusy(true);
        setError(null);
        try {
            const q = new URLSearchParams({
                filename: file.name,
                phone_number_id: phoneNumberId,
            });
            const res = await fetch(`/bff/media-upload?${q}`, {
                method: 'POST',
                body: file,
                headers: {
                    'Content-Type': file.type || 'application/octet-stream',
                },
            });
            const body = (await res.json().catch(() => null)) as {
                data?: { id: string; filename: string };
                error?: { message?: string };
            } | null;
            if (!res.ok || !body?.data) {
                setError(
                    body?.error?.message ?? t('common.somethingWentWrong'),
                );
                return;
            }
            const src =
                kind === 'document' ? undefined : URL.createObjectURL(file);
            setPreview(src);
            onMedia(
                {
                    type: kind,
                    id: body.data.id,
                    filename:
                        kind === 'document' ? body.data.filename : undefined,
                },
                src,
            );
        } finally {
            setBusy(false);
        }
    };

    const applyLink = (v: string) => {
        setLink(v);
        const ok = /^https:\/\/\S+$/i.test(v.trim());
        setError(v && !ok ? t('params.media.httpsOnly') : null);
        onMedia(
            ok
                ? {
                      type: kind,
                      link: v.trim(),
                      filename: kind === 'document' ? fileName(v) : undefined,
                  }
                : undefined,
            ok && kind !== 'document' ? v.trim() : undefined,
        );
    };

    return (
        <div className="space-y-3 rounded-lg border p-3" data-testid={testId}>
            <div className="flex flex-col items-start gap-2 @md:flex-row @md:items-center @md:justify-between">
                <p className="flex items-center gap-2 text-sm font-medium">
                    <Icon className="size-4 text-muted-foreground" />
                    {t('params.media.title', { kind: kindLabel.toLowerCase() })}
                </p>
                <Segmented
                    value={mode}
                    disabled={disabled}
                    onValueChange={setMode}
                    options={[
                        {
                            value: 'upload',
                            label: t('params.media.upload'),
                            icon: <Upload />,
                        },
                        {
                            value: 'link',
                            label: t('params.media.link'),
                            icon: <Link2 />,
                        },
                    ]}
                />
            </div>
            {media && (media.id || media.link) && (
                <div className="flex items-center gap-2 rounded-md bg-emerald-500/10 px-2.5 py-1.5 text-sm text-emerald-800 dark:text-emerald-300">
                    <CheckCircle2 className="size-4 shrink-0" />
                    <span className="min-w-0 flex-1 truncate">
                        {media.link ??
                            media.filename ??
                            t('params.media.uploaded')}
                    </span>
                    <Button
                        type="button"
                        size="sm"
                        variant="ghost"
                        disabled={disabled}
                        onClick={() => {
                            setLink('');
                            onMedia(undefined, undefined);
                        }}
                    >
                        <X /> {t('params.media.remove')}
                    </Button>
                </div>
            )}
            {mode === 'upload' ? (
                <div className="space-y-1">
                    <FileDrop
                        compact
                        accept={ACCEPT[kind]}
                        disabled={disabled || busy || !phoneNumberId}
                        onFile={upload}
                        hint={
                            busy ? (
                                <>
                                    <Spinner /> {t('params.media.uploading')}
                                </>
                            ) : (
                                t(`params.media.accept.${kind}`)
                            )
                        }
                    />
                    <p className="text-xs text-muted-foreground">
                        {phoneNumberId
                            ? t('params.media.uploadHint')
                            : t('params.media.needSender')}
                    </p>
                </div>
            ) : (
                <div className="space-y-1">
                    <Input
                        type="url"
                        inputMode="url"
                        aria-label={t('params.media.link')}
                        value={link}
                        placeholder={t('params.media.linkPlaceholder')}
                        disabled={disabled}
                        onChange={(e) => applyLink(e.target.value)}
                    />
                    <p className="text-xs text-muted-foreground">
                        {t('params.media.linkHint')}
                    </p>
                </div>
            )}
            {error && <p className="text-sm text-destructive">{error}</p>}
        </div>
    );
}

function fileName(url: string): string | undefined {
    try {
        const last = new URL(url).pathname.split('/').pop();
        return last ? decodeURIComponent(last) : undefined;
    } catch {
        return undefined;
    }
}

/** What still stops sending, as a short list. */
export function ParamIssues({
    shape,
    value,
    fills,
    className,
}: {
    shape: TemplateShape;
    value: ParamValues;
    fills?: OfferFills;
    className?: string;
}) {
    const t = useT();
    const issues = issuesOf(value, shape, fills);
    if (issues.length === 0) return null;
    return (
        <Alert className={cn(className)}>
            <AlertTriangle />
            <AlertTitle>{t('params.issues.title')}</AlertTitle>
            <AlertDescription>
                <ul className="list-disc pl-4">
                    {issues.map((i, n) => (
                        <li key={n}>
                            {i.kind === 'media'
                                ? t('params.issues.media', {
                                      kind: t(
                                          `params.media.${shape.header as MediaHeaderKind}`,
                                      ).toLowerCase(),
                                  })
                                : i.kind === 'location'
                                  ? t('params.issues.location')
                                  : i.kind === 'cardMedia'
                                    ? t('params.issues.cardMedia', {
                                          n: i.card + 1,
                                      })
                                    : i.kind === 'flowData'
                                      ? t('params.flow.invalid')
                                      : t('params.issues.value', {
                                            slot: slotLabel(t, i.slot),
                                        })}
                        </li>
                    ))}
                </ul>
            </AlertDescription>
        </Alert>
    );
}
