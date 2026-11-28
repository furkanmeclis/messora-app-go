'use client';

import { Copy } from 'lucide-react';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { useT } from '@/i18n/client';

/** A read-only value with a copy button (links shown once, DNS records). */
export function CopyField({
    value,
    testId,
}: {
    value: string;
    testId?: string;
}) {
    const t = useT();
    return (
        <div className="flex gap-2">
            <Input
                readOnly
                value={value}
                data-testid={testId}
                className="font-mono text-xs"
                onFocus={(e) => e.currentTarget.select()}
            />
            <Button
                type="button"
                variant="outline"
                size="icon"
                aria-label={t('platform.tenants.copy')}
                onClick={() =>
                    navigator.clipboard
                        .writeText(value)
                        .then(() => toast.success(t('platform.tenants.copied')))
                        .catch(() => undefined)
                }
            >
                <Copy />
            </Button>
        </div>
    );
}
