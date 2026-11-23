import { ExternalLink, Image as ImageIcon, Phone, Reply } from 'lucide-react';
import { fillVariables, parseComponents } from '@/lib/templates';

/**
 * A message bubble showing a template's text parts. Variables stay visible
 * as {{1}} / {{name}} unless values are given.
 */
export function TemplatePreview({
    components,
    values,
    className,
}: {
    components: unknown;
    values?: { header?: Record<string, string>; body?: Record<string, string> };
    className?: string;
}) {
    const p = parseComponents(components);
    const header = p.header
        ? fillVariables(p.header, values?.header ?? {})
        : '';
    const body = fillVariables(p.body, values?.body ?? {});
    return (
        <div
            className={`max-w-sm rounded-lg border bg-muted/40 p-3 text-sm ${className ?? ''}`}
            data-testid="template-preview"
        >
            {p.headerFormat && p.headerFormat !== 'TEXT' && (
                <div className="mb-2 flex items-center gap-2 rounded bg-muted p-2 text-xs text-muted-foreground">
                    <ImageIcon className="size-4" />
                    {p.headerFormat.toLowerCase()} header
                </div>
            )}
            {header && (
                <p className="mb-1 font-semibold break-words">{header}</p>
            )}
            {body ? (
                <p className="break-words whitespace-pre-wrap">{body}</p>
            ) : (
                <p className="text-muted-foreground italic">No body text</p>
            )}
            {p.footer && (
                <p className="mt-1 text-xs text-muted-foreground">{p.footer}</p>
            )}
            {p.buttons.length > 0 && (
                <ul className="mt-2 space-y-1 border-t pt-2">
                    {p.buttons.map((b, i) => (
                        <li
                            key={i}
                            className="flex items-center justify-center gap-1.5 text-xs font-medium text-primary"
                        >
                            {b.type === 'URL' ? (
                                <ExternalLink className="size-3" />
                            ) : b.type === 'PHONE_NUMBER' ? (
                                <Phone className="size-3" />
                            ) : (
                                <Reply className="size-3" />
                            )}
                            {b.text || b.type.toLowerCase()}
                            {b.url && (
                                <span className="sr-only"> ({b.url})</span>
                            )}
                        </li>
                    ))}
                </ul>
            )}
        </div>
    );
}
