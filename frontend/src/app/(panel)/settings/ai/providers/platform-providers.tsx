'use client';

import { Sparkles } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { useT } from '@/i18n/client';
import type { Schemas } from '@/lib/types';
import { cn } from '@/lib/utils';

/** Messora's AI keys: usable without set-up, paid with AI credits. */
export function PlatformProviders({
    providers,
}: {
    providers: Schemas['AIPlatformProvider'][];
}) {
    const t = useT();
    const usable = providers.filter((p) => p.enabled);
    return (
        <section
            className="rounded-2xl border bg-linear-to-br from-primary/5 to-transparent p-5"
            data-testid="platform-ai-providers"
        >
            <h2 className="flex items-center gap-2 font-semibold tracking-tight">
                <Sparkles className="size-4 text-primary" aria-hidden />
                {t('plans.ai.platformTitle')}
            </h2>
            <p className="mt-1 text-sm text-muted-foreground">
                {t('plans.ai.platformHint')}
            </p>
            {usable.length === 0 ? (
                <p className="mt-3 text-sm text-muted-foreground">
                    {t('plans.ai.platformEmpty')}
                </p>
            ) : (
                <ul className="mt-3 flex flex-wrap gap-2">
                    {usable.map((p) => (
                        <li key={p.id}>
                            <Badge
                                variant="outline"
                                className={cn(
                                    'gap-1.5 py-1',
                                    p.status === 'ok' &&
                                        'border-success/40 bg-success/10 text-success',
                                )}
                            >
                                {t(`ai.kinds.${p.kind}`)}
                                <span className="text-muted-foreground">
                                    · {p.name}
                                </span>
                            </Badge>
                        </li>
                    ))}
                </ul>
            )}
        </section>
    );
}
