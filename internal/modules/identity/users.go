package identity

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/furkanmeclis/messora-app-go/internal/api"
	"github.com/furkanmeclis/messora-app-go/internal/db"
	"github.com/furkanmeclis/messora-app-go/internal/modules/audit"
	"github.com/furkanmeclis/messora-app-go/internal/platform/crypto"
	"github.com/furkanmeclis/messora-app-go/internal/platform/entitlements"
	"github.com/furkanmeclis/messora-app-go/internal/platform/mailer/templates"
	"github.com/furkanmeclis/messora-app-go/internal/platform/tenant"
	"github.com/furkanmeclis/messora-app-go/internal/shared/apperr"
	"github.com/furkanmeclis/messora-app-go/internal/shared/principal"
)

// roleRank mirrors the seeded roles; lower is more privileged.
var roleRank = map[string]int{"owner": 0, "admin": 10, "manager": 20, "agent": 30}

// canAssign: owners may grant any role; everyone else only roles strictly
// less privileged than their own.
func canAssign(actor *principal.Principal, role string) bool {
	r, ok := roleRank[role]
	if !ok {
		return false
	}
	return actor.RoleKey == "owner" || r > actor.RoleRank
}

// ListUsers returns every member of the workspace with their role.
func (s *Service) ListUsers(ctx context.Context) ([]api.User, error) {
	rows, err := s.q.ListUsersWithRole(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]api.User, 0, len(rows))
	for _, r := range rows {
		u := db.User{ID: r.ID, Email: r.Email, Name: r.Name, Status: r.Status, TotpConfirmedAt: r.TotpConfirmedAt,
			Locale: r.Locale, Timezone: r.Timezone, LastLoginAt: r.LastLoginAt, CreatedAt: r.CreatedAt}
		out = append(out, toAPIUser(u, r.RoleKey, r.RoleName))
	}
	return out, nil
}

// UpdateUser changes another member's role and/or membership status (the
// account itself stays as it is: it may belong to other workspaces),
// enforcing: no self-modification, no acting on more privileged users,
// owner-only owner grants, and never fewer than one active owner.
func (s *Service) UpdateUser(ctx context.Context, actor *principal.Principal, id uuid.UUID, req api.UpdateUserRequest) (api.User, error) {
	if req.Role == nil && req.Status == nil {
		return api.User{}, apperr.Validation("role", "Nothing to change")
	}
	if id == actor.ID {
		return api.User{}, apperr.Forbidden("SELF_MODIFICATION", "You cannot change your own role or status")
	}
	var out api.User
	err := s.tx(ctx, func(q *db.Queries) error {
		if err := q.LockOwnerChanges(ctx); err != nil {
			return err
		}
		target, err := q.GetUserByID(ctx, id)
		if isNoRows(err) {
			return apperr.NotFound("User not found")
		}
		if err != nil {
			return err
		}
		cur, err := q.GetMembershipForUpdate(ctx, id)
		if isNoRows(err) {
			return apperr.NotFound("User not found")
		}
		if err != nil {
			return err
		}
		if actor.RoleKey != "owner" && int(cur.RoleRank) <= actor.RoleRank {
			return apperr.Forbidden("INSUFFICIENT_ROLE", "You cannot manage users with an equal or higher role")
		}
		changes := map[string]any{}
		newRole := cur.RoleKey
		if req.Role != nil && string(*req.Role) != cur.RoleKey {
			newRole = string(*req.Role)
			if !canAssign(actor, newRole) {
				return apperr.Forbidden("INSUFFICIENT_ROLE", "You cannot grant this role")
			}
			if err := q.ReplaceRole(ctx, db.ReplaceRoleParams{UserID: id, RoleKey: newRole}); err != nil {
				return err
			}
			changes["role"] = []string{cur.RoleKey, newRole}
		}
		newStatus := cur.Status
		if req.Status != nil && string(*req.Status) != cur.Status {
			newStatus = string(*req.Status)
			if err := q.SetMembershipStatus(ctx, db.SetMembershipStatusParams{UserID: id, Status: newStatus}); err != nil {
				return err
			}
			if newStatus == "disabled" {
				if _, err := q.RevokeUserSessions(ctx, db.RevokeUserSessionsParams{UserID: id}); err != nil {
					return err
				}
			}
			changes["status"] = []string{cur.Status, newStatus}
		}
		if cur.RoleKey == "owner" && (newRole != "owner" || newStatus != "active") {
			n, err := q.CountActiveOwners(ctx)
			if err != nil {
				return err
			}
			if n < 1 {
				return apperr.Conflict("LAST_OWNER", "The workspace must keep at least one active owner")
			}
		}
		if len(changes) > 0 {
			if err := audit.Record(ctx, q, audit.Entry{Action: "user.update", EntityType: "user", EntityID: uuidPtr(id), Changes: changes}); err != nil {
				return err
			}
		}
		if target.Status == "active" {
			target.Status = newStatus
		}
		roleName := strings.ToUpper(newRole[:1]) + newRole[1:]
		out = toAPIUser(target, newRole, roleName)
		return nil
	})
	return out, err
}

