import { SidebarInset, SidebarProvider } from '@/components/ui/sidebar';
import { Separator } from '@/components/ui/separator';
import { SidebarTrigger } from '@/components/ui/sidebar';
import { ConfirmProvider } from '@/components/confirm';
import { DisplayPreferences } from '@/components/display-preferences';
import { getLocale } from '@/i18n/server';
import { requirePlatformMe } from '@/lib/server/platform';
import { PlatformSidebar } from './sidebar';

/** The signed-in part of the platform admin panel (two-factor required). */
export default async function ConsoleLayout({
    children,
}: {
    children: React.ReactNode;
}) {
    const [me, locale] = await Promise.all([requirePlatformMe(), getLocale()]);
    return (
        <SidebarProvider>
            <PlatformSidebar admin={me.admin} />
            <SidebarInset>
                <header className="flex h-14 shrink-0 items-center gap-2 border-b px-4">
                    <SidebarTrigger className="-ml-1" />
                    <Separator orientation="vertical" className="mr-2 h-4" />
                    <span className="text-sm text-muted-foreground">
                        {me.admin.email}
                    </span>
                </header>
                <ConfirmProvider>
                    <DisplayPreferences
                        timeZone="Europe/Istanbul"
                        locale={locale}
                    >
                        <main className="w-full flex-1 p-4 md:p-6 xl:px-8">
                            {children}
                        </main>
                    </DisplayPreferences>
                </ConfirmProvider>
            </SidebarInset>
        </SidebarProvider>
    );
}
