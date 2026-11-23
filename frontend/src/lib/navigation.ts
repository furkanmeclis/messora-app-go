import {
    Activity,
    BookUser,
    FileText,
    Inbox,
    KeyRound,
    Megaphone,
    Phone,
    ScrollText,
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
};

/**
 * Every section of the panel. The sidebar and desktop mode both render
 * this list, filtered by the user's permissions.
 */
export const NAV: NavItem[] = [
    {
        id: 'overview',
        href: '/',
        label: 'nav.overview',
        icon: Activity,
        group: 'workspace',
    },
    {
        id: 'inbox',
        href: '/inbox',
        label: 'nav.inbox',
        icon: Inbox,
        permission: 'conversations.read',
        group: 'workspace',
    },
    {
        id: 'contacts',
        href: '/contacts',
        label: 'nav.contacts',
        icon: BookUser,
        permission: 'contacts.read',
        group: 'workspace',
    },
    {
        id: 'campaigns',
        href: '/campaigns',
        label: 'nav.campaigns',
        icon: Megaphone,
        permission: 'campaigns.read',
        group: 'workspace',
    },
    {
        id: 'templates',
        href: '/templates',
        label: 'nav.templates',
        icon: FileText,
        permission: 'templates.read',
        group: 'workspace',
    },
    {
        id: 'automations',
        href: '/automations',
        label: 'nav.automations',
        icon: Workflow,
        permission: 'automations.read',
        group: 'workspace',
    },
    {
        id: 'channels',
        href: '/channels',
        label: 'nav.channels',
        icon: Phone,
        permission: 'channels.read',
        group: 'administration',
    },
    {
        id: 'users',
        href: '/users',
        label: 'nav.users',
        icon: Users,
        permission: 'users.manage',
        group: 'administration',
    },
    {
        id: 'api-keys',
        href: '/api-keys',
        label: 'nav.apiKeys',
        icon: KeyRound,
        permission: 'api_keys.manage',
        group: 'administration',
    },
    {
        id: 'webhooks',
        href: '/settings/webhooks',
        label: 'nav.webhooks',
        icon: Webhook,
        permission: 'webhooks.manage',
        group: 'administration',
    },
    {
        id: 'audit',
        href: '/audit',
        label: 'nav.audit',
        icon: ScrollText,
        permission: 'audit.read',
        group: 'administration',
    },
    {
        id: 'settings',
        href: '/settings',
        label: 'nav.settings',
        icon: Settings,
        permission: 'settings.read',
        group: 'administration',
    },
    {
        id: 'profile',
        href: '/account/profile',
        label: 'nav.profile',
        icon: UserRound,
        group: 'account',
    },
    {
        id: 'security',
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
