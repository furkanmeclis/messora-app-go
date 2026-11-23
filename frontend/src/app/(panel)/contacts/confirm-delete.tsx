'use client';

import { useTransition } from 'react';
import { Trash2 } from 'lucide-react';
import { notify } from '@/components/notify';
import {
    AlertDialog,
    AlertDialogAction,
    AlertDialogCancel,
    AlertDialogContent,
    AlertDialogDescription,
    AlertDialogFooter,
    AlertDialogHeader,
    AlertDialogTitle,
    AlertDialogTrigger,
} from '@/components/ui/alert-dialog';
import { Button } from '@/components/ui/button';
import type { ActionResult } from '@/lib/types';

/** Icon button that deletes something after a confirmation. */
export function ConfirmDelete({
    label,
    title,
    description,
    confirm,
    action,
    onDone,
}: {
    /** Accessible name of the trigger, e.g. "Delete group VIP". */
    label: string;
    title: string;
    description: string;
    confirm: string;
    action: () => Promise<ActionResult>;
    onDone?: () => void;
}) {
    const [pending, startTransition] = useTransition();
    return (
        <AlertDialog>
            <AlertDialogTrigger asChild>
                <Button
                    variant="ghost"
                    size="icon-sm"
                    aria-label={label}
                    disabled={pending}
                >
                    <Trash2 />
                </Button>
            </AlertDialogTrigger>
            <AlertDialogContent>
                <AlertDialogHeader>
                    <AlertDialogTitle>{title}</AlertDialogTitle>
                    <AlertDialogDescription>
                        {description}
                    </AlertDialogDescription>
                </AlertDialogHeader>
                <AlertDialogFooter>
                    <AlertDialogCancel>Cancel</AlertDialogCancel>
                    <AlertDialogAction
                        variant="destructive"
                        onClick={() =>
                            startTransition(async () => {
                                if (notify(await action())) onDone?.();
                            })
                        }
                    >
                        {confirm}
                    </AlertDialogAction>
                </AlertDialogFooter>
            </AlertDialogContent>
        </AlertDialog>
    );
}
