'use server';

import { revalidatePath } from 'next/cache';
import { getT } from '@/i18n/server';
import { api } from '@/lib/server/api';
import { runAction } from '@/lib/server/actions';
import { invalidateAppearance } from '@/lib/server/appearance';
import type { ActionResult, Schemas } from '@/lib/types';

type Info = Schemas['LicenseInfo'];

/** Verifies and stores a licence key (owner only). */
export async function setLicenseAction(
    key: string,
): Promise<ActionResult<Info>> {
    const t = await getT();
    const res = await runAction(
        async () =>
            (await api<Info>('/license', { method: 'PUT', body: { key } }))
                .data,
        t('license.keySaved'),
    );
    invalidateAppearance();
    revalidatePath('/', 'layout');
    return res;
}
