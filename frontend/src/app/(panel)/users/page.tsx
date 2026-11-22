import type { Metadata } from 'next';
import {
    Card,
    CardContent,
    CardDescription,
    CardHeader,
    CardTitle,
} from '@/components/ui/card';
import { api } from '@/lib/server/api';
import { requirePermission } from '@/lib/server/auth';
import type { Invitation, RolesAndPermissions, User } from '@/lib/types';
import { PageHeading } from '../panel-header';
import { InvitationList, InviteDialog, UserTable } from './users-client';

export const metadata: Metadata = { title: 'Users' };

const RANK: Record<string, number> = {
    owner: 0,
    admin: 10,
    manager: 20,
    agent: 30,
};

export default async function UsersPage() {
    const me = await requirePermission('users.manage');
    const [users, invitations, roles] = await Promise.all([
        api<User[]>('/users').then((r) => r.data),
        api<Invitation[]>('/invitations').then((r) => r.data),
        api<RolesAndPermissions>('/roles').then((r) => r.data),
    ]);
    const myRank = RANK[me.user.role.key] ?? 99;
    // Owners may grant any role; others only less privileged roles.
    const assignable = roles.roles
        .filter((r) => me.user.role.key === 'owner' || r.rank > myRank)
        .map((r) => ({ key: r.key, name: r.name }));

    return (
        <>
            <PageHeading
                title="Users"
                description="People who can sign in to this workspace."
                actions={<InviteDialog roles={assignable} />}
            />
            <div className="space-y-6">
                <Card>
                    <CardContent className="p-0">
                        <UserTable
                            users={users}
                            meId={me.user.id}
                            meIsOwner={me.user.role.key === 'owner'}
                            myRank={myRank}
                            roles={assignable}
                        />
                    </CardContent>
                </Card>
                {invitations.length > 0 && (
                    <Card>
                        <CardHeader>
                            <CardTitle>Pending invitations</CardTitle>
                            <CardDescription>
                                Invitations expire after 7 days.
                            </CardDescription>
                        </CardHeader>
                        <CardContent>
                            <InvitationList invitations={invitations} />
                        </CardContent>
                    </Card>
                )}
                <Card>
                    <CardHeader>
                        <CardTitle>Roles</CardTitle>
                        <CardDescription>
                            What each role is allowed to do.
                        </CardDescription>
                    </CardHeader>
                    <CardContent className="space-y-3 text-sm">
                        {roles.roles.map((r) => (
                            <div key={r.key}>
                                <span className="font-medium">{r.name}</span>
                                <span className="text-muted-foreground">
                                    {' '}
                                    — {r.permissions.length} of{' '}
                                    {roles.permissions.length} permissions
                                </span>
                            </div>
                        ))}
                    </CardContent>
                </Card>
            </div>
        </>
    );
}
