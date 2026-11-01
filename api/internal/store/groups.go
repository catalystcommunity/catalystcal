package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// Group limits.
const (
	// MaxGroupMembers is the most members one group can have.
	MaxGroupMembers = 5000
	// MaxGroupDepth is the longest chain of nested groups, counted in
	// groups. A deeper chain is refused when it is made.
	MaxGroupDepth = 8
)

// Group errors.
var (
	// ErrRevision means the group changed after the caller read it.
	ErrRevision = errors.New("store: group revision does not match")
	// ErrGroupCycle means a member group contains the group.
	ErrGroupCycle = errors.New("store: a group cannot contain itself")
	// ErrGroupDepth means the nesting is deeper than MaxGroupDepth.
	ErrGroupDepth = errors.New("store: groups are nested too deeply")
	// ErrGroupMember means a member group does not exist or has another
	// owner.
	ErrGroupMember = errors.New("store: a member group must exist and have the same owner")
)

// Group is a list of identities or other groups that one owner keeps.
type Group struct {
	ID          string
	OwnerID     string
	Name        string
	Description string
	Revision    uint64
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// GroupMember is an identity (Principal set) or a group (GroupID set).
type GroupMember struct {
	Principal
	GroupID string
}

// ListedMember is a stored member plus the user id of the identity, when
// that person or application signed in here.
type ListedMember struct {
	GroupMember
	UserID string
}

const groupCols = `id, owner_id, name, description, revision, created_at, updated_at`

func scanGroup(row interface{ Scan(...any) error }) (Group, error) {
	var g Group
	var desc sql.NullString
	var created, updated string
	if err := row.Scan(&g.ID, &g.OwnerID, &g.Name, &desc, &g.Revision, &created, &updated); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Group{}, ErrNotFound
		}
		return Group{}, err
	}
	g.Description = strOf(desc)
	g.CreatedAt, g.UpdatedAt = mustTime(created), mustTime(updated)
	return g, nil
}

// CreateGroup makes a group with its members.
func (s *Store) CreateGroup(ctx context.Context, g Group, members []GroupMember) (Group, error) {
	now := FormatTime(s.now())
	id := NewID()
	err := s.InTx(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `INSERT INTO groups (id, owner_id, name, description, revision, created_at, updated_at)
			VALUES (?, ?, ?, ?, 1, ?, ?)`, id, g.OwnerID, g.Name, nullStr(g.Description), now, now)
		if isUnique(err) {
			return ErrConflict
		}
		if err != nil {
			return err
		}
		return writeMembers(ctx, tx, id, g.OwnerID, members)
	})
	if err != nil {
		return Group{}, err
	}
	return s.GetGroup(ctx, id)
}

// GetGroup reads one group.
func (s *Store) GetGroup(ctx context.Context, id string) (Group, error) {
	return scanGroup(s.DB.QueryRowContext(ctx, `SELECT `+groupCols+` FROM groups WHERE id = ?`, id))
}

