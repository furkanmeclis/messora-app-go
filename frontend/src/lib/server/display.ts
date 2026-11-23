import 'server-only';

import { getAppearance } from './appearance';
import { requireMe } from './auth';
import { prefsFor, type DisplayPrefs } from '@/lib/format';

/** Date display preferences for server components: the user's zone and
 * language with the workspace's date and time formats. */
export async function getDisplayPrefs(): Promise<DisplayPrefs> {
    const [me, a] = await Promise.all([requireMe(), getAppearance()]);
    return prefsFor(
        me.user.timezone,
        me.user.locale,
        a.date_format,
        a.time_format,
    );
}
