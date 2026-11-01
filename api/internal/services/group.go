package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/catalystcommunity/catalystcal/api/internal/auth"
	"github.com/catalystcommunity/catalystcal/api/internal/csil"
	"github.com/catalystcommunity/catalystcal/api/internal/csilrpc"
	"github.com/catalystcommunity/catalystcal/api/internal/store"
)

// MaxGroupsPerOwner caps how many groups one owner keeps.
const MaxGroupsPerOwner = 1000

// GroupService implements csil.GroupService. Only the owner sees or changes
// a group, with a self bearer. Another caller gets 404, so group ids do not
// leak.
type GroupService struct{ *Deps }

var _ csil.GroupService = (*GroupService)(nil)

// owned loads a group of the caller.
func (s *GroupService) owned(ctx context.Context, id *auth.Identity, groupID string) (store.Group, error) {
	g, err := s.Store.GetGroup(ctx, groupID)
	if errors.Is(err, store.ErrNotFound) || (err == nil && g.OwnerID != id.UserID) {
		return store.Group{}, errNotFound("group")
	}
	if err != nil {
		return store.Group{}, internal(err, "load group")
	}
	csilrpc.SetAuditTarget(ctx, "group:"+g.ID)
	return g, nil
}

// membersIn turns request members into stored members. Identities resolve
// like share grantees: a user_id needs no sign-in, a handle does.
func (s *GroupService) membersIn(ctx context.Context, in []csil.GroupMember) ([]store.GroupMember, error) {
	if len(in) > store.MaxGroupMembers {
		return nil, badReq(fmt.Sprintf("a group can have at most %d members", store.MaxGroupMembers))
	}
	out := make([]store.GroupMember, 0, len(in))
	for i, m := range in {
		switch {
		case m.Identity != nil && m.GroupId != nil:
			return nil, badReq(fmt.Sprintf("members[%d]: give identity or group_id, not both", i))
		case m.GroupId != nil:
			out = append(out, store.GroupMember{GroupID: string(*m.GroupId)})
		case m.Identity != nil:
			p, _, err := s.resolveRef(ctx, *m.Identity)
			if err != nil {
				return nil, err
			}
			out = append(out, store.GroupMember{Principal: p})
		default:
			return nil, badReq(fmt.Sprintf("members[%d]: identity or group_id is required", i))
		}
	}
	return out, nil
}

// groupErr maps the store's group errors.
func groupErr(err error, what string) error {
	switch {
	case errors.Is(err, store.ErrConflict):
		return csilrpc.Conflict("you already have a group with this name")
	case errors.Is(err, store.ErrRevision):
		return csilrpc.Conflict("the group changed after you read it; read it again")
	case errors.Is(err, store.ErrGroupCycle):
		return badReq("a group cannot contain itself, directly or through other groups")
	case errors.Is(err, store.ErrGroupDepth):
		return badReq(fmt.Sprintf("groups can be nested at most %d deep", store.MaxGroupDepth))
	case errors.Is(err, store.ErrGroupMember):
		return errNotFound("member group")
	case errors.Is(err, store.ErrNotFound):
		return errNotFound("group")
	}
	return internal(err, what)
}

// groupOut builds the output form, with each member's handle and person
// when that person signed in here.
func (s *GroupService) groupOut(ctx context.Context, g store.Group) (csil.Group, error) {
	owner, err := s.Store.GetUser(ctx, g.OwnerID)
	if err != nil {
		return csil.Group{}, internal(err, "load group owner")
	}
	members, err := s.Store.ListGroupMembers(ctx, g.ID)
	if err != nil {
		return csil.Group{}, internal(err, "list group members")
	}
	var userIDs, groupIDs []string
	for _, m := range members {
		if m.GroupID != "" {
			groupIDs = append(groupIDs, m.GroupID)
		} else if m.UserID != "" {
			userIDs = append(userIDs, m.UserID)
		}
	}
	users, err := s.Store.GetUsers(ctx, userIDs)
	if err != nil {
		return csil.Group{}, internal(err, "load group member users")
	}
	nested, err := s.Store.GetGroups(ctx, groupIDs)
	if err != nil {
		return csil.Group{}, internal(err, "load nested groups")
	}
	out := csil.Group{
		GroupId:     csil.GroupID(g.ID),
		Owner:       person(owner),
		Name:        g.Name,
		Description: optStr(g.Description),
		Revision:    g.Revision,
		Members:     make([]csil.GroupMember, 0, len(members)),
		CreatedAt:   ts(g.CreatedAt),
		UpdatedAt:   ts(g.UpdatedAt),
	}
	for _, m := range members {
		if m.GroupID != "" {
			gid := csil.GroupID(m.GroupID)
			om := csil.GroupMember{GroupId: &gid}
			if ng, ok := nested[m.GroupID]; ok {
				om.GroupName = ptr(ng.Name)
			}
			out.Members = append(out.Members, om)
			continue
		}
		var found *store.User
		om := csil.GroupMember{}
		if u, ok := users[m.UserID]; ok {
			found = &u
			p := person(u)
			om.Person = &p
		}
		ref := identityRef(m.Principal, found)
		om.Identity = &ref
		out.Members = append(out.Members, om)
	}
	return out, nil
}

