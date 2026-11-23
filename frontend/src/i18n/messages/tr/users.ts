import type { Messages } from '../en';

const users: Messages['users'] = {
    title: 'Kullanıcılar',
    description: 'Bu çalışma alanına giriş yapabilen kişiler.',
    pendingInvitations: 'Bekleyen davetler',
    invitationsExpire: 'Davetlerin süresi 7 gün sonra dolar.',
    roles: 'Roller',
    rolesHint: 'Her rolün neler yapabildiği.',
    permissionCount: '— {total} yetkiden {count} tanesi',
    columns: {
        name: 'Ad',
        role: 'Rol',
        twoFactor: '2FA',
        lastSignIn: 'Son giriş',
        status: 'Durum',
    },
    roleOf: '{name} kullanıcısının rolü',
    on: 'Açık',
    off: 'Kapalı',
    disable: 'Devre dışı bırak',
    enable: 'Etkinleştir',
    you: 'Siz',
    statuses: { active: 'etkin', disabled: 'devre dışı' },
    updated: 'Kullanıcı güncellendi.',
    invite: {
        button: 'Kullanıcı davet et',
        title: 'Kullanıcı davet edin',
        description: 'Hesabını oluşturması için bir bağlantı gönderilir.',
        emailed:
            '{email} adresine davet e-postası gönderdik. Bu bağlantıyı siz de paylaşabilirsiniz:',
        noEmail:
            'E-posta yapılandırılmamış. Bu bağlantıyı {email} ile paylaşın — yalnızca bir kez gösterilir:',
        copyLink: 'Bağlantıyı kopyala',
        linkCopied: 'Bağlantı kopyalandı',
        done: 'Tamam',
        email: 'E-posta',
        role: 'Rol',
        send: 'Davet gönder',
    },
    invitation: {
        details: '{role} · davet eden: {by} · son geçerlilik: {expires}',
        unknown: 'bilinmiyor',
        expired: 'Süresi doldu',
        revoke: 'İptal et',
    },
    invitationRevoked: 'Davet iptal edildi.',
};
export default users;
