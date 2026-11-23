import type { Messages } from '../en';

const templates: Messages['templates'] = {
    statuses: {
        APPROVED: 'Onaylandı',
        PENDING: 'Onay bekliyor',
        REJECTED: 'Reddedildi',
        PAUSED: 'Duraklatıldı',
        DISABLED: 'Devre dışı',
    },
    categories: {
        MARKETING: 'Pazarlama',
        UTILITY: 'Hizmet',
        AUTHENTICATION: 'Kimlik doğrulama',
    },
};
export default templates;
