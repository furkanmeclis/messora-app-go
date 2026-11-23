'use client';

import { useState, useTransition } from 'react';
import { useRouter } from 'next/navigation';
import { NumberField } from '@/components/ui/number-field';
import { notify } from '@/components/notify';
import { useT } from '@/i18n/client';
import type { RoleKey, Settings } from '@/lib/types';
import { saveSettingsAction } from '../actions';
import { MfaRolesField } from '../fields';
import { LockedBadge } from '../locked';
import { Row, Section } from '../section';

export function SecurityForm({
    settings,
    readOnly,
}: {
    settings: Settings;
    readOnly: boolean;
}) {
    const t = useT();
    const router = useRouter();
    const [saving, start] = useTransition();
    const [roles, setRoles] = useState<RoleKey[]>(settings.mfa_required_roles);
    const [savedRoles, setSavedRoles] = useState(settings.mfa_required_roles);
    const [idle, setIdle] = useState(settings.session_idle_hours);
    const [life, setLife] = useState(settings.session_lifetime_days);
    const [savedSessions, setSavedSessions] = useState({
        idle: settings.session_idle_hours,
        life: settings.session_lifetime_days,
    });
    const rolesLocked = settings.locked.includes('mfa_required_roles');
    const rolesDirty =
        [...roles].sort().join() !== [...savedRoles].sort().join();
    const sessionsDirty =
        idle !== savedSessions.idle || life !== savedSessions.life;

    const save = (patch: Record<string, unknown>) =>
        start(async () => {
            const r = await saveSettingsAction(patch);
            if (notify(r)) {
                setSavedRoles(r.data.mfa_required_roles);
                setRoles(r.data.mfa_required_roles);
                setSavedSessions({
                    idle: r.data.session_idle_hours,
                    life: r.data.session_lifetime_days,
                });
                router.refresh();
            }
        });

    return (
        <div className="grid items-start gap-6 xl:grid-cols-2">
            <Section
                title={t('settings.security.twoFactor')}
                description={t('settings.security.twoFactorHint')}
                readOnly={readOnly || rolesLocked}
                dirty={rolesDirty}
                saving={saving}
                onSave={() => save({ mfa_required_roles: roles })}
                onReset={() => setRoles(savedRoles)}
            >
                <Row
                    label={t('settings.security.requiredFor')}
                    badge={
                        rolesLocked ? (
                            <LockedBadge field="mfa_required_roles" />
                        ) : null
                    }
                    hint={
                        roles.length === 0
                            ? t('settings.security.noneRequired')
                            : undefined
                    }
                >
                    <MfaRolesField
                        value={roles}
                        onChange={setRoles}
                        disabled={readOnly || rolesLocked}
                    />
                </Row>
            </Section>
            <Section
                title={t('settings.security.sessions')}
                readOnly={readOnly}
                dirty={sessionsDirty}
                saving={saving}
                onSave={() =>
                    save({
                        session_idle_hours: idle,
                        session_lifetime_days: life,
                    })
                }
                onReset={() => {
                    setIdle(savedSessions.idle);
                    setLife(savedSessions.life);
                }}
            >
                <div className="grid gap-6 sm:grid-cols-2">
                    <Row label={t('settings.security.idleHours')}>
                        <NumberField
                            value={idle}
                            onValueChange={(n) => setIdle(n ?? 1)}
                            min={1}
                            max={720}
                            disabled={readOnly}
                            aria-label={t('settings.security.idleHours')}
                        />
                    </Row>
                    <Row label={t('settings.security.lifetimeDays')}>
                        <NumberField
                            value={life}
                            onValueChange={(n) => setLife(n ?? 1)}
                            min={1}
                            max={90}
                            disabled={readOnly}
                            aria-label={t('settings.security.lifetimeDays')}
                        />
                    </Row>
                </div>
            </Section>
        </div>
    );
}
