'use client';

import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { useTheme } from 'next-themes';
import {
    Building2,
    Coins,
    Cpu,
    Gauge,
    LifeBuoy,
    LogOut,
    Monitor,
    Moon,
    ScrollText,
    Settings,
    Sparkles,
    Sun,
    Users,
    type LucideIcon,
} from 'lucide-react';
import { BrandMark } from '@/components/brand';
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
import { useT } from '@/i18n/client';
import type { MessageKey } from '@/i18n/translate';
import type { Schemas } from '@/lib/types';

type Item = { href: string; label: MessageKey; icon: LucideIcon };

const OPERATIONS: Item[] = [
    { href: '/', label: 'platform.nav.overview', icon: Gauge },
    { href: '/tenants', label: 'platform.nav.tenants', icon: Building2 },
    { href: '/users', label: 'platform.nav.users', icon: Users },
    { href: '/support', label: 'platform.nav.support', icon: LifeBuoy },
    { href: '/audit', label: 'platform.nav.audit', icon: ScrollText },
    { href: '/settings', label: 'platform.nav.settings', icon: Settings },
];
const CATALOGUE: Item[] = [
    { href: '/meta-rates', label: 'platform.nav.metaRates', icon: Coins },
    { href: '/ai-prices', label: 'platform.nav.aiPrices', icon: Cpu },
    {
        href: '/tech-provider',
        label: 'platform.nav.techProvider',
        icon: Sparkles,
    },
];

export function PlatformSidebar({
    admin,
}: {
    admin: Schemas['PlatformAdmin'];
}) {
    const t = useT();
    const pathname = usePathname();
    const { theme, setTheme } = useTheme();
    const active = (href: string) =>
        href === '/' ? pathname === '/' : pathname.startsWith(href);
    const group = (label: MessageKey, items: Item[]) => (
        <SidebarGroup>
            <SidebarGroupLabel>{t(label)}</SidebarGroupLabel>
            <SidebarMenu>
                {items.map((i) => (
                    <SidebarMenuItem key={i.href}>
                        <SidebarMenuButton
                            asChild
                            isActive={active(i.href)}
                            tooltip={t(i.label)}
                        >
                            <Link href={i.href}>
                                <i.icon />
                                <span>{t(i.label)}</span>
                            </Link>
                        </SidebarMenuButton>
                    </SidebarMenuItem>
                ))}
            </SidebarMenu>
        </SidebarGroup>
    );
    const next =
        theme === 'light' ? 'dark' : theme === 'dark' ? 'system' : 'light';
    const ThemeIcon =
        theme === 'light' ? Sun : theme === 'dark' ? Moon : Monitor;
    return (
        <Sidebar collapsible="icon">
            <SidebarHeader className="h-14 shrink-0 justify-center border-b border-sidebar-border">
                <div className="flex items-center gap-2.5 px-1 py-0.5">
                    <BrandMark className="size-8 shrink-0" />
                    <div className="grid text-sm leading-tight group-data-[collapsible=icon]:hidden">
                        <span className="truncate font-semibold">Messora</span>
                        <span className="truncate text-xs text-muted-foreground">
                            {t('platform.name')}
                        </span>
                    </div>
                </div>
            </SidebarHeader>
            <SidebarContent>
                {group('platform.nav.operations', OPERATIONS)}
                {group('platform.nav.catalogue', CATALOGUE)}
            </SidebarContent>
            <SidebarFooter>
                <SidebarMenu>
                    <SidebarMenuItem>
                        <SidebarMenuButton
                            onClick={() => setTheme(next)}
                            tooltip={t('common.theme')}
                        >
                            <ThemeIcon />
                            <span>{t('common.theme')}</span>
                        </SidebarMenuButton>
                    </SidebarMenuItem>
                    <SidebarMenuItem>
                        <form action="/logout" method="post">
                            <SidebarMenuButton
                                type="submit"
                                tooltip={t('platform.nav.signOut')}
                            >
                                <LogOut />
                                <span className="truncate">
                                    {t('platform.nav.signOut')} · {admin.name}
                                </span>
                            </SidebarMenuButton>
                        </form>
                    </SidebarMenuItem>
                </SidebarMenu>
            </SidebarFooter>
        </Sidebar>
    );
}