// ListGroups returns the caller's groups.
func (s *GroupService) ListGroups(ctx context.Context, _ csil.EmptyRequest) (csil.GroupList, error) {
	id, err := requireSelf(ctx)
	if err != nil {
		return csil.GroupList{}, err
	}
	gs, err := s.Store.ListGroupsByOwner(ctx, id.UserID)
	if err != nil {
		return csil.GroupList{}, internal(err, "list groups")
	}
	out := csil.GroupList{Groups: make([]csil.Group, 0, len(gs))}
	for _, g := range gs {
		og, err := s.groupOut(ctx, g)
		if err != nil {
			return csil.GroupList{}, err
		}
		out.Groups = append(out.Groups, og)
	}
	return out, nil
}

// GetGroup reads one group of the caller.
func (s *GroupService) GetGroup(ctx context.Context, req csil.GroupID) (csil.Group, error) {
	id, err := requireSelf(ctx)
	if err != nil {
		return csil.Group{}, err
	}
	g, err := s.owned(ctx, id, string(req))
	if err != nil {
		return csil.Group{}, err
	}
	return s.groupOut(ctx, g)
}

// CreateGroup makes a group that the caller owns.
func (s *GroupService) CreateGroup(ctx context.Context, req csil.CreateGroupRequest) (csil.Group, error) {
	id, err := requireSelf(ctx)
	if err != nil {
		return csil.Group{}, err
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return csil.Group{}, badReq("name is required")
	}
	n, err := s.Store.CountGroupsByOwner(ctx, id.UserID)
	if err != nil {
		return csil.Group{}, internal(err, "count groups")
	}
	if n >= MaxGroupsPerOwner {
		return csil.Group{}, badReq(fmt.Sprintf("you can have at most %d groups", MaxGroupsPerOwner))
	}
	members, err := s.membersIn(ctx, req.Members)
	if err != nil {
		return csil.Group{}, err
	}
	g, err := s.Store.CreateGroup(ctx, store.Group{
		OwnerID: id.UserID, Name: name, Description: strings.TrimSpace(deref(req.Description)),
	}, members)
	if err != nil {
		return csil.Group{}, groupErr(err, "create group")
	}
	csilrpc.SetAuditTarget(ctx, "group:"+g.ID)
	return s.groupOut(ctx, g)
}

// UpdateGroup changes the name or the description.
func (s *GroupService) UpdateGroup(ctx context.Context, req csil.UpdateGroupRequest) (csil.Group, error) {
	id, err := requireSelf(ctx)
	if err != nil {
		return csil.Group{}, err
	}
	g, err := s.owned(ctx, id, string(req.GroupId))
	if err != nil {
		return csil.Group{}, err
	}
	var patch store.GroupPatch
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return csil.Group{}, badReq("name cannot be empty")
		}
		patch.Name = &name
	}
	if req.Description != nil {
		d := strings.TrimSpace(*req.Description)
		patch.Description = &d
	}
	g, err = s.Store.UpdateGroup(ctx, g.ID, patch)
	if err != nil {
		return csil.Group{}, groupErr(err, "update group")
	}
	return s.groupOut(ctx, g)
}

// SetMembers replaces the member list. A removed member loses the access
// of the group's shares at the next call: access is read at each call.
func (s *GroupService) SetMembers(ctx context.Context, req csil.SetGroupMembersRequest) (csil.Group, error) {
	id, err := requireSelf(ctx)
	if err != nil {
		return csil.Group{}, err
	}
	g, err := s.owned(ctx, id, string(req.GroupId))
	if err != nil {
		return csil.Group{}, err
	}
	members, err := s.membersIn(ctx, req.Members)
	if err != nil {
		return csil.Group{}, err
	}
	g, err = s.Store.SetGroupMembers(ctx, g.ID, req.ExpectedRevision, members)
	if err != nil {
		return csil.Group{}, groupErr(err, "set group members")
	}
	return s.groupOut(ctx, g)
}

// DeleteGroup removes a group, its shares, and its places in other groups.
func (s *GroupService) DeleteGroup(ctx context.Context, req csil.GroupID) (csil.EmptyResponse, error) {
	id, err := requireSelf(ctx)
	if err != nil {
		return csil.EmptyResponse{}, err
	}
	g, err := s.owned(ctx, id, string(req))
	if err != nil {
		return csil.EmptyResponse{}, err
	}
	if err := s.Store.DeleteGroup(ctx, g.ID); err != nil {
		return csil.EmptyResponse{}, groupErr(err, "delete group")
	}
	return csil.EmptyResponse{}, nil
}
