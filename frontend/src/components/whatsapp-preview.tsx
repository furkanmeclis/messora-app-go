'use client';

import { Fragment, useState, type ReactNode } from 'react';
import {
    CheckCheck,
    Copy,
    ExternalLink,
    FileText,
    Image as ImageIcon,
    MapPin,
    Phone,
    Play,
    Reply,
} from 'lucide-react';
import { useT } from '@/i18n/client';
import {
    parseComponents,
    resolvePieces,
    type ParamValues,
    type SampleContact,
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
 * formatting, buttons. With params and a sample contact, variables show the
 * values that contact would receive.
 */
export function WhatsAppPreview({
    components,
    params,
    literal,
    contact,
    mediaSrc,
    mediaName,
    className,
}: {
    components: unknown;
    /** Values in the campaign format; placeholders are resolved. */
    params?: ParamValues;
    /** Plain values by variable (the template editor's examples). */
    literal?: {
        header?: Record<string, string>;
        body?: Record<string, string>;
    };
    contact?: SampleContact;
    /** What to show in a media header (link, object URL or example). */
    mediaSrc?: string;
    mediaName?: string;
    className?: string;
}) {
    const t = useT();
    const p = parseComponents(components);
    const label: Labels = (field) =>
        field.startsWith('attr:')
            ? field.slice(5)
            : t(`params.fields.${field as 'name'}`);
    const header = params?.header ?? literal?.header;
    const body = params?.body ?? literal?.body;
    const format = p.headerFormat?.toUpperCase();
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
                    <MediaPlaceholder
                        icon={<MapPin />}
                        text={t('params.media.location')}
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
                    {p.body ? (
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
                    {p.footer && (
                        <p className="text-xs text-[#667781] dark:text-[#8696a0]">
                            {p.footer}
                        </p>
                    )}
                    <p className="flex items-center justify-end gap-1 text-[0.6875rem] text-[#667781] dark:text-[#8696a0]">
                        12:34 <CheckCheck className="size-3.5 text-[#53bdeb]" />
                    </p>
                </div>
                {p.buttons.length > 0 && (
                    <ul className="divide-y divide-black/10 border-t border-black/10 dark:divide-white/10 dark:border-white/10">
                        {p.buttons.map((b, i) => (
                            <li
                                key={i}
                                className="flex items-center justify-center gap-1.5 px-2 py-2 text-[0.8125rem] font-medium text-[#027eb5] dark:text-[#53bdeb]"
                                title={
                                    b.url
                                        ? fillUrl(b.url, params?.buttons[i])
                                        : undefined
                                }
                            >
                                {b.type === 'URL' ? (
                                    <ExternalLink className="size-3.5" />
                                ) : b.type === 'PHONE_NUMBER' ? (
                                    <Phone className="size-3.5" />
                                ) : b.type === 'COPY_CODE' ? (
                                    <Copy className="size-3.5" />
                                ) : (
                                    <Reply className="size-3.5" />
                                )}
                                {b.text || b.type.toLowerCase()}
                            </li>
                        ))}
                    </ul>
                )}
            </div>
        </div>
    );
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
