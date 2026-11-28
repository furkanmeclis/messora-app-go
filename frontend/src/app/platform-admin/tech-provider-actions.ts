'use server';

import { revalidatePath } from 'next/cache';
import { runAction } from '@/lib/server/actions';
import { papi } from '@/lib/server/platform';
import { getT } from '@/i18n/server';
import type { ActionResult, Schemas } from '@/lib/types';

/*
 * Messora as Meta Tech Provider (phase 17 WP6): the platform admin's
 * settings. Secrets are write-only; the API answers whether each is set.
 */

type Config = Schemas['PlatformTechProvider'];

export async function updateTechProviderAction(
    body: Schemas['UpdatePlatformTechProviderRequest'],
): Promise<ActionResult<Config>> {
    const t = await getT();
    const r = await runAction(
        async () =>
            (
                await papi<Config>('/platform/meta/tech-provider', {
                    method: 'PATCH',
                    body,
                })
            ).data,
        t('platform.techProvider.saved'),
    );
    revalidatePath('/platform-admin', 'layout');
    return r;
}

export async function testTechProviderAction(): Promise<
    ActionResult<Schemas['PlatformTechProviderTest']>
> {
    return runAction(
        async () =>
            (
                await papi<Schemas['PlatformTechProviderTest']>(
                    '/platform/meta/tech-provider/test',
                    { method: 'POST', timeoutMs: 60_000 },
                )
            ).data,
    );
}