// GetGroups reads several groups by id into a map. Unknown ids are skipped.
func (s *Store) GetGroups(ctx context.Context, ids []string) (map[string]Group, error) {
	out := map[string]Group{}
	var args []any
	for _, id := range ids {
		if id != "" {
			args = append(args, id)
		}
	}
	if len(args) == 0 {
		return out, nil
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT `+groupCols+` FROM groups WHERE id IN (`+placeholders(len(args))+`)`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		g, err := scanGroup(rows)
		if err != nil {
			return nil, err
		}
		out[g.ID] = g
	}
	return out, rows.Err()
}

// ListGroupsByOwner returns the groups of one owner, by name.
func (s *Store) ListGroupsByOwner(ctx context.Context, ownerID string) ([]Group, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT `+groupCols+` FROM groups WHERE owner_id = ? ORDER BY name, id`, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Group
	for rows.Next() {
		g, err := scanGroup(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// CountGroupsByOwner returns how many groups one owner has.
func (s *Store) CountGroupsByOwner(ctx context.Context, ownerID string) (int, error) {
	var n int
	err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM groups WHERE owner_id = ?`, ownerID).Scan(&n)
	return n, err
}

// ListGroupMembers returns the members of a group, in their order.
func (s *Store) ListGroupMembers(ctx context.Context, groupID string) ([]ListedMember, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT gm.member_domain, gm.member_subject, gm.member_application, gm.member_group_id, u.id
		FROM group_members gm
		LEFT JOIN users u ON gm.member_group_id IS NULL AND u.domain = gm.member_domain
			AND u.subject = gm.member_subject AND u.application_id = gm.member_application
		WHERE gm.group_id = ? ORDER BY gm.position`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ListedMember
	for rows.Next() {
		var m ListedMember
		var nested, uid sql.NullString
		if err := rows.Scan(&m.Domain, &m.Subject, &m.Application, &nested, &uid); err != nil {
			return nil, err
		}
		m.GroupID, m.UserID = strOf(nested), strOf(uid)
		out = append(out, m)
	}
	return out, rows.Err()
}

// GroupPatch holds the fields UpdateGroup changes. Nil keeps a field.
type GroupPatch struct {
	Name        *string
	Description *string
}

// UpdateGroup changes the name or the description and raises the revision.
func (s *Store) UpdateGroup(ctx context.Context, id string, p GroupPatch) (Group, error) {
	var desc any
	if p.Description != nil {
		desc = nullStr(*p.Description)
	}
	res, err := s.DB.ExecContext(ctx, `UPDATE groups SET
		name = COALESCE(?, name),
		description = CASE WHEN ? THEN ? ELSE description END,
		revision = revision + 1, updated_at = ?
		WHERE id = ?`,
		p.Name, p.Description != nil, desc, FormatTime(s.now()), id)
	if isUnique(err) {
		return Group{}, ErrConflict
	}
	if err != nil {
		return Group{}, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return Group{}, ErrNotFound
	}
	return s.GetGroup(ctx, id)
}

// SetGroupMembers replaces the member list and raises the revision. With
// expected set, a different stored revision gives ErrRevision.
func (s *Store) SetGroupMembers(ctx context.Context, id string, expected *uint64, members []GroupMember) (Group, error) {
	err := s.InTx(ctx, func(tx *sql.Tx) error {
		g, err := scanGroup(tx.QueryRowContext(ctx, `SELECT `+groupCols+` FROM groups WHERE id = ?`, id))
		if err != nil {
			return err
		}
		if expected != nil && *expected != g.Revision {
			return ErrRevision
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM group_members WHERE group_id = ?`, id); err != nil {
			return err
		}
		if err := writeMembers(ctx, tx, id, g.OwnerID, members); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `UPDATE groups SET revision = revision + 1, updated_at = ? WHERE id = ?`,
			FormatTime(s.now()), id)
		return err
	})
	if err != nil {
		return Group{}, err
	}
	return s.GetGroup(ctx, id)
}

// DeleteGroup removes a group. Its shares and its places in other groups
// go with it.
func (s *Store) DeleteGroup(ctx context.Context, id string) error {
	return s.InTx(ctx, func(tx *sql.Tx) error {
		// The parents change, so their revisions go up.
		if _, err := tx.ExecContext(ctx, `UPDATE groups SET revision = revision + 1, updated_at = ?
			WHERE id IN (SELECT group_id FROM group_members WHERE member_group_id = ?)`, FormatTime(s.now()), id); err != nil {
			return err
		}
		res, err := tx.ExecContext(ctx, `DELETE FROM groups WHERE id = ?`, id)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return ErrNotFound
		}
		return nil
	})
}

// writeMembers inserts the members of group id. Duplicates are dropped. A
// member group must exist, have the same owner, and not contain id; the
// nesting must stay inside MaxGroupDepth.
func writeMembers(ctx context.Context, tx *sql.Tx, id, ownerID string, members []GroupMember) error {
	seen := map[GroupMember]bool{}
	pos := 0
	nested := false
	for _, m := range members {
		if m.GroupID == "" {
			m.Principal = m.Principal.Norm()
		} else {
			m.Principal = Principal{}
		}
		if seen[m] {
			continue
		}
		seen[m] = true
		if m.GroupID != "" {
			nested = true
			if m.GroupID == id {
				return ErrGroupCycle
			}
			var owner string
			err := tx.QueryRowContext(ctx, `SELECT owner_id FROM groups WHERE id = ?`, m.GroupID).Scan(&owner)
			if errors.Is(err, sql.ErrNoRows) || (err == nil && owner != ownerID) {
				return ErrGroupMember
			}
			if err != nil {
				return err
			}
			_, err = tx.ExecContext(ctx, `INSERT INTO group_members (group_id, position, member_group_id) VALUES (?, ?, ?)`,
				id, pos, m.GroupID)
			if err != nil {
				return err
			}
		} else {
			_, err := tx.ExecContext(ctx, `INSERT INTO group_members
				(group_id, position, member_domain, member_subject, member_application) VALUES (?, ?, ?, ?, ?)`,
				id, pos, m.Domain, m.Subject, m.Application)
			if err != nil {
				return err
			}
		}
		pos++
	}
	if !nested {
		return nil
	}
	// A cycle: id is now below itself.
	var cycle int
	if err := tx.QueryRowContext(ctx, `WITH RECURSIVE below(id, depth) AS (
			SELECT member_group_id, 1 FROM group_members WHERE group_id = ? AND member_group_id IS NOT NULL
			UNION
			SELECT gm.member_group_id, below.depth + 1 FROM group_members gm JOIN below ON gm.group_id = below.id
			WHERE gm.member_group_id IS NOT NULL AND below.depth <= ?
		) SELECT COUNT(*) FROM below WHERE id = ?`, id, MaxGroupDepth+1, id).Scan(&cycle); err != nil {
		return err
	}
	if cycle > 0 {
		return ErrGroupCycle
	}
	// The longest chain through id: the groups above it, id, and the
	// groups below it.
	var above, below int
	if err := tx.QueryRowContext(ctx, `WITH RECURSIVE up(id, depth) AS (
			SELECT group_id, 1 FROM group_members WHERE member_group_id = ?
			UNION
			SELECT gm.group_id, up.depth + 1 FROM group_members gm JOIN up ON gm.member_group_id = up.id
			WHERE up.depth <= ?
		) SELECT COALESCE(MAX(depth), 0) FROM up`, id, MaxGroupDepth).Scan(&above); err != nil {
		return err
	}
	if err := tx.QueryRowContext(ctx, `WITH RECURSIVE down(id, depth) AS (
			SELECT member_group_id, 1 FROM group_members WHERE group_id = ? AND member_group_id IS NOT NULL
			UNION
			SELECT gm.member_group_id, down.depth + 1 FROM group_members gm JOIN down ON gm.group_id = down.id
			WHERE gm.member_group_id IS NOT NULL AND down.depth <= ?
		) SELECT COALESCE(MAX(depth), 0) FROM down`, id, MaxGroupDepth).Scan(&below); err != nil {
		return err
	}
	if above+1+below > MaxGroupDepth {
		return ErrGroupDepth
	}
	return nil
}

// GroupsContaining returns the ids of every group that holds the identity,
// directly or through nested groups.
func (s *Store) GroupsContaining(ctx context.Context, p Principal) ([]string, error) {
	p = p.Norm()
	rows, err := s.DB.QueryContext(ctx, `WITH RECURSIVE holds(id, depth) AS (
			SELECT group_id, 1 FROM group_members
			WHERE member_group_id IS NULL AND member_domain = ? AND member_subject = ? AND member_application = ?
			UNION
			SELECT gm.group_id, holds.depth + 1 FROM group_members gm JOIN holds ON gm.member_group_id = holds.id
			WHERE holds.depth < ?
		) SELECT DISTINCT id FROM holds ORDER BY id`, p.Domain, p.Subject, p.Application, MaxGroupDepth)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}