// ── Invitations ────────────────────────────────────────────────────────────

// CreateInvitation invites email with role. The address may already have
// an account (a person in another workspace): accepting then adds a
// membership instead of creating one. The acceptance link is returned once
// and e-mailed when e-mail is configured.
func (s *Service) CreateInvitation(ctx context.Context, actor *principal.Principal, req api.CreateInvitationRequest) (api.InvitationCreated, error) {
	email, err := normalizeEmail("email", string(req.Email))
	if err != nil {
		return api.InvitationCreated{}, err
	}
	role := string(req.Role)
	if !canAssign(actor, role) {
		return api.InvitationCreated{}, apperr.Forbidden("INSUFFICIENT_ROLE", "You cannot invite users with this role")
	}
	return s.invite(ctx, actor, email, role)
}

// invite creates an invitation into the connection's workspace (also used
// by the platform admin panel for owners) and e-mails it.
func (s *Service) invite(ctx context.Context, actor *principal.Principal, email, role string) (api.InvitationCreated, error) {
	token, err := crypto.NewToken(inviteTokenPrefix)
	if err != nil {
		return api.InvitationCreated{}, err
	}
	var invitedBy *uuid.UUID
	inviter := ""
	if actor != nil {
		invitedBy, inviter = uuidPtr(actor.ID), actor.Name
	}
	var row db.CreateInvitationRow
	err = s.tx(ctx, func(q *db.Queries) error {
		if u, err := q.GetUserByEmail(ctx, email); err == nil {
			if _, err := q.GetUserRole(ctx, u.ID); err == nil {
				return apperr.Conflict("USER_EXISTS", "This person is already a member of this workspace")
			} else if !isNoRows(err) {
				return err
			}
		} else if !isNoRows(err) {
			return err
		}
		if err := q.ExpireStaleInvitation(ctx, email); err != nil {
			return err
		}
		// Seats: active members and open invitations count against the
		// plan's users. The platform's owner invitation is not capped.
		if actor != nil {
			if err := entitlements.Check(ctx, q, s.plans, entitlements.Users, func() (int64, error) { return q.CountSeatsForPlan(ctx) }); err != nil {
				return err
			}
		}
		row, err = q.CreateInvitation(ctx, db.CreateInvitationParams{
			Email: email, RoleKey: role, TokenHash: crypto.HashToken(token), InvitedBy: invitedBy, ExpiresAt: s.now().Add(invitationTTL),
		})
		if err != nil {
			if strings.Contains(err.Error(), "user_invitations_open_email_idx") {
				return apperr.Conflict("INVITATION_EXISTS", "This address already has a pending invitation")
			}
			return err
		}
		return audit.RecordInTenant(ctx, q, audit.Entry{Action: "invitation.create", EntityType: "invitation", EntityID: uuidPtr(row.ID),
			Changes: map[string]any{"email": email, "role": role}})
	})
	if err != nil {
		return api.InvitationCreated{}, err
	}
	w, err := s.workspaceOf(ctx, tenant.ID(ctx))
	if err != nil {
		return api.InvitationCreated{}, err
	}
	link := w.link("/invite", "token", token)
	// The invitee's language is unknown: an existing account has one,
	// otherwise the workspace's default.
	locale := "en"
	if st, err := s.settings.Get(ctx); err == nil {
		locale = st.DefaultLocale
	}
	if u, err := s.q.GetUserByEmail(ctx, email); err == nil {
		locale = u.Locale
	}
	if inviter == "" {
		inviter = "Messora"
	}
	sent := s.sendTo(ctx, w, email, locale, templates.Invitation, templates.InvitationData{
		Common: w.common(""), InviterName: inviter, WorkspaceName: w.Name, Role: roleLabel(role, locale),
		AcceptURL: link, ExpiresIn: invitationTTL,
	})
	var by *string
	if actor != nil {
		by = &inviter
	}
	return api.InvitationCreated{
		Invitation: api.Invitation{Id: row.ID, Email: row.Email, Role: roleRef(role), InvitedBy: by, CreatedAt: row.CreatedAt, ExpiresAt: row.ExpiresAt, Status: api.InvitationStatusPending},
		AcceptUrl:  link,
		EmailSent:  sent,
	}, nil
}

