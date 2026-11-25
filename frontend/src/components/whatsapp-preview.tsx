'use client';

import { Fragment, useEffect, useState, type ReactNode } from 'react';
import {
    CheckCheck,
    Copy,
    ExternalLink,
    FileText,
    Gift,
    Image as ImageIcon,
    List,
    ListChecks,
    MapPin,
    Phone,
    PhoneCall,
    Play,
    Reply,
} from 'lucide-react';
import { useT } from '@/i18n/client';
import {
    parseComponents,
    remaining,
    resolvePieces,
    type ParamValues,
    type SampleContact,
    type TemplateButton,
} from '@/lib/templates';
import { cn } from '@/lib/utils';

const VARIABLE = /\{\{\s*([A-Za-z0-9_]+)\s*\}\}/g;

/** WhatsApp's inline formatting: *bold* _italic_ ~strike~ ```mono```. */
function formatted(text: string, key: string): ReactNode[] {
    const out: ReactNode[] = [];
    const re = /```([^`]+)```|\*([^*\n]+)\*|_([^_\n]+)_|~([^~\n]+)~/g;
    let last = 0;
    let i = 0;
    for (const m of text.matchAll(re)) {
        if (m.index! > last) out.push(text.slice(last, m.index));
        const k = `${key}-f${i++}`;
        if (m[1] !== undefined)
            out.push(
                <code key={k} className="font-mono text-[0.85em]">
                    {m[1]}
                </code>,
            );
        else if (m[2] !== undefined) out.push(<strong key={k}>{m[2]}</strong>);
        else if (m[3] !== undefined) out.push(<em key={k}>{m[3]}</em>);
        else out.push(<s key={k}>{m[4]}</s>);
        last = m.index! + m[0].length;
    }
    if (last < text.length) out.push(text.slice(last));
    return out;
}

type Labels = (field: string) => string;

/** Template text with its variables replaced by values or chips. */
function Filled({
    text,
    values,
    contact,
    fallback,
    label,
    prefix,
}: {
    text: string;
    values?: Record<string, string>;
    contact?: SampleContact;
    fallback?: Record<string, string>;
    label: Labels;
    prefix: string;
}) {
    const nodes: ReactNode[] = [];
    let last = 0;
    let n = 0;
    for (const m of text.matchAll(VARIABLE)) {
        if (m.index! > last)
            nodes.push(
                ...formatted(text.slice(last, m.index), `${prefix}${n}`),
            );
        const name = m[1]!;
        const value = values?.[name];
        const k = `${prefix}v${n++}`;
        if (!value?.trim()) {
            nodes.push(
                <span
                    key={k}
                    className="rounded bg-black/10 px-1 font-mono text-[0.8em] dark:bg-white/15"
                    data-unfilled
                >
                    {`{{${name}}}`}
                </span>,
            );
        } else {
            nodes.push(
                <Fragment key={k}>
                    {resolvePieces(value, contact, fallback).map((p, j) =>
                        'text' in p ? (
                            <Fragment key={j}>
                                {formatted(p.text, `${k}t${j}`)}
                            </Fragment>
                        ) : p.value ? (
                            <span
                                key={j}
                                className={cn(
                                    'underline decoration-dotted underline-offset-2',
                                    p.fallback && 'decoration-amber-500',
                                )}
                                title={label(p.field)}
                            >
                                {p.value}
                            </span>
                        ) : (
                            <span
                                key={j}
                                className="rounded bg-emerald-500/15 px-1 text-[0.85em] font-medium text-emerald-800 dark:text-emerald-300"
                            >
                                {label(p.field)}
                            </span>
                        ),
                    )}
                </Fragment>,
            );
        }
        last = m.index! + m[0].length;
    }
    if (last < text.length)
        nodes.push(...formatted(text.slice(last), `${prefix}e`));
    return <>{nodes}</>;
}

/**
 * A template as WhatsApp shows it: chat background, bubble, media header,
 * formatting, buttons, and the special components (location map,
 * limited-time offer with its countdown, copy code, carousel cards,
 * one-time password). With params and a sample contact, variables show the
 * values that contact would receive.
 */
export function WhatsAppPreview({
    components,
    params,
    literal,
    contact,
    mediaSrc,
    mediaName,
    category,
    expiresAt,
    coupon,
    cardMedia,
    className,
}: {
    components: unknown;
    /** Values in the campaign format; placeholders are resolved. */
    params?: ParamValues;
    /** Plain values by variable (the template editor's examples). */
    literal?: {
        header?: Record<string, string>;
        body?: Record<string, string>;
        /** Carousel cards' body examples, by card. */
        cards?: Record<string, string>[];
    };
    contact?: SampleContact;
    /** What to show in a media header (link, object URL or example). */
    mediaSrc?: string;
    mediaName?: string;
    /** AUTHENTICATION shows Meta's fixed code message. */
    category?: string;
    /** End of a limited-time offer (Unix ms) for the countdown row. */
    expiresAt?: number;
    /** The code a copy code button copies. */
    coupon?: string;
    /** What to show in each carousel card's header. */
    cardMedia?: (string | undefined)[];
    className?: string;
}) {
    const t = useT();
    const p = parseComponents(components);
    const label: Labels = (field) =>
        field.startsWith('attr:')
            ? field.slice(5)
            : field.startsWith('offer_')
              ? t('params.offerField', { field: field.slice(6) })
              : t(`params.fields.${field as 'name'}`);
    const header = params?.header ?? literal?.header;
    const body = params?.body ?? literal?.body;
    const format = p.headerFormat?.toUpperCase();
    const auth =
        category?.toUpperCase() === 'AUTHENTICATION' ||
        p.buttons.some((b) => b.type === 'OTP');
    const code =
        coupon ??
        (params?.coupon || undefined) ??
        p.buttons.find((b) => b.type === 'COPY_CODE')?.example;
    const ends = expiresAt ?? params?.expirationMs;
    const location = params?.location;
    return (
        <div
            className={cn(
                'rounded-xl bg-[#efeae2] p-3 dark:bg-[#0b141a]',
                className,
            )}
            data-testid="template-preview"
        >
            <div className="relative max-w-[20rem] overflow-hidden rounded-lg rounded-tl-none bg-white text-[0.875rem] leading-snug text-[#111b21] shadow-sm dark:bg-[#202c33] dark:text-[#e9edef]">
                {format === 'IMAGE' && (
                    <ImageHeader
                        src={mediaSrc}
                        label={t('params.media.image')}
                    />
                )}
                {format === 'VIDEO' &&
                    (mediaSrc ? (
                        <video
                            src={mediaSrc}
                            muted
                            className="m-1 mb-0 aspect-video w-[calc(100%-0.5rem)] rounded-md bg-black object-cover"
                        />
                    ) : (
                        <MediaPlaceholder
                            icon={<Play />}
                            text={t('params.media.video')}
                        />
                    ))}
                {format === 'DOCUMENT' && (
                    <div className="m-1 mb-0 flex items-center gap-2 rounded-md bg-black/5 p-2.5 dark:bg-white/10">
                        <FileText className="size-7 shrink-0 text-red-500" />
                        <span className="truncate text-xs">
                            {mediaName || t('params.media.document')}
                        </span>
                    </div>
                )}
                {format === 'LOCATION' && (
                    <div
                        className="m-1 mb-0 overflow-hidden rounded-md bg-[repeating-linear-gradient(45deg,#d9e7d4_0_12px,#e6efe1_12px_24px)] dark:bg-[repeating-linear-gradient(45deg,#1e2d24_0_12px,#24352a_12px_24px)]"
                        data-testid="preview-location"
                    >
                        <div className="flex aspect-[1.91/1] items-center justify-center">
                            <MapPin className="size-8 fill-red-500 text-red-700" />
                        </div>
                        {(location?.name || location?.address) && (
                            <div className="bg-white/80 px-2 py-1 text-xs dark:bg-black/40">
                                {location.name && (
                                    <p className="font-medium">
                                        {location.name}
                                    </p>
                                )}
                                {location.address && (
                                    <p className="text-[#667781]">
                                        {location.address}
                                    </p>
                                )}
                            </div>
                        )}
                        {!location?.name && !location?.address && (
                            <p className="bg-white/80 px-2 py-1 text-center text-xs text-[#667781] dark:bg-black/40">
                                {t('params.media.location')}
                            </p>
                        )}
                    </div>
                )}
                {p.lto && (
                    <OfferRow
                        text={p.lto.text}
                        hasExpiration={p.lto.hasExpiration}
                        ends={ends}
                        code={code}
                    />
                )}
                <div className="space-y-1 px-2.5 pt-1.5 pb-1">
                    {format === 'TEXT' && p.header && (
                        <p className="font-semibold break-words">
                            <Filled
                                text={p.header}
                                values={header}
                                contact={contact}
                                fallback={params?.fallback}
                                label={label}
                                prefix="h"
                            />
                        </p>
                    )}
                    {auth && !p.body ? (
                        <p className="break-words whitespace-pre-wrap">
                            <strong>{params?.otp || '123456'}</strong>{' '}
                            {t('templates.preview.otpBody')}
                            {p.securityRecommendation &&
                                ` ${t('templates.preview.otpSecurity')}`}
                        </p>
                    ) : p.body ? (
                        <p className="break-words whitespace-pre-wrap">
                            <Filled
                                text={p.body}
                                values={body}
                                contact={contact}
                                fallback={params?.fallback}
                                label={label}
                                prefix="b"
                            />
                        </p>
                    ) : (
                        <p className="text-[#667781] italic">
                            {t('templates.preview.noBody')}
                        </p>
                    )}
                    {(p.footer ||
                        (auth && p.codeExpirationMinutes !== undefined)) && (
                        <p className="text-xs text-[#667781] dark:text-[#8696a0]">
                            {p.footer && !auth
                                ? p.footer
                                : t('templates.preview.otpExpiry', {
                                      minutes: p.codeExpirationMinutes ?? 10,
                                  })}
                        </p>
                    )}
                    <p className="flex items-center justify-end gap-1 text-[0.6875rem] text-[#667781] dark:text-[#8696a0]">
                        12:34 <CheckCheck className="size-3.5 text-[#53bdeb]" />
                    </p>
                </div>
                {p.buttons.length > 0 && (
                    <PreviewButtons
                        buttons={p.buttons}
                        urlValues={params?.buttons}
                    />
                )}
            </div>
            {p.cards.length > 0 && (
                <ul
                    className="mt-1.5 flex max-w-full snap-x gap-2 overflow-x-auto pb-1"
                    data-testid="preview-cards"
                >
                    {p.cards.map((c, k) => (
                        <li
                            key={k}
                            className="w-44 shrink-0 snap-start overflow-hidden rounded-lg bg-white text-[0.8125rem] shadow-sm dark:bg-[#202c33] dark:text-[#e9edef]"
                        >
                            {c.headerFormat === 'VIDEO' ? (
                                <MediaPlaceholder
                                    icon={<Play />}
                                    text={t('params.media.video')}
                                />
                            ) : (
                                <ImageHeader
                                    src={
                                        cardMedia?.[k] ??
                                        params?.cards[k]?.mediaPreview ??
                                        params?.cards[k]?.media?.link
                                    }
                                    label={t('params.media.image')}
                                />
                            )}
                            {c.body && (
                                <p className="px-2 pt-1.5 break-words whitespace-pre-wrap">
                                    <Filled
                                        text={c.body}
                                        values={
                                            params?.cards[k]?.body ??
                                            literal?.cards?.[k]
                                        }
                                        contact={contact}
                                        fallback={params?.fallback}
                                        label={label}
                                        prefix={`c${k}`}
                                    />
                                </p>
                            )}
                            {c.buttons.length > 0 && (
                                <PreviewButtons
                                    buttons={c.buttons}
                                    urlValues={params?.cards[k]?.buttons}
                                />
                            )}
                        </li>
                    ))}
                </ul>
            )}
        </div>
    );
}

/** Meta's limited-time offer block: heading, countdown and code. */
function OfferRow({
    text,
    hasExpiration,
    ends,
    code,
}: {
    text: string;
    hasExpiration: boolean;
    ends?: number;
    code?: string;
}) {
    const t = useT();
    const [now, setNow] = useState(() => Date.now());
    useEffect(() => {
        if (!hasExpiration || !ends) return;
        const id = setInterval(() => setNow(Date.now()), 30_000);
        return () => clearInterval(id);
    }, [hasExpiration, ends]);
    const left = ends ? remaining(ends, now) : undefined;
    const soon = !!ends && ends - now < 3_600_000;
    return (
        <div
            className="mx-1 mt-1 flex items-start gap-2 rounded-md bg-black/5 p-2 dark:bg-white/10"
            data-testid="preview-offer"
        >
            <Gift className="mt-0.5 size-5 shrink-0 text-[#027eb5] dark:text-[#53bdeb]" />
            <div className="min-w-0 flex-1">
                <p className="font-semibold">
                    {text || t('templates.preview.offerHeading')}
                </p>
                {hasExpiration && (
                    <p
                        className={cn(
                            'text-xs text-[#667781] dark:text-[#8696a0]',
                            soon && 'text-red-600 dark:text-red-400',
                        )}
                    >
                        {!left
                            ? t('templates.preview.offerEndsSample')
                            : left.ended
                              ? t('templates.preview.offerEnded')
                              : t('templates.preview.offerEndsIn', {
                                    time: duration(t, left),
                                })}
                    </p>
                )}
                {code && (
                    <p className="font-mono text-xs tracking-wide">{code}</p>
                )}
            </div>
        </div>
    );
}

function duration(
    t: ReturnType<typeof useT>,
    d: { days: number; hours: number; minutes: number },
): string {
    if (d.days > 0)
        return t('templates.preview.daysHours', {
            days: d.days,
            hours: d.hours,
        });
    if (d.hours > 0)
        return t('templates.preview.hoursMinutes', {
            hours: d.hours,
            minutes: d.minutes,
        });
    return t('templates.preview.minutes', { minutes: Math.max(1, d.minutes) });
}

function PreviewButtons({
    buttons,
    urlValues,
}: {
    buttons: TemplateButton[];
    urlValues?: Record<number, string>;
}) {
    const t = useT();
    const shown = buttons.length > 3 ? buttons.slice(0, 2) : buttons;
    return (
        <ul className="divide-y divide-black/10 border-t border-black/10 dark:divide-white/10 dark:border-white/10">
            {shown.map((b, i) => (
                <li
                    key={i}
                    className="flex items-center justify-center gap-1.5 px-2 py-2 text-[0.8125rem] font-medium text-[#027eb5] dark:text-[#53bdeb]"
                    title={b.url ? fillUrl(b.url, urlValues?.[i]) : undefined}
                    data-button-type={b.type}
                >
                    <ButtonIcon type={b.type} />
                    {buttonLabel(t, b)}
                </li>
            ))}
            {buttons.length > 3 && (
                <li className="flex items-center justify-center gap-1.5 px-2 py-2 text-[0.8125rem] font-medium text-[#027eb5] dark:text-[#53bdeb]">
                    <List className="size-3.5" />
                    {t('templates.preview.seeAll')}
                </li>
            )}
        </ul>
    );
}

function ButtonIcon({ type }: { type: string }) {
    const cls = 'size-3.5';
    switch (type) {
        case 'URL':
            return <ExternalLink className={cls} />;
        case 'PHONE_NUMBER':
            return <Phone className={cls} />;
        case 'VOICE_CALL':
            return <PhoneCall className={cls} />;
        case 'COPY_CODE':
        case 'OTP':
            return <Copy className={cls} />;
        case 'FLOW':
            return <ListChecks className={cls} />;
        default:
            return <Reply className={cls} />;
    }
}

function buttonLabel(t: ReturnType<typeof useT>, b: TemplateButton): string {
    if (b.text) return b.text;
    switch (b.type) {
        case 'COPY_CODE':
            return t('templates.preview.copyCode');
        case 'OTP':
            return b.otp_type === 'COPY_CODE' || !b.otp_type
                ? t('templates.preview.copyCode')
                : b.autofill_text || t('templates.preview.autofill');
        case 'VOICE_CALL':
            return t('templates.preview.callNow');
    }
    return b.type.toLowerCase();
}

function fillUrl(url: string, value: string | undefined): string {
    return value ? url.replace(VARIABLE, value) : url;
}

function MediaPlaceholder({
    icon,
    text,
}: {
    icon: ReactNode;
    text: ReactNode;
}) {
    return (
        <div className="m-1 mb-0 flex aspect-[1.91/1] flex-col items-center justify-center gap-1 rounded-md bg-black/5 text-xs text-[#667781] dark:bg-white/10 [&>svg]:size-7">
            {icon}
            {text}
        </div>
    );
}

/**
 * The header image. Links to other sites are not loaded by the panel (its
 * content security policy), so they show as a labelled placeholder.
 */
function ImageHeader({ src, label }: { src?: string; label: string }) {
    const [failed, setFailed] = useState<string | null>(null);
    if (!src || failed === src) {
        return (
            <MediaPlaceholder
                icon={<ImageIcon />}
                text={
                    src ? (
                        <span className="max-w-full truncate px-3">{src}</span>
                    ) : (
                        label
                    )
                }
            />
        );
    }
    return (
        // eslint-disable-next-line @next/next/no-img-element -- user-provided preview, any origin
        <img
            src={src}
            alt=""
            onError={() => setFailed(src)}
            className="m-1 mb-0 aspect-[1.91/1] w-[calc(100%-0.5rem)] rounded-md object-cover"
        />
    );
}
