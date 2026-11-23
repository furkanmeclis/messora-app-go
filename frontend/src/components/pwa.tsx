'use client';

import {
    createContext,
    useCallback,
    useContext,
    useEffect,
    useState,
} from 'react';

type InstallEvent = Event & {
    prompt: () => Promise<void>;
    userChoice: Promise<{ outcome: string }>;
};
type Ctx = {
    canInstall: boolean;
    installed: boolean;
    install: () => Promise<void>;
};
const PwaContext = createContext<Ctx>({
    canInstall: false,
    installed: false,
    install: async () => {},
});

/**
 * Registers the service worker when the app is enabled in Settings → PWA
 * (and removes it when disabled), and keeps the browser's install prompt
 * for an "Install app" button.
 */
export function PwaProvider({
    enabled,
    children,
}: {
    enabled: boolean;
    children: React.ReactNode;
}) {
    const [prompt, setPrompt] = useState<InstallEvent | null>(null);
    const [installed, setInstalled] = useState(false);

    useEffect(() => {
        if (!('serviceWorker' in navigator) || window.top !== window.self)
            return;
        if (enabled) {
            navigator.serviceWorker
                .register('/sw.js', { scope: '/' })
                .catch(() => {
                    // not available (e.g. plain HTTP outside localhost)
                });
        } else {
            void navigator.serviceWorker
                .getRegistrations()
                .then((rs) => rs.forEach((r) => void r.unregister()));
        }
        const onPrompt = (e: Event) => {
            e.preventDefault();
            setPrompt(e as InstallEvent);
        };
        const onInstalled = () => {
            setInstalled(true);
            setPrompt(null);
        };
        window.addEventListener('beforeinstallprompt', onPrompt);
        window.addEventListener('appinstalled', onInstalled);
        // eslint-disable-next-line react-hooks/set-state-in-effect -- browser state known only after hydration
        setInstalled(window.matchMedia('(display-mode: standalone)').matches);
        return () => {
            window.removeEventListener('beforeinstallprompt', onPrompt);
            window.removeEventListener('appinstalled', onInstalled);
        };
    }, [enabled]);

    const install = useCallback(async () => {
        if (!prompt) return;
        await prompt.prompt();
        await prompt.userChoice;
        setPrompt(null);
    }, [prompt]);

    return (
        <PwaContext.Provider
            value={{ canInstall: enabled && !!prompt, installed, install }}
        >
            {children}
        </PwaContext.Provider>
    );
}

export function usePwa() {
    return useContext(PwaContext);
}