// InviteOwner invites an owner into workspace tenantID on behalf of a
// platform admin (actor is nil: the invitation reads "from Messora").
func (s *Service) InviteOwner(ctx context.Context, tenantID uuid.UUID, email string) (api.InvitationCreated, error) {
	email, err := normalizeEmail("email", email)
	if err != nil {
		return api.InvitationCreated{}, err
	}
	return s.invite(tenant.WithID(ctx, tenantID), nil, email, "owner")
}

func roleRef(key string) api.RoleRef {
	return api.RoleRef{Key: api.RoleKey(key), Name: strings.ToUpper(key[:1]) + key[1:]}
}

// ListInvitations returns open invitations (pending or expired).
func (s *Service) ListInvitations(ctx context.Context) ([]api.Invitation, error) {
	rows, err := s.q.ListOpenInvitations(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]api.Invitation, 0, len(rows))
	for _, r := range rows {
		status := api.InvitationStatusPending
		if !s.now().Before(r.ExpiresAt) {
			status = api.InvitationStatusExpired
		}
		out = append(out, api.Invitation{Id: r.ID, Email: r.Email, Role: api.RoleRef{Key: api.RoleKey(r.RoleKey), Name: r.RoleName},
			InvitedBy: r.InvitedByName, CreatedAt: r.CreatedAt, ExpiresAt: r.ExpiresAt, Status: status})
	}
	return out, nil
}

// RevokeInvitation cancels a pending invitation.
func (s *Service) RevokeInvitation(ctx context.Context, id uuid.UUID) error {
	return s.tx(ctx, func(q *db.Queries) error {
		n, err := q.RevokeInvitation(ctx, id)
		if err != nil {
			return err
		}
		if n == 0 {
			return apperr.NotFound("Invitation not found")
		}
		return audit.Record(ctx, q, audit.Entry{Action: "invitation.revoke", EntityType: "invitation", EntityID: uuidPtr(id)})
	})
}

var errInvitationInvalid = apperr.Forbidden("INVITATION_INVALID", "This invitation is invalid, expired or already used")

