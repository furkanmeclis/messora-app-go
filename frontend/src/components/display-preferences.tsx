'use client';

import { Fragment, useEffect, useState } from 'react';
import { setDisplayPreferences } from '@/lib/format';

/**
 * Renders the panel once with the fixed server formatting (so hydration
 * matches), then applies the user's time zone and language and re-renders.
 */
export function DisplayPreferences({
    timeZone,
    locale,
    children,
}: {
    timeZone: string;
    locale: string;
    children: React.ReactNode;
}) {
    const [applied, setApplied] = useState('');
    useEffect(() => {
        setDisplayPreferences(timeZone, locale);
        // eslint-disable-next-line react-hooks/set-state-in-effect -- one re-render after an external (module) update
        setApplied(`${timeZone}|${locale}`);
    }, [timeZone, locale]);
    return <Fragment key={applied}>{children}</Fragment>;
}
