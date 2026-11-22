'use client';

import {
    createContext,
    useActionState,
    useContext,
    useEffect,
    useRef,
    useState,
} from 'react';
import {
    FormStatus,
    SubmitButton,
    TextField,
    fieldError,
} from '@/components/forms';
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogHeader,
    DialogTitle,
} from '@/components/ui/dialog';
import { reauthAction } from '@/app/(panel)/actions';
import type { ActionResult } from '@/lib/types';

type Ctx = {
    /**
     * Runs fn; if the API asks for re-authentication, shows the password
     * dialog and retries once after a successful confirmation.
     */
    guard: <T>(fn: () => Promise<ActionResult<T>>) => Promise<ActionResult<T>>;
};

const ReauthContext = createContext<Ctx | null>(null);

export function useReauth(): Ctx {
    const ctx = useContext(ReauthContext);
    if (!ctx) throw new Error('useReauth outside ReauthProvider');
    return ctx;
}

export function ReauthProvider({
    mfaEnabled,
    children,
}: {
    mfaEnabled: boolean;
    children: React.ReactNode;
}) {
    const [open, setOpen] = useState(false);
    const pending = useRef<((ok: boolean) => void) | null>(null);
    const [state, action] = useActionState<
        ActionResult<string> | null,
        FormData
    >(reauthAction, null);

    useEffect(() => {
        if (state?.ok && pending.current) {
            pending.current(true);
            pending.current = null;
            // Closing the dialog right after the confirmed action settles.
            queueMicrotask(() => setOpen(false));
        }
    }, [state]);

    const guard: Ctx['guard'] = async (fn) => {
        const first = await fn();
        if (first.ok || first.code !== 'REAUTH_REQUIRED') return first;
        const confirmed = await new Promise<boolean>((resolve) => {
            pending.current = resolve;
            setOpen(true);
        });
        return confirmed ? fn() : first;
    };

    return (
        <ReauthContext.Provider value={{ guard }}>
            {children}
            <Dialog
                open={open}
                onOpenChange={(o) => {
                    setOpen(o);
                    if (!o && pending.current) {
                        pending.current(false);
                        pending.current = null;
                    }
                }}
            >
                <DialogContent>
                    <DialogHeader>
                        <DialogTitle>Confirm it is you</DialogTitle>
                        <DialogDescription>
                            This action needs a fresh confirmation. It stays
                            valid for 10 minutes.
                        </DialogDescription>
                    </DialogHeader>
                    <form action={action} className="space-y-4">
                        <FormStatus state={state && !state.ok ? state : null} />
                        <TextField
                            label="Password"
                            name="password"
                            type="password"
                            autoComplete="current-password"
                            autoFocus
                            error={fieldError(state, 'password')}
                        />
                        {mfaEnabled && (
                            <TextField
                                label="…or authenticator code"
                                name="code"
                                inputMode="numeric"
                                maxLength={6}
                                pattern="\d{6}"
                                autoComplete="one-time-code"
                            />
                        )}
                        <SubmitButton className="w-full">Confirm</SubmitButton>
                    </form>
                </DialogContent>
            </Dialog>
        </ReauthContext.Provider>
    );
}