// LookupInvitation shows whom a token invites, to which workspace and by
// whom, and whether the address already has an account.
func (s *Service) LookupInvitation(ctx context.Context, token string) (api.InvitationPreview, error) {
	if !strings.HasPrefix(token, inviteTokenPrefix) {
		return api.InvitationPreview{}, errInvitationInvalid
	}
	r, err := s.q.GetOpenInvitationByTokenHash(ctx, crypto.HashToken(token))
	if isNoRows(err) {
		return api.InvitationPreview{}, errInvitationInvalid
	}
	if err != nil {
		return api.InvitationPreview{}, err
	}
	out := api.InvitationPreview{Email: r.Email, Role: api.RoleRef{Key: api.RoleKey(r.RoleKey), Name: r.RoleName}, ExpiresAt: r.ExpiresAt}
	if _, err := s.q.GetUserByEmail(ctx, r.Email); err == nil {
		out.AccountExists = true
	} else if !isNoRows(err) {
		return api.InvitationPreview{}, err
	}
	if r.InvitedByName != nil {
		out.InvitedBy = &struct {
			Name string `json:"name"`
		}{Name: *r.InvitedByName}
	}
	if w, err := s.workspaceOf(ctx, tenant.ID(ctx)); err == nil {
		t := &struct {
			LogoUrl *string `json:"logo_url"`
			Name    string  `json:"name"`
		}{Name: w.Name}
		if w.LogoURL != "" {
			logo := "/bff/logo"
			t.LogoUrl = &logo
		}
		out.Tenant = t
	}
	return out, nil
}

// AcceptInvitation adds the invited person to the workspace and signs them
// in: a new account with name and password, or, when the address already
// has an account, after checking that account's password.
func (s *Service) AcceptInvitation(ctx context.Context, req api.AcceptInvitationRequest) (api.SessionCreated, error) {
	if !strings.HasPrefix(req.Token, inviteTokenPrefix) {
		return api.SessionCreated{}, errInvitationInvalid
	}
	inv, err := s.q.GetOpenInvitationByTokenHash(ctx, crypto.HashToken(req.Token))
	if isNoRows(err) {
		return api.SessionCreated{}, errInvitationInvalid
	}
	if err != nil {
		return api.SessionCreated{}, err
	}
	existing, err := s.q.GetUserByEmail(ctx, inv.Email)
	switch {
	case err == nil:
		// The account's own password proves it is them; the shared
		// lockout and rate limits apply.
		if _, err := s.checkPassword(ctx, inv.Email, req.Password); err != nil {
			return api.SessionCreated{}, err
		}
	case isNoRows(err):
		if req.Name == nil || strings.TrimSpace(*req.Name) == "" {
			return api.SessionCreated{}, apperr.Validation("name", "Enter your name")
		}
		if err := validatePassword("password", req.Password, inv.Email); err != nil {
			return api.SessionCreated{}, err
		}
	default:
		return api.SessionCreated{}, err
	}
	var (
		out   api.SessionCreated
		alert *newSignIn
	)
	err = s.tx(ctx, func(q *db.Queries) error {
		n, err := q.AcceptInvitation(ctx, inv.ID)
		if err != nil {
			return err
		}
		if n == 0 {
			return errInvitationInvalid // accepted concurrently
		}
		// The invitation held a seat; after a plan change there may be no
		// seat left for it (the active members count).
		if err := entitlements.Check(ctx, q, s.plans, entitlements.Users, func() (int64, error) { return q.CountActiveMembersForPlan(ctx) }); err != nil {
			return err
		}
		u := existing
		if u.ID == uuid.Nil {
			hash, err := crypto.HashPassword(req.Password, s.o.Argon2)
			if err != nil {
				return err
			}
			st, err := s.settings.Get(ctx)
			if err != nil {
				return err
			}
			// Following the e-mailed link proves the address.
			now := s.now()
			u, err = q.CreateUser(ctx, db.CreateUserParams{Email: inv.Email, Name: strings.TrimSpace(*req.Name), PasswordHash: hash,
				Locale: st.DefaultLocale, Timezone: st.DefaultTimezone, EmailVerifiedAt: &now})
			if err != nil {
				if strings.Contains(err.Error(), "users_email_key") {
					return apperr.Conflict("USER_EXISTS", "An account with this e-mail address already exists")
				}
				return err
			}
		} else if u.EmailVerifiedAt == nil {
			if _, err := q.MarkEmailVerified(ctx, u.ID); err != nil {
				return err
			}
		}
		if err := q.CreateMembership(ctx, db.CreateMembershipParams{UserID: u.ID, RoleKey: inv.RoleKey, InvitedBy: inv.InvitedBy}); err != nil {
			if strings.Contains(err.Error(), "tenant_memberships_pkey") {
				return apperr.Conflict("USER_EXISTS", "You are already a member of this workspace")
			}
			return err
		}
		if err := audit.Record(ctx, q, audit.Entry{Action: "invitation.accept", EntityType: "user", EntityID: uuidPtr(u.ID),
			Actor:   &principal.Principal{Kind: principal.KindUser, ID: u.ID, Name: u.Name},
			Changes: map[string]any{"invitation_id": inv.ID, "role": inv.RoleKey, "existing_account": existing.ID != uuid.Nil}}); err != nil {
			return err
		}
		if u, err = q.GetUserByID(ctx, u.ID); err != nil {
			return err
		}
		out, alert, err = s.createSession(ctx, q, u)
		return err
	})
	if err == nil {
		s.alertNewSignIn(ctx, alert)
	}
	return out, err
}

