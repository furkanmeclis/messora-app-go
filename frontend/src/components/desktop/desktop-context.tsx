'use client';

import {
    createContext,
    useCallback,
    useContext,
    useEffect,
    useRef,
    useState,
} from 'react';

type Ctx = {
    active: boolean;
    available: boolean;
    enter: () => void;
    exit: () => void;
};

const DesktopContext = createContext<Ctx>({
    active: false,
    available: false,
    enter: () => {},
    exit: () => {},
});

export const DESKTOP_KEY_MESSAGE = 'messora:desktop-key';
const MIN_WIDTH = 1024;

function embedded() {
    return typeof window !== 'undefined' && window.top !== window.self;
}

/**
 * Desktop mode: turned on explicitly (top bar, command palette, Ctrl+Shift+D)
 * and off again when its full screen ends. Never inside a desktop window,
 * and never on small screens.
 */
export function DesktopProvider({ children }: { children: React.ReactNode }) {
    const [active, setActive] = useState(false);
    const [available, setAvailable] = useState(false);
    const auto = useRef(false);

    useEffect(() => {
        if (embedded()) {
            // Inside a window: forward desktop shortcuts to the desktop.
            const onKey = (e: KeyboardEvent) => {
                if (!e.altKey || e.ctrlKey || e.metaKey) return;
                if (/^(Digit[1-9]|KeyW|KeyM|ArrowUp)$/.test(e.code)) {
                    e.preventDefault();
                    window.parent.postMessage(
                        { type: DESKTOP_KEY_MESSAGE, code: e.code },
                        window.location.origin,
                    );
                }
            };
            window.addEventListener('keydown', onKey);
            return () => window.removeEventListener('keydown', onKey);
        }
        // Desktop mode only starts on request; leaving the full screen it
        // opened ends it, and small screens never show it.
        const check = () => {
            const wide = window.innerWidth >= MIN_WIDTH;
            setAvailable(wide);
            setActive((on) => {
                if (!wide && on) return false;
                if (on && auto.current && !document.fullscreenElement) {
                    auto.current = false;
                    return false;
                }
                return on;
            });
        };
        check();
        const onKey = (e: KeyboardEvent) => {
            if (
                (e.ctrlKey || e.metaKey) &&
                e.shiftKey &&
                e.code === 'KeyD' &&
                window.innerWidth >= MIN_WIDTH
            ) {
                e.preventDefault();
                window.dispatchEvent(new Event('messora:desktop-toggle'));
            }
        };
        window.addEventListener('keydown', onKey);
        window.addEventListener('resize', check);
        document.addEventListener('fullscreenchange', check);
        return () => {
            window.removeEventListener('keydown', onKey);
            window.removeEventListener('resize', check);
            document.removeEventListener('fullscreenchange', check);
        };
    }, []);

    const enter = useCallback(() => {
        auto.current = false;
        setActive(true);
        document.documentElement
            .requestFullscreen?.()
            .then(() => (auto.current = true))
            .catch(() => {
                // Full screen refused: desktop mode works in the window too.
            });
    }, []);
    const exit = useCallback(() => {
        auto.current = false;
        setActive(false);
        if (document.fullscreenElement) void document.exitFullscreen();
    }, []);

    useEffect(() => {
        const toggle = () => (active ? exit() : enter());
        window.addEventListener('messora:desktop-toggle', toggle);
        return () =>
            window.removeEventListener('messora:desktop-toggle', toggle);
    }, [active, enter, exit]);

    return (
        <DesktopContext.Provider value={{ active, available, enter, exit }}>
            {children}
        </DesktopContext.Provider>
    );
}

export function useDesktop() {
    return useContext(DesktopContext);
}
