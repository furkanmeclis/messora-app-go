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

/** Full screen, whether by the Fullscreen API or the browser's F11. */
function fullscreen() {
    return (
        !!document.fullscreenElement ||
        window.matchMedia('(display-mode: fullscreen)').matches ||
        (window.innerHeight >= screen.height - 1 &&
            window.innerWidth >= screen.width - 1)
    );
}

/**
 * Desktop mode: turned on by going full screen (F11) or explicitly, and
 * off again when full screen ends. Never inside a desktop window, and
 * never on small screens.
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
        const check = () => {
            const wide = window.innerWidth >= MIN_WIDTH;
            setAvailable(wide);
            const fs = fullscreen();
            setActive((on) => {
                if (fs && wide && !on) {
                    auto.current = true;
                    return true;
                }
                if (!fs && on && auto.current) {
                    auto.current = false;
                    return false;
                }
                if (!wide && on) return false;
                return on;
            });
        };
        check();
        window.addEventListener('resize', check);
        document.addEventListener('fullscreenchange', check);
        return () => {
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

    return (
        <DesktopContext.Provider value={{ active, available, enter, exit }}>
            {children}
        </DesktopContext.Provider>
    );
}

export function useDesktop() {
    return useContext(DesktopContext);
}
