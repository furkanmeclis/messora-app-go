import { readFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

/**
 * The platform admin of the E2E stack: scripts/e2e.sh creates it with
 * `messora admin create`; identity.spec.ts enrols its second factor and
 * leaves the secret in a file for the later specs.
 */
export const PLATFORM = {
    adminUrl: process.env.E2E_ADMIN_URL ?? 'http://admin.e2e.localhost:8090',
    platformUrl:
        process.env.E2E_PLATFORM_URL ?? 'http://app.e2e.localhost:8090',
    baseDomain: process.env.E2E_BASE_DOMAIN ?? 'e2e.localhost',
    mailpitUrl: process.env.E2E_MAILPIT_URL ?? 'http://127.0.0.1:8097',
    email: process.env.E2E_ADMIN_EMAIL ?? 'pat@example.com',
    password: process.env.E2E_ADMIN_PASSWORD ?? 'Platform-keeper-2026-x',
};

/** The origin of a workspace sub-domain on the stack. */
export function workspaceUrl(slug: string): string {
    const port = new URL(PLATFORM.adminUrl).port;
    return `http://${slug}.${PLATFORM.baseDomain}${port ? ':' + port : ''}`;
}

export function platformStateFile(): string {
    return join(
        tmpdir(),
        `${process.env.E2E_PROJECT ?? 'messora-e2e'}-platform-admin.json`,
    );
}

/** The second factor identity.spec.ts set up. */
export function platformSecret(): { secret: string; recovery: string[] } {
    return JSON.parse(readFileSync(platformStateFile(), 'utf8'));
}

type MailpitSummary = {
    ID: string;
    Subject: string;
    To: { Address: string }[];
};

/** Waits for an e-mail to `to` in the stack's mail catcher and returns its text. */
export async function waitForMail(
    to: string,
    subject: RegExp,
    timeoutMs = 30_000,
): Promise<string> {
    const deadline = Date.now() + timeoutMs;
    while (Date.now() < deadline) {
        const res = await fetch(
            `${PLATFORM.mailpitUrl}/api/v1/search?query=${encodeURIComponent(`to:"${to}"`)}`,
        );
        if (res.ok) {
            const body = (await res.json()) as { messages: MailpitSummary[] };
            const hit = body.messages.find((m) => subject.test(m.Subject));
            if (hit) {
                const msg = await fetch(
                    `${PLATFORM.mailpitUrl}/api/v1/message/${hit.ID}`,
                );
                const full = (await msg.json()) as { Text: string };
                return full.Text;
            }
        }
        await new Promise((r) => setTimeout(r, 500));
    }
    throw new Error(`no e-mail to ${to} matching ${subject}`);
}
