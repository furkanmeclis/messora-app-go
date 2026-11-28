'use client';

import { UpgradeHint } from '@/components/plan-hint';
import { useT } from '@/i18n/client';

/** In place of the SMTP form on plans without custom_smtp. */
export function CustomSmtpLocked() {
    const t = useT();
    return (
        <UpgradeHint testId="smtp-locked">
            {t('plans.email.customLocked')}
        </UpgradeHint>
    );
}