// ── Roles ──────────────────────────────────────────────────────────────────

// ListRoles returns every role with its permissions plus the permission catalogue.
func (s *Service) ListRoles(ctx context.Context) (api.RolesAndPermissions, error) {
	roles, err := s.q.ListRoles(ctx)
	if err != nil {
		return api.RolesAndPermissions{}, err
	}
	rp, err := s.q.ListRolePermissions(ctx)
	if err != nil {
		return api.RolesAndPermissions{}, err
	}
	perms, err := s.q.ListPermissions(ctx)
	if err != nil {
		return api.RolesAndPermissions{}, err
	}
	byRole := map[string][]api.Permission{}
	for _, r := range rp {
		byRole[r.RoleKey] = append(byRole[r.RoleKey], r.PermissionKey)
	}
	out := api.RolesAndPermissions{Roles: []api.Role{}, Permissions: []api.PermissionInfo{}}
	for _, r := range roles {
		p := byRole[r.Key]
		if p == nil {
			p = []api.Permission{}
		}
		out.Roles = append(out.Roles, api.Role{Key: api.RoleKey(r.Key), Name: r.Name, Rank: int(r.Rank), Permissions: p})
	}
	for _, p := range perms {
		out.Permissions = append(out.Permissions, api.PermissionInfo{Key: p.Key, Module: p.Module, Description: p.Description})
	}
	return out, nil
}

// ── API keys ───────────────────────────────────────────────────────────────

func toAPIKey(k db.ApiKey, createdBy *string) api.ApiKey {
	scopes := make([]api.Permission, len(k.Scopes))
	copy(scopes, k.Scopes)
	return api.ApiKey{Id: k.ID, Name: k.Name, Prefix: k.Prefix, Scopes: scopes, CreatedBy: createdBy,
		CreatedAt: k.CreatedAt, LastUsedAt: k.LastUsedAt, ExpiresAt: k.ExpiresAt}
}

