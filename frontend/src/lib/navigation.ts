import {
    Activity,
    BookUser,
    FileText,
    Inbox,
    KeyRound,
    Megaphone,
    Phone,
    ScrollText,
    ServerCrash,
    Settings,
    ShieldCheck,
    UserRound,
    Users,
    Webhook,
    Workflow,
    type LucideIcon,
} from 'lucide-react';
import type { MessageKey } from '@/i18n/translate';

export type NavItem = {
    id: string;
    href: string;
    label: MessageKey;
    icon: LucideIcon;
    permission?: string;
    group: 'workspace' | 'administration' | 'account';
    /** Second key of the "G then …" shortcut. */
    goKey?: string;
};

/**
 * Every section of the panel. The sidebar and desktop mode both render
 * this list, filtered by the user's permissions.
 */
export const NAV: NavItem[] = [
    {
        id: 'overview',
        goKey: 'h',
        href: '/',
        label: 'nav.overview',
        icon: Activity,
        group: 'workspace',
    },
    {
        id: 'inbox',
        goKey: 'i',
        href: '/inbox',
        label: 'nav.inbox',
        icon: Inbox,
        permission: 'conversations.read',
        group: 'workspace',
    },
    {
        id: 'contacts',
        goKey: 'c',
        href: '/contacts',
        label: 'nav.contacts',
        icon: BookUser,
        permission: 'contacts.read',
        group: 'workspace',
    },
    {
        id: 'campaigns',
        goKey: 'p',
        href: '/campaigns',
        label: 'nav.campaigns',
        icon: Megaphone,
        permission: 'campaigns.read',
        group: 'workspace',
    },
    {
        id: 'templates',
        goKey: 't',
        href: '/templates',
        label: 'nav.templates',
        icon: FileText,
        permission: 'templates.read',
        group: 'workspace',
    },
    {
        id: 'automations',
        goKey: 'a',
        href: '/automations',
        label: 'nav.automations',
        icon: Workflow,
        permission: 'automations.read',
        group: 'workspace',
    },
    {
        id: 'channels',
        goKey: 'n',
        href: '/channels',
        label: 'nav.channels',
        icon: Phone,
        permission: 'channels.read',
        group: 'administration',
    },
    {
        id: 'users',
        goKey: 'u',
        href: '/users',
        label: 'nav.users',
        icon: Users,
        permission: 'users.manage',
        group: 'administration',
    },
    {
        id: 'api-keys',
        goKey: 'k',
        href: '/api-keys',
        label: 'nav.apiKeys',
        icon: KeyRound,
        permission: 'api_keys.manage',
        group: 'administration',
    },
    {
        id: 'webhooks',
        goKey: 'w',
        href: '/settings/webhooks',
        label: 'nav.webhooks',
        icon: Webhook,
        permission: 'webhooks.manage',
        group: 'administration',
    },
    {
        id: 'audit',
        goKey: 'l',
        href: '/audit',
        label: 'nav.audit',
        icon: ScrollText,
        permission: 'audit.read',
        group: 'administration',
    },
    {
        id: 'failed-jobs',
        goKey: 'f',
        href: '/system/failed-jobs',
        label: 'nav.failedJobs',
        icon: ServerCrash,
        permission: 'settings.read',
        group: 'administration',
    },
    {
        id: 'settings',
        goKey: 's',
        href: '/settings',
        label: 'nav.settings',
        icon: Settings,
        permission: 'settings.read',
        group: 'administration',
    },
    {
        id: 'profile',
        goKey: 'm',
        href: '/account/profile',
        label: 'nav.profile',
        icon: UserRound,
        group: 'account',
    },
    {
        id: 'security',
        goKey: 'g',
        href: '/account/security',
        label: 'nav.security',
        icon: ShieldCheck,
        group: 'account',
    },
];

/** Items the user may open. While two-factor enrolment is pending only
 * the security page is available. */
export function visibleNav(permissions: string[], restricted: boolean) {
    return NAV.filter(
        (i) =>
            (!i.permission || permissions.includes(i.permission)) &&
            (!restricted || i.id === 'security'),
    );
}

/** The most specific item matching a path (/settings/webhooks is not also
 * /settings). */
export function activeNav(pathname: string): NavItem | undefined {
    return NAV.filter((i) =>
        i.href === '/'
            ? pathname === '/'
            : pathname === i.href || pathname.startsWith(`${i.href}/`),
    ).sort((a, b) => b.href.length - a.href.length)[0];
}
