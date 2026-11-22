import 'server-only';

import { readFileSync } from 'node:fs';

/**
 * Server-side configuration for the BFF. Secrets are read from the process
 * environment or from the file `messora init` writes (MESSORA_SECRETS_FILE),
 * and never reach client bundles: this module is `server-only`.
 */
export type ServerConfig = {
    apiInternalUrl: string;
    bffSharedSecret: string;
};

let cached: ServerConfig | undefined;

function parseDotenv(text: string): Record<string, string> {
    const out: Record<string, string> = {};
    for (const raw of text.split('\n')) {
        const line = raw.trim();
        if (!line || line.startsWith('#')) continue;
        const eq = line.indexOf('=');
        if (eq < 1) continue;
        const key = line
            .slice(0, eq)
            .trim()
            .replace(/^export\s+/, '');
        let value = line.slice(eq + 1).trim();
        if (
            value.length >= 2 &&
            (value[0] === '"' || value[0] === "'") &&
            value.endsWith(value[0])
        ) {
            value = value.slice(1, -1);
        }
        out[key] = value;
    }
    return out;
}

export function serverConfig(): ServerConfig {
    if (cached) return cached;

    let file: Record<string, string> = {};
    const path = process.env.MESSORA_SECRETS_FILE;
    if (path) {
        try {
            file = parseDotenv(readFileSync(path, 'utf8'));
        } catch (err) {
            throw new Error(
                `Cannot read MESSORA_SECRETS_FILE: ${(err as NodeJS.ErrnoException).code ?? 'unknown error'}`,
            );
        }
    }
    const pick = (key: string) =>
        process.env[key]?.trim() || file[key]?.trim() || '';

    const bffSharedSecret = pick('BFF_SHARED_SECRET');
    if (bffSharedSecret.length < 32) {
        throw new Error(
            'BFF_SHARED_SECRET is missing or shorter than 32 characters',
        );
    }
    cached = {
        apiInternalUrl: (
            pick('MESSORA_API_INTERNAL_URL') || 'http://messora-api:8080'
        ).replace(/\/+$/, ''),
        bffSharedSecret,
    };
    return cached;
}
