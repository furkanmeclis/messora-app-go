'use client';

import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { useTheme } from 'next-themes';
import {
    Activity,
    ChevronsUpDown,
    Inbox,
    KeyRound,
    LogOut,
    Monitor,
    Moon,
    ScrollText,
    Settings,
    ShieldCheck,
    Sun,
    UserRound,
    Users,
    Webhook,
} from 'lucide-react';
import { BrandMark } from '@/components/brand';
import { Avatar, AvatarFallback } from '@/components/ui/avatar';
import {
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuLabel,
    DropdownMenuRadioGroup,
    DropdownMenuRadioItem,
    DropdownMenuSeparator,
    DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import {
    Sidebar,
    SidebarContent,
    SidebarFooter,
    SidebarGroup,
    SidebarGroupLabel,
    SidebarHeader,
    SidebarMenu,
    SidebarMenuButton,
    SidebarMenuItem,
} from '@/components/ui/sidebar';
import type { User } from '@/lib/types';

type NavItem = {
    href: string;
    label: string;
    icon: React.ComponentType;
    permission?: string;
};

const WORKSPACE: NavItem[] = [
    { href: '/', label: 'Overview', icon: Activity },
    {
        href: '/inbox',
        label: 'Inbox',
        icon: Inbox,
        permission: 'conversations.read',
    },
    { href: '/users', label: 'Users', icon: Users, permission: 'users.manage' },
    {
        href: '/api-keys',
        label: 'API keys',
        icon: KeyRound,
        permission: 'api_keys.manage',
    },
    {
        href: '/audit',
        label: 'Audit log',
        icon: ScrollText,
        permission: 'audit.read',
    },
    {
        href: '/settings',
        label: 'Settings',
        icon: Settings,
        permission: 'settings.read',
    },
    {
        href: '/settings/webhooks',
        label: 'Webhooks',
        icon: Webhook,
        permission: 'webhooks.manage',
    },
];

const ACCOUNT: NavItem[] = [
    { href: '/account/profile', label: 'Profile', icon: UserRound },
    { href: '/account/security', label: 'Security', icon: ShieldCheck },
];

function initials(name: string) {
    return name
        .split(/\s+/)
        .map((p) => p[0])
        .slice(0, 2)
        .join('')
        .toUpperCase();
}

export function AppSidebar({
    user,
    permissions,
    organization,
    restricted,
}: {
    user: User;
    permissions: string[];
    organization: string;
    restricted: boolean;
}) {
    const pathname = usePathname();
    const { theme, setTheme } = useTheme();
    const visible = (items: NavItem[]) =>
        items.filter(
            (i) =>
                (!i.permission || permissions.includes(i.permission)) &&
                (!restricted || i.href === '/account/security'),
        );
    // The most specific matching item is active (/settings/webhooks is not
    // also /settings).
    const matches = (href: string) =>
        href === '/'
            ? pathname === '/'
            : pathname === href || pathname.startsWith(`${href}/`);
    const activeHref = [...WORKSPACE, ...ACCOUNT]
        .map((i) => i.href)
        .filter(matches)
        .sort((a, b) => b.length - a.length)[0];
    const isActive = (href: string) => href === activeHref;

    const group = (label: string, items: NavItem[]) => {
        const v = visible(items);
        if (v.length === 0) return null;
        return (
            <SidebarGroup>
                <SidebarGroupLabel>{label}</SidebarGroupLabel>
                <SidebarMenu>
                    {v.map((item) => (
                        <SidebarMenuItem key={item.href}>
                            <SidebarMenuButton
                                asChild
                                isActive={isActive(item.href)}
                                tooltip={item.label}
                            >
                                <Link href={item.href}>
                                    <item.icon />
                                    <span>{item.label}</span>
                                </Link>
                            </SidebarMenuButton>
                        </SidebarMenuItem>
                    ))}
                </SidebarMenu>
            </SidebarGroup>
        );
    };

    return (
        <Sidebar collapsible="icon">
            <SidebarHeader>
                <div className="flex items-center gap-2 px-1 py-1.5">
                    <BrandMark className="size-8 shrink-0" />
                    <div className="grid text-sm leading-tight group-data-[collapsible=icon]:hidden">
                        <span className="truncate font-semibold">
                            {organization}
                        </span>
                        <span className="truncate text-xs text-muted-foreground">
                            Messora
                        </span>
                    </div>
                </div>
            </SidebarHeader>
            <SidebarContent>
                {group('Workspace', WORKSPACE)}
                {group('Account', ACCOUNT)}
            </SidebarContent>
            <SidebarFooter>
                <SidebarMenu>
                    <SidebarMenuItem>
                        <DropdownMenu>
                            <DropdownMenuTrigger asChild>
                                <SidebarMenuButton size="lg">
                                    <Avatar className="size-8 rounded-lg">
                                        <AvatarFallback className="rounded-lg">
                                            {initials(user.name)}
                                        </AvatarFallback>
                                    </Avatar>
                                    <div className="grid flex-1 text-left text-sm leading-tight">
                                        <span className="truncate font-medium">
                                            {user.name}
                                        </span>
                                        <span className="truncate text-xs text-muted-foreground">
                                            {user.role.name}
                                        </span>
                                    </div>
                                    <ChevronsUpDown className="ml-auto size-4" />
                                </SidebarMenuButton>
                            </DropdownMenuTrigger>
                            <DropdownMenuContent
                                side="top"
                                align="start"
                                className="w-56"
                            >
                                <DropdownMenuLabel className="truncate font-normal">
                                    {user.email}
                                </DropdownMenuLabel>
                                <DropdownMenuSeparator />
                                <DropdownMenuLabel className="text-xs text-muted-foreground">
                                    Theme
                                </DropdownMenuLabel>
                                <DropdownMenuRadioGroup
                                    value={theme}
                                    onValueChange={setTheme}
                                >
                                    <DropdownMenuRadioItem value="light">
                                        <Sun /> Light
                                    </DropdownMenuRadioItem>
                                    <DropdownMenuRadioItem value="dark">
                                        <Moon /> Dark
                                    </DropdownMenuRadioItem>
                                    <DropdownMenuRadioItem value="system">
                                        <Monitor /> System
                                    </DropdownMenuRadioItem>
                                </DropdownMenuRadioGroup>
                                <DropdownMenuSeparator />
                                <DropdownMenuItem asChild>
                                    <form action="/logout" method="post">
                                        <button
                                            type="submit"
                                            className="flex w-full items-center gap-2"
                                        >
                                            <LogOut className="size-4" /> Sign
                                            out
                                        </button>
                                    </form>
                                </DropdownMenuItem>
                            </DropdownMenuContent>
                        </DropdownMenu>
                    </SidebarMenuItem>
                </SidebarMenu>
            </SidebarFooter>
        </Sidebar>
    );
}
