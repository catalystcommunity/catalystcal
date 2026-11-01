package store

import (
	"context"
	"errors"
	"testing"
)

func TestGroupNestingRules(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	owner, _, _ := s.UpsertUser(ctx, UserInput{Domain: "d", Subject: "owner"})
	other, _, _ := s.UpsertUser(ctx, UserInput{Domain: "d", Subject: "other"})
	pat := Principal{Domain: "D", Subject: "pat"}

	// A chain of MaxGroupDepth groups is allowed; one more is not.
	leaf, err := s.CreateGroup(ctx, Group{OwnerID: owner.ID, Name: "g0"}, []GroupMember{{Principal: pat}})
	if err != nil {
		t.Fatal(err)
	}
	chain := []Group{leaf}
	for i := 1; i < MaxGroupDepth; i++ {
		g, err := s.CreateGroup(ctx, Group{OwnerID: owner.ID, Name: "g" + string(rune('0'+i))}, []GroupMember{{GroupID: chain[i-1].ID}})
		if err != nil {
			t.Fatalf("depth %d: %v", i+1, err)
		}
		chain = append(chain, g)
	}
	if _, err := s.CreateGroup(ctx, Group{OwnerID: owner.ID, Name: "too deep"}, []GroupMember{{GroupID: chain[len(chain)-1].ID}}); !errors.Is(err, ErrGroupDepth) {
		t.Fatalf("too deep: %v", err)
	}
	// Adding below the leaf would make the chain too deep too.
	extra, _ := s.CreateGroup(ctx, Group{OwnerID: owner.ID, Name: "extra"}, nil)
	if _, err := s.SetGroupMembers(ctx, leaf.ID, nil, []GroupMember{{GroupID: extra.ID}}); !errors.Is(err, ErrGroupDepth) {
		t.Fatalf("too deep below: %v", err)
	}
	// A cycle through the chain.
	if _, err := s.SetGroupMembers(ctx, leaf.ID, nil, []GroupMember{{GroupID: chain[3].ID}}); !errors.Is(err, ErrGroupCycle) && !errors.Is(err, ErrGroupDepth) {
		t.Fatalf("cycle: %v", err)
	}
	short, _ := s.CreateGroup(ctx, Group{OwnerID: owner.ID, Name: "a"}, nil)
	shortParent, _ := s.CreateGroup(ctx, Group{OwnerID: owner.ID, Name: "b"}, []GroupMember{{GroupID: short.ID}})
	if _, err := s.SetGroupMembers(ctx, short.ID, nil, []GroupMember{{GroupID: shortParent.ID}}); !errors.Is(err, ErrGroupCycle) {
		t.Fatalf("short cycle: %v", err)
	}
	// Another owner's group cannot be a member.
	foreign, _ := s.CreateGroup(ctx, Group{OwnerID: other.ID, Name: "f"}, nil)
	if _, err := s.SetGroupMembers(ctx, short.ID, nil, []GroupMember{{GroupID: foreign.ID}}); !errors.Is(err, ErrGroupMember) {
		t.Fatalf("foreign: %v", err)
	}

	// pat is in every group of the chain, through nesting.
	got, err := s.GroupsContaining(ctx, Principal{Domain: "d", Subject: "pat"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != MaxGroupDepth {
		t.Fatalf("pat is in %d groups, want %d", len(got), MaxGroupDepth)
	}
	// An application principal of the same account is not pat.
	if got, _ := s.GroupsContaining(ctx, Principal{Domain: "d", Subject: "pat", Application: "app"}); len(got) != 0 {
		t.Fatalf("application matched person: %v", got)
	}

	// Revisions and the parents of a deleted group.
	before, _ := s.GetGroup(ctx, chain[1].ID)
	if err := s.DeleteGroup(ctx, leaf.ID); err != nil {
		t.Fatal(err)
	}
	after, _ := s.GetGroup(ctx, chain[1].ID)
	if after.Revision != before.Revision+1 {
		t.Fatalf("parent revision %d -> %d", before.Revision, after.Revision)
	}
	rev := after.Revision - 1
	if _, err := s.SetGroupMembers(ctx, chain[1].ID, &rev, nil); !errors.Is(err, ErrRevision) {
		t.Fatalf("stale revision: %v", err)
	}
}
