import type { DeepString } from '../types';
import appearance from './appearance';
import common from './common';
import errors from './errors';
import nav from './nav';
import settings from './settings';
import wizard from './wizard';
import topbar from './topbar';
import account from './account';
import apiKeys from './api-keys';
import audit from './audit';
import campaigns from './campaigns';
import channels from './channels';
import templates from './templates';
import users from './users';
import webhooks from './webhooks';

/**
 * The English dictionary defines the shape every language must match.
 * Add a namespace file here and in ../tr, each typed Messages['name'].
 */
export const en = {
    common,
    nav,
    errors,
    appearance,
    settings,
    wizard,
    topbar,
    account,
    apiKeys,
    audit,
    campaigns,
    channels,
    templates,
    users,
    webhooks,
};

export type Messages = DeepString<typeof en>;
