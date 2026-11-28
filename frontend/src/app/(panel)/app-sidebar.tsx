'use client';

import Link from 'next/link';
import { usePathname, useRouter } from 'next/navigation';
import { useTransition } from 'react';
import { useTheme } from 'next-themes';
import {
    ArrowLeftRight,
    BookOpen,
    Building2,
    Check,
    ChevronsUpDown,
    Languages,
    LogOut,
    Monitor,
    Moon,
    Sun,
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
    DropdownMenuSub,
    DropdownMenuSubContent,
    DropdownMenuSubTrigger,
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
import { notify } from '@/components/notify';
import { useLocale, useT } from '@/i18n/client';
import { localeNames, locales } from '@/i18n/config';
import { activeNav, visibleNav, type NavItem } from '@/lib/navigation';
import type { Membership } from '@/lib/auth/contracts';
import type { User } from '@/lib/types';
import { switchWorkspaceAction } from '../(auth)/actions';
import { setLanguageAction } from './actions';

function initials(name: string) {
    return name
        .split(/\s+/)
        .map((p) => p[0])
        .slice(0, 2)
        .join('')
        .toUpperCase();
}

/** The organisation logo, or Messora's mark without one. */
export function OrgMark({
    logoVersion,
    className,
}: {
    logoVersion?: string | null;
    className?: string;
}) {
    if (!logoVersion) return <BrandMark className={className} />;
    return (
        // A same-origin image of unknown size; next/image adds nothing here.
        // eslint-disable-next-line @next/next/no-img-element
        <img
            src={`/bff/logo?v=${encodeURIComponent(logoVersion)}`}
            alt=""
            className={`${className ?? ''} rounded-lg object-contain`}
        />
    );
}

export function AppSidebar({
    user,
    permissions,
    organization,
    logoVersion,
    restricted,
    workspaces = [],
}: {
    user: User;
    permissions: string[];
    organization: string;
    logoVersion?: string | null;
    restricted: boolean;
    /** The person's workspaces; with more than one a switcher shows. */
    workspaces?: Membership[];
}) {
    const t = useT();
    const locale = useLocale();
    const router = useRouter();
    const pathname = usePathname();
    const { theme, setTheme } = useTheme();
    const [switching, startSwitch] = useTransition();
    const items = visibleNav(permissions, restricted);
    const active = activeNav(pathname)?.id;

    const group = (key: NavItem['group']) => {
        const v = items.filter((i) => i.group === key);
        if (v.length === 0) return null;
        return (
            <SidebarGroup>
                <SidebarGroupLabel>{t(`nav.${key}`)}</SidebarGroupLabel>
                <SidebarMenu>
                    {v.map((item) => (
                        <SidebarMenuItem key={item.id}>
                            <SidebarMenuButton
                                asChild
                                isActive={item.id === active}
                                tooltip={t(item.label)}
                            >
                                <Link
                                    href={item.href}
                                    // Prefetch on intent, not on sight: the
                                    // sidebar shows ~20 links, and every
                                    // viewport prefetch of a dynamic route
                                    // renders its layouts on the server
                                    // (each calling /me against the
                                    // session's API budget). Hover, focus
                                    // or touch prefetches the one link the
                                    // user is about to open.
                                    prefetch={false}
                                    onMouseEnter={() =>
                                        router.prefetch(item.href)
                                    }
                                    onFocus={() => router.prefetch(item.href)}
                                    onTouchStart={() =>
                                        router.prefetch(item.href)
                                    }
                                >
                                    <item.icon />
                                    <span>{t(item.label)}</span>
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
            {/* As tall as the top bar, so their bottom edges meet. */}
            <SidebarHeader className="h-14 shrink-0 justify-center border-b border-sidebar-border">
                <div className="flex items-center gap-2.5 px-1 py-0.5">
                    <OrgMark
                        logoVersion={logoVersion}
                        className="size-8 shrink-0"
                    />
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
                {group('workspace')}
                {group('administration')}
                {group('account')}
            </SidebarContent>
            <SidebarFooter>
                <SidebarMenu>
                    <SidebarMenuItem>
                        <SidebarMenuButton asChild tooltip={t('apiDocs.link')}>
                            <Link
                                href="/docs/api"
                                target="_blank"
                                prefetch={false}
                            >
                                <BookOpen />
                                <span>{t('apiDocs.link')}</span>
                            </Link>
                        </SidebarMenuButton>
                    </SidebarMenuItem>
                    <SidebarMenuItem>
                        <DropdownMenu>
                            <DropdownMenuTrigger asChild>
                                <SidebarMenuButton size="lg">
                                    <Avatar className="size-8 rounded-lg">
                                        <AvatarFallback className="rounded-lg bg-primary/10 text-primary">
                                            {initials(user.name)}
                                        </AvatarFallback>
                                    </Avatar>
                                    <div className="grid flex-1 text-left text-sm leading-tight">
                                        <span className="truncate font-medium">
                                            {user.name}
                                        </span>
                                        <span className="truncate text-xs text-muted-foreground">
                                            {t(
                                                `settings.security.roles.${user.role.key}`,
                                            )}
                                        </span>
                                    </div>
                                    <ChevronsUpDown className="ml-auto size-4" />
                                </SidebarMenuButton>
                            </DropdownMenuTrigger>
                            <DropdownMenuContent
                                side="top"
                                align="start"
                                className="w-60"
                            >
                                <DropdownMenuLabel className="truncate font-normal">
                                    {user.email}
                                </DropdownMenuLabel>
                                <DropdownMenuSeparator />
                                <DropdownMenuLabel className="text-xs text-muted-foreground">
                                    {t('common.theme')}
                                </DropdownMenuLabel>
                                <DropdownMenuRadioGroup
                                    value={theme}
                                    onValueChange={setTheme}
                                >
                                    <DropdownMenuRadioItem value="light">
                                        <Sun /> {t('common.light')}
                                    </DropdownMenuRadioItem>
                                    <DropdownMenuRadioItem value="dark">
                                        <Moon /> {t('common.dark')}
                                    </DropdownMenuRadioItem>
                                    <DropdownMenuRadioItem value="system">
                                        <Monitor /> {t('common.system')}
                                    </DropdownMenuRadioItem>
                                </DropdownMenuRadioGroup>
                                <DropdownMenuSeparator />
                                <DropdownMenuSub>
                                    <DropdownMenuSubTrigger
                                        disabled={switching}
                                    >
                                        <Languages className="size-4" />
                                        {t('common.language')}
                                        <span className="ml-auto text-xs text-muted-foreground">
                                            {localeNames[locale]}
                                        </span>
                                    </DropdownMenuSubTrigger>
                                    <DropdownMenuSubContent>
                                        {locales.map((l) => (
                                            <DropdownMenuItem
                                                key={l}
                                                onSelect={() =>
                                                    startSwitch(async () => {
                                                        if (
                                                            notify(
                                                                await setLanguageAction(
                                                                    l,
                                                                ),
                                                            )
                                                        )
                                                            router.refresh();
                                                    })
                                                }
                                            >
                                                {localeNames[l]}
                                                {l === locale && (
                                                    <Check className="ml-auto size-4" />
                                                )}
                                            </DropdownMenuItem>
                                        ))}
                                    </DropdownMenuSubContent>
                                </DropdownMenuSub>
                                {workspaces.length > 1 && (
                                    <>
                                        <DropdownMenuSeparator />
                                        <DropdownMenuSub>
                                            <DropdownMenuSubTrigger data-testid="workspace-switcher">
                                                <ArrowLeftRight className="size-4" />
                                                {t('topbar.workspaces')}
                                            </DropdownMenuSubTrigger>
                                            <DropdownMenuSubContent className="w-64">
                                                {workspaces.map((w) => (
                                                    <DropdownMenuItem
                                                        key={w.tenant.id}
                                                        asChild
                                                        disabled={w.current}
                                                    >
                                                        <form
                                                            action={
                                                                switchWorkspaceAction
                                                            }
                                                        >
                                                            <input
                                                                type="hidden"
                                                                name="tenant_id"
                                                                value={
                                                                    w.tenant.id
                                                                }
                                                            />
                                                            <button
                                                                type="submit"
                                                                className="flex w-full items-center gap-2 text-left"
                                                            >
                                                                <Building2 className="size-4" />
                                                                <span className="min-w-0 flex-1 truncate">
                                                                    {
                                                                        w.tenant
                                                                            .name
                                                                    }
                                                                </span>
                                                                {w.current && (
                                                                    <Check className="size-4" />
                                                                )}
                                                            </button>
                                                        </form>
                                                    </DropdownMenuItem>
                                                ))}
                                            </DropdownMenuSubContent>
                                        </DropdownMenuSub>
                                    </>
                                )}
                                <DropdownMenuSeparator />
                                <DropdownMenuItem asChild>
                                    <form action="/logout" method="post">
                                        <button
                                            type="submit"
                                            className="flex w-full items-center gap-2"
                                        >
                                            <LogOut className="size-4" />
                                            {t('common.signOut')}
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
