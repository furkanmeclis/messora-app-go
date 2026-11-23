import type { components } from './api/schema';

/** Shapes generated from api/openapi.yaml (`npm run api:types`). */
export type Schemas = components['schemas'];

export type ApiErrorBody = Schemas['ErrorBody'];
export type User = Schemas['User'];
export type Me = Schemas['Me'];
export type SessionInfo = Schemas['SessionInfo'];
export type Invitation = Schemas['Invitation'];
export type Role = Schemas['Role'];
export type RoleKey = Schemas['RoleKey'];
export type RolesAndPermissions = Schemas['RolesAndPermissions'];
export type Settings = Schemas['Settings'];
export type AuditLog = Schemas['AuditLog'];
export type ApiKey = Schemas['ApiKey'];
export type SystemStatus = Schemas['SystemStatus'];
export type Conversation = Schemas['Conversation'];
export type InboxMessage = Schemas['InboxMessage'];
export type ConversationEvent = Schemas['ConversationEvent'];
export type Template = Schemas['Template'];
export type Contact = Schemas['Contact'];
export type Note = Schemas['Note'];

/** Result of a Server Action: either success data or a displayable error. */
export type ActionResult<T = undefined> =
    | { ok: true; data: T; message?: string }
    | {
          ok: false;
          code: string;
          message: string;
          fieldErrors?: Record<string, string>;
      };

// ── Contacts, tags, groups, segments and imports ───────────────────────────
export type Tag = Schemas['Tag'];
export type TagRef = Schemas['TagRef'];
export type ContactGroup = Schemas['Group'];
export type Segment = Schemas['Segment'];
export type SegmentPreview = Schemas['SegmentPreview'];
export type ContactImport = Schemas['ContactImport'];
