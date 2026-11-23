'use client';

import { Lock } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import {
    Tooltip,
    TooltipContent,
    TooltipTrigger,
} from '@/components/ui/tooltip';
import { useT } from '@/i18n/client';

/** Environment variables behind each lockable setting (see .env.example). */
export const ENV_NAMES: Record<string, string> = {
    organization_name: 'APP_ORGANIZATION_NAME',
    default_locale: 'APP_DEFAULT_LOCALE',
    default_timezone: 'APP_DEFAULT_TIMEZONE',
    date_format: 'APP_DATE_FORMAT',
    time_format: 'APP_TIME_FORMAT',
    mfa_required_roles: 'APP_MFA_REQUIRED_ROLES',
};

/** Marks a field set by an environment variable. */
export function LockedBadge({ field }: { field: string }) {
    const t = useT();
    return (
        <Tooltip>
            <TooltipTrigger asChild>
                <Badge variant="outline" className="gap-1 font-normal">
                    <Lock className="size-3" />
                    {t('common.managedByEnv')}
                </Badge>
            </TooltipTrigger>
            <TooltipContent className="max-w-64">
                {t('common.managedByEnvHint', {
                    name: ENV_NAMES[field] ?? field,
                })}
            </TooltipContent>
        </Tooltip>
    );
}
