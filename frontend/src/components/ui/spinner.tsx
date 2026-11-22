// Derived from ZeroMsg (MIT, Copyright (c) 2026 megoxv) — see NOTICE.
import { Loader2Icon } from 'lucide-react';

import { cn } from '@/lib/utils';

function Spinner({ className, ...props }: React.ComponentProps<'svg'>) {
    return (
        <Loader2Icon
            role="status"
            aria-label="Loading"
            className={cn('size-4 animate-spin', className)}
            {...props}
        />
    );
}

export { Spinner };
