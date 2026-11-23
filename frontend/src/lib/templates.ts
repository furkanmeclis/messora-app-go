import type { Template } from './types';

/**
 * Helpers for Meta's template `components` JSON, shared by the templates
 * page (preview, create form) and the campaign wizard (variable mapping).
 * Templates come from Meta, so every field is treated as untrusted input.
 */

export type TemplateButton = {
    type: string;
    text: string;
    url?: string;
    phone_number?: string;
};

export type ParsedTemplate = {
    headerFormat?: string;
    header?: string;
    body: string;
    footer?: string;
    buttons: TemplateButton[];
};

const VARIABLE = /\{\{\s*([A-Za-z0-9_]+)\s*\}\}/g;

function str(v: unknown): string | undefined {
    return typeof v === 'string' ? v : undefined;
}

export function parseComponents(components: unknown): ParsedTemplate {
    const out: ParsedTemplate = { body: '', buttons: [] };
    if (!Array.isArray(components)) return out;
    for (const raw of components) {
        if (!raw || typeof raw !== 'object') continue;
        const c = raw as Record<string, unknown>;
        switch (String(c.type ?? '').toUpperCase()) {
            case 'HEADER':
                out.headerFormat = String(c.format ?? 'TEXT').toUpperCase();
                out.header = str(c.text);
                break;
            case 'BODY':
                out.body = str(c.text) ?? '';
                break;
            case 'FOOTER':
                out.footer = str(c.text);
                break;
            case 'BUTTONS':
                if (Array.isArray(c.buttons)) {
                    for (const b of c.buttons) {
                        if (!b || typeof b !== 'object') continue;
                        const x = b as Record<string, unknown>;
                        out.buttons.push({
                            type: String(x.type ?? '').toUpperCase(),
                            text: str(x.text) ?? '',
                            url: str(x.url),
                            phone_number: str(x.phone_number),
                        });
                    }
                }
                break;
        }
    }
    return out;
}

/** Variable names in order of first appearance ("1", "2" or "first_name"). */
export function variablesOf(text: string | undefined): string[] {
    if (!text) return [];
    const seen: string[] = [];
    for (const m of text.matchAll(VARIABLE)) {
        const name = m[1]!;
        if (!seen.includes(name)) seen.push(name);
    }
    return seen;
}

/** What a campaign has to fill in for a template. */
export type TemplateSlots = {
    named: boolean;
    header: string[];
    body: string[];
    /** Indexes (button positions) of URL buttons with a variable. */
    urlButtons: number[];
    /** A media header needs a file at send time, which campaigns lack. */
    mediaHeader: boolean;
};

export function slotsOf(t: Template): TemplateSlots {
    const p = parseComponents(t.components);
    const named = t.parameter_format?.toUpperCase() === 'NAMED';
    const order = (vars: string[]) =>
        named ? vars : [...vars].sort((a, b) => Number(a) - Number(b) || 0);
    return {
        named,
        header:
            p.headerFormat === 'TEXT' || p.headerFormat === undefined
                ? order(variablesOf(p.header))
                : [],
        body: order(variablesOf(p.body)),
        urlButtons: p.buttons
            .map((b, i) => (b.type === 'URL' && hasVariable(b.url) ? i : -1))
            .filter((i) => i >= 0),
        mediaHeader:
            p.headerFormat !== undefined &&
            p.headerFormat !== 'TEXT' &&
            p.headerFormat !== 'NONE',
    };
}

function hasVariable(s: string | undefined): boolean {
    return !!s && /\{\{\s*[A-Za-z0-9_]+\s*\}\}/.test(s);
}

/** Replaces each {{var}} in text with values[var] when given. */
export function fillVariables(
    text: string,
    values: Record<string, string>,
): string {
    return text.replace(VARIABLE, (m, name: string) => {
        const v = values[name];
        return v ? v : m;
    });
}

/** Templates are sent by name and language; this key identifies one. */
export function templateKey(t: { name: string; language: string }): string {
    return `${t.name}|${t.language}`;
}

export const TEMPLATE_STATUSES = [
    'APPROVED',
    'PENDING',
    'REJECTED',
    'PAUSED',
    'DISABLED',
] as const;
