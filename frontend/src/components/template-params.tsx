'use client';

import { useEffect, useMemo, useState } from 'react';
import {
    AlertTriangle,
    CheckCircle2,
    FileText,
    Image as ImageIcon,
    Link2,
    Play,
    Type,
    Upload,
    UserRound,
    X,
} from 'lucide-react';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import { Combobox, type ComboboxOption } from '@/components/ui/combobox';
import { FileDrop } from '@/components/ui/file-drop';
import { Input } from '@/components/ui/input';
import { Segmented } from '@/components/ui/segmented';
import { Spinner } from '@/components/ui/spinner';
import { useT } from '@/i18n/client';
import {
    CONTACT_FIELDS,
    fieldOf,
    fieldToken,
    isMediaHeader,
    issuesOf,
    placeholdersIn,
    type MediaHeaderKind,
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

type Slot = {
    key: string;
    section: 'header' | 'body' | 'button';
    name: string;
};

function slotsOf(shape: TemplateShape): Slot[] {
    return [
        ...shape.headerVars.map((name) => ({
            key: `header:${name}`,
            section: 'header' as const,
            name,
        })),
        ...shape.bodyVars.map((name) => ({
            key: `body:${name}`,
            section: 'body' as const,
            name,
        })),
        ...shape.urlButtons.map((i) => ({
            key: `button:${i}`,
            section: 'button' as const,
            name: String(i),
        })),
    ];
}

function get(v: ParamValues, s: Slot): string {
    if (s.section === 'button') return v.buttons[Number(s.name)] ?? '';
    return v[s.section][s.name] ?? '';
}

function set(v: ParamValues, s: Slot, value: string): ParamValues {
    if (s.section === 'button')
        return { ...v, buttons: { ...v.buttons, [Number(s.name)]: value } };
    return { ...v, [s.section]: { ...v[s.section], [s.name]: value } };
}

/**
 * Fills a template's variables, header media and URL buttons: each
 * variable is a contact field (with a value for contacts that lack it) or
 * fixed text. Used by campaigns, the inbox and automations.
 */
export function TemplateParamsEditor({
    shape,
    value,
    onChange,
    attributeKeys = [],
    phoneNumberId,
    fieldMapping = true,
    disabled,
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
}) {
    const t = useT();
    const slots = slotsOf(shape);
    const fieldLabel = (f: string) =>
        f.startsWith('attr:')
            ? t('params.attribute', { key: f.slice(5) })
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
        ...new Set(slots.flatMap((s) => placeholdersIn(get(value, s)))),
    ];

    return (
        <div className="space-y-5">
            {isMediaHeader(shape.header) && (
                <MediaPicker
                    kind={shape.header}
                    value={value}
                    onChange={onChange}
                    phoneNumberId={phoneNumberId}
                    disabled={disabled}
                />
            )}
            {slots.length === 0 && !isMediaHeader(shape.header) && (
                <p className="text-sm text-muted-foreground">
                    {t('params.noVariables')}
                </p>
            )}
            {slots.map((s) => (
                <SlotRow
                    key={s.key}
                    label={
                        s.section === 'button'
                            ? t('params.button', { n: Number(s.name) + 1 })
                            : t(`params.${s.section}`, { name: s.name })
                    }
                    example={shape.examples[s.key]}
                    value={get(value, s)}
                    onChange={(x) => onChange(set(value, s, x))}
                    options={options}
                    fieldMapping={fieldMapping}
                    disabled={disabled}
                    testId={`param-${s.key}`}
                />
            ))}
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

function SlotRow({
    label,
    example,
    value,
    onChange,
    options,
    fieldMapping,
    disabled,
    testId,
}: {
    label: string;
    example?: string;
    value: string;
    onChange: (v: string) => void;
    options: ComboboxOption[];
    fieldMapping: boolean;
    disabled?: boolean;
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
            {mode === 'field' ? (
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
                    maxLength={1024}
                    onChange={(e) => onChange(e.target.value)}
                />
            )}
        </div>
    );
}

function MediaPicker({
    kind,
    value,
    onChange,
    phoneNumberId,
    disabled,
}: {
    kind: MediaHeaderKind;
    value: ParamValues;
    onChange: (v: ParamValues) => void;
    phoneNumberId?: string;
    disabled?: boolean;
}) {
    const t = useT();
    const media = value.media?.type === kind ? value.media : undefined;
    const [mode, setMode] = useState<'upload' | 'link'>(
        media?.link ? 'link' : 'upload',
    );
    const [link, setLink] = useState(media?.link ?? '');
    const [busy, setBusy] = useState(false);
    const [error, setError] = useState<string | null>(null);
    const Icon = MEDIA_ICON[kind];
    const kindLabel = t(`params.media.${kind}`);

    // Free the object URL of a replaced upload.
    useEffect(() => {
        const src = value.mediaPreview;
        return () => {
            if (src?.startsWith('blob:')) URL.revokeObjectURL(src);
        };
    }, [value.mediaPreview]);

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
            onChange({
                ...value,
                media: {
                    type: kind,
                    id: body.data.id,
                    filename:
                        kind === 'document' ? body.data.filename : undefined,
                },
                mediaPreview:
                    kind === 'document' ? undefined : URL.createObjectURL(file),
            });
        } finally {
            setBusy(false);
        }
    };

    const applyLink = (v: string) => {
        setLink(v);
        const ok = /^https:\/\/\S+$/i.test(v.trim());
        setError(v && !ok ? t('params.media.httpsOnly') : null);
        onChange({
            ...value,
            media: ok
                ? {
                      type: kind,
                      link: v.trim(),
                      filename: kind === 'document' ? fileName(v) : undefined,
                  }
                : undefined,
            mediaPreview: ok && kind !== 'document' ? v.trim() : undefined,
        });
    };

    return (
        <div
            className="space-y-3 rounded-lg border p-3"
            data-testid="param-media"
        >
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
                            onChange({
                                ...value,
                                media: undefined,
                                mediaPreview: undefined,
                            });
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
    className,
}: {
    shape: TemplateShape;
    value: ParamValues;
    className?: string;
}) {
    const t = useT();
    const issues = issuesOf(value, shape);
    if (issues.length === 0) return null;
    const slotLabel = (slot: string) => {
        const [section, name] = slot.split(':') as [
            'header' | 'body' | 'button',
            string,
        ];
        return section === 'button'
            ? t('params.button', { n: Number(name) + 1 })
            : t(`params.${section}`, { name });
    };
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
                                  : t('params.issues.value', {
                                        slot: slotLabel(i.slot),
                                    })}
                        </li>
                    ))}
                </ul>
            </AlertDescription>
        </Alert>
    );
}
