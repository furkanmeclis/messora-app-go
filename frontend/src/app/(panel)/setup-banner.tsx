import Link from 'next/link';
import { ArrowRight, Sparkles } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { getT } from '@/i18n/server';

/** Reminds the owner to finish the first-run wizard. */
export async function SetupBanner({ organization }: { organization: string }) {
    const t = await getT();
    return (
        <div className="mb-6 flex flex-wrap items-center gap-3 rounded-xl border border-primary/20 bg-linear-to-r from-primary/10 via-accent/40 to-transparent p-4">
            <span className="flex size-9 items-center justify-center rounded-lg bg-primary text-primary-foreground">
                <Sparkles className="size-4" />
            </span>
            <p className="min-w-0 flex-1 text-sm font-medium">
                {t('nav.finishSetup', { organization })}
            </p>
            <Button asChild size="sm">
                <Link href="/welcome">
                    {t('nav.resumeSetup')} <ArrowRight />
                </Link>
            </Button>
        </div>
    );
}