// CreateAPIKey issues a key whose scopes cannot exceed the creator's own
// permissions. The secret is returned once.
func (s *Service) CreateAPIKey(ctx context.Context, actor *principal.Principal, req api.CreateApiKeyRequest) (api.ApiKeyCreated, error) {
	if err := s.requireReauth(actor); err != nil {
		return api.ApiKeyCreated{}, err
	}
	catalogue, err := s.q.ListPermissions(ctx)
	if err != nil {
		return api.ApiKeyCreated{}, err
	}
	known := map[string]bool{}
	for _, p := range catalogue {
		known[p.Key] = true
	}
	scopes := make([]string, 0, len(req.Scopes))
	for _, sc := range req.Scopes {
		if !known[sc] {
			return api.ApiKeyCreated{}, apperr.Validation("scopes", fmt.Sprintf("Unknown scope %q", sc))
		}
		if !actor.Has(sc) {
			return api.ApiKeyCreated{}, apperr.Forbidden("SCOPE_NOT_ALLOWED", fmt.Sprintf("You do not have the %q permission yourself", sc))
		}
		scopes = append(scopes, sc)
	}
	slices.Sort(scopes)
	secret, err := crypto.NewToken(APIKeyPrefix)
	if err != nil {
		return api.ApiKeyCreated{}, err
	}
	var expires *time.Time
	if req.ExpiresInDays != nil {
		t := s.now().Add(time.Duration(*req.ExpiresInDays) * 24 * time.Hour)
		expires = &t
	}
	var k db.ApiKey
	err = s.tx(ctx, func(q *db.Queries) error {
		k, err = q.CreateApiKey(ctx, db.CreateApiKeyParams{
			Name: strings.TrimSpace(req.Name), Prefix: secret[:len(APIKeyPrefix)+6], KeyHash: crypto.HashToken(secret),
			Scopes: scopes, CreatedBy: uuidPtr(actor.ID), ExpiresAt: expires,
		})
		if err != nil {
			return err
		}
		return audit.Record(ctx, q, audit.Entry{Action: "api_key.create", EntityType: "api_key", EntityID: uuidPtr(k.ID), Changes: map[string]any{"name": k.Name, "scopes": scopes}})
	})
	if err != nil {
		return api.ApiKeyCreated{}, err
	}
	name := actor.Name
	return api.ApiKeyCreated{ApiKey: toAPIKey(k, &name), Secret: secret}, nil
}

// ListAPIKeys returns active keys.
func (s *Service) ListAPIKeys(ctx context.Context) ([]api.ApiKey, error) {
	rows, err := s.q.ListApiKeys(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]api.ApiKey, 0, len(rows))
	for _, r := range rows {
		k := db.ApiKey{ID: r.ID, Name: r.Name, Prefix: r.Prefix, Scopes: r.Scopes, CreatedAt: r.CreatedAt, LastUsedAt: r.LastUsedAt, ExpiresAt: r.ExpiresAt}
		out = append(out, toAPIKey(k, r.CreatedByName))
	}
	return out, nil
}

// RevokeAPIKey disables a key immediately.
func (s *Service) RevokeAPIKey(ctx context.Context, id uuid.UUID) error {
	return s.tx(ctx, func(q *db.Queries) error {
		name, err := q.RevokeApiKey(ctx, id)
		if isNoRows(err) {
			return apperr.NotFound("API key not found")
		}
		if err != nil {
			return err
		}
		return audit.Record(ctx, q, audit.Entry{Action: "api_key.revoke", EntityType: "api_key", EntityID: uuidPtr(id), Changes: map[string]any{"name": name}})
	})
}

// APIKeySelf describes the calling key.
func (s *Service) APIKeySelf(ctx context.Context, p *principal.Principal) (api.ApiKey, error) {
	scopes := make([]api.Permission, len(p.Permissions))
	copy(scopes, p.Permissions)
	k := api.ApiKey{Id: p.ID, Name: p.Name, Scopes: scopes}
	rows, err := s.q.ListApiKeys(ctx)
	if err != nil {
		return k, err
	}
	for _, r := range rows {
		if r.ID == p.ID {
			return toAPIKey(db.ApiKey{ID: r.ID, Name: r.Name, Prefix: r.Prefix, Scopes: r.Scopes, CreatedAt: r.CreatedAt, LastUsedAt: r.LastUsedAt, ExpiresAt: r.ExpiresAt}, r.CreatedByName), nil
		}
	}
	return k, apperr.NotFound("API key not found")
}
