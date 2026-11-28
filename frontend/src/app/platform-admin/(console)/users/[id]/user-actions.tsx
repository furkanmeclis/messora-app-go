'use client';

import { useTransition } from 'react';
import { useRouter } from 'next/navigation';
import { KeyRound, UserCheck, UserX } from 'lucide-react';
import { useConfirm } from '@/components/confirm';
import { notify } from '@/components/notify';
import { Button } from '@/components/ui/button';
import { Card, CardContent } from '@/components/ui/card';
import { useT } from '@/i18n/client';
import type { Schemas } from '@/lib/types';
import { userAction } from '../../../actions';

export function UserActions({ user }: { user: Schemas['PlatformUser'] }) {
    const t = useT();
    const router = useRouter();
    const ask = useConfirm();
    const [pending, start] = useTransition();
    const run = (op: 'disable' | 'enable' | 'force-password-reset') =>
        start(async () => {
            if (notify(await userAction(user.id, op))) router.refresh();
        });
    return (
        <Card>
            <CardContent className="grid gap-2">
                {user.status === 'active' ? (
                    <Button
                        variant="destructive"
                        disabled={pending}
                        onClick={async () => {
                            if (
                                await ask({
                                    title: t('platform.user.disableTitle', {
                                        email: user.email,
                                    }),
                                    description: t('platform.user.disableBody'),
                                    confirmLabel: t('platform.user.disable'),
                                    destructive: true,
                                })
                            )
                                run('disable');
                        }}
                    >
                        <UserX /> {t('platform.user.disable')}
                    </Button>
                ) : (
                    <Button
                        variant="outline"
                        disabled={pending}
                        onClick={() => run('enable')}
                    >
                        <UserCheck /> {t('platform.user.enable')}
                    </Button>
                )}
                <Button
                    variant="outline"
                    disabled={pending}
                    onClick={async () => {
                        if (
                            await ask({
                                title: t('platform.user.forceResetTitle'),
                                description: t('platform.user.forceResetBody'),
                                confirmLabel: t('platform.user.forceReset'),
                            })
                        )
                            run('force-password-reset');
                    }}
                >
                    <KeyRound /> {t('platform.user.forceReset')}
                </Button>
            </CardContent>
        </Card>
    );
}
