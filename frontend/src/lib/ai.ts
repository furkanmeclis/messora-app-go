/**
 * Rough token estimate for the prompt editor: about four characters per
 * token for Latin text. Providers count exactly; this is only a guide.
 */
export function estimateTokens(text: string): number {
    const trimmed = text.trim();
    return trimmed ? Math.ceil(trimmed.length / 4) : 0;
}

/** Critical intents typed as "complaint, Refund request" → snake_case, unique. */
export function parseIntents(text: string): string[] {
    const out: string[] = [];
    for (const part of text.split(',')) {
        const v = part
            .trim()
            .toLowerCase()
            .replace(/[\s-]+/g, '_')
            .replace(/[^a-z0-9_]/g, '');
        if (v && !out.includes(v)) out.push(v);
    }
    return out;
}
