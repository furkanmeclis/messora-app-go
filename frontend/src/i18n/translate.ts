import type { Locale } from './config';
import type { Messages } from './messages/en';

type Join<K extends string, P extends string> = `${K}.${P}`;

/** Every dotted path to a string in the dictionary ("settings.title"). */
export type MessageKey<T = Messages> = {
    [K in keyof T & string]: T[K] extends string
        ? K
        : Join<K, MessageKey<T[K]>>;
}[keyof T & string];

export type Vars = Record<string, string | number>;
export type Translate = (key: MessageKey, vars?: Vars) => string;

function lookup(messages: Messages, key: string): string | undefined {
    let node: unknown = messages;
    for (const part of key.split('.')) {
        if (node === null || typeof node !== 'object') return undefined;
        node = (node as Record<string, unknown>)[part];
    }
    return typeof node === 'string' ? node : undefined;
}

/**
 * Builds t(). Messages interpolate {name}; "one|other" picks a plural form
 * from vars.count with the language's plural rules.
 */
export function createTranslator(
    messages: Messages,
    locale: Locale,
): Translate {
    const plural = new Intl.PluralRules(locale);
    return (key, vars) => {
        let msg = lookup(messages, key) ?? key;
        if (vars && typeof vars.count === 'number' && msg.includes('|')) {
            const forms = msg.split('|');
            const form =
                plural.select(vars.count) === 'one' ? forms[0] : forms[1];
            msg = form ?? forms[0] ?? msg;
        }
        if (!vars) return msg;
        return msg.replace(/\{(\w+)\}/g, (m, name: string) =>
            name in vars ? String(vars[name]) : m,
        );
    };
}
