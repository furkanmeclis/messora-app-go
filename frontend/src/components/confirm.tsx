'use client';

import * as React from 'react';
import {
    AlertDialog,
    AlertDialogAction,
    AlertDialogCancel,
    AlertDialogContent,
    AlertDialogDescription,
    AlertDialogFooter,
    AlertDialogHeader,
    AlertDialogTitle,
} from '@/components/ui/alert-dialog';
import { useT } from '@/i18n/client';

type Options = {
    title?: string;
    description?: React.ReactNode;
    confirmLabel?: string;
    destructive?: boolean;
};

const ConfirmContext = React.createContext<
    ((o?: Options) => Promise<boolean>) | null
>(null);

/** Replaces window.confirm with the panel's own dialog. */
export function ConfirmProvider({ children }: { children: React.ReactNode }) {
    const t = useT();
    const [state, setState] = React.useState<
        (Options & { resolve: (ok: boolean) => void }) | null
    >(null);
    const confirm = React.useCallback(
        (o: Options = {}) =>
            new Promise<boolean>((resolve) => setState({ ...o, resolve })),
        [],
    );
    const close = (ok: boolean) => {
        state?.resolve(ok);
        setState(null);
    };
    return (
        <ConfirmContext.Provider value={confirm}>
            {children}
            <AlertDialog
                open={!!state}
                onOpenChange={(o) => !o && close(false)}
            >
                <AlertDialogContent>
                    <AlertDialogHeader>
                        <AlertDialogTitle>
                            {state?.title ?? t('common.areYouSure')}
                        </AlertDialogTitle>
                        <AlertDialogDescription>
                            {state?.description ?? t('common.cannotBeUndone')}
                        </AlertDialogDescription>
                    </AlertDialogHeader>
                    <AlertDialogFooter>
                        <AlertDialogCancel onClick={() => close(false)}>
                            {t('common.cancel')}
                        </AlertDialogCancel>
                        <AlertDialogAction
                            variant={
                                state?.destructive ? 'destructive' : 'default'
                            }
                            onClick={() => close(true)}
                        >
                            {state?.confirmLabel ?? t('common.confirm')}
                        </AlertDialogAction>
                    </AlertDialogFooter>
                </AlertDialogContent>
            </AlertDialog>
        </ConfirmContext.Provider>
    );
}

/** `if (await confirm({ destructive: true })) …` */
export function useConfirm() {
    const c = React.useContext(ConfirmContext);
    if (!c) throw new Error('useConfirm outside ConfirmProvider');
    return c;
}
