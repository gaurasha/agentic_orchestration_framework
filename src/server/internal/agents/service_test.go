package agents_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/gaurasha/agentic_orchestration_framework/src/server/internal/agents"
	"github.com/gaurasha/agentic_orchestration_framework/src/server/internal/agents/deps/memory"
)

type clock struct{ t time.Time }

func (c clock) Now() time.Time { return c.t }

type ids struct{ n int }

func (i *ids) NewID() string { i.n++; return fmt.Sprintf("agent-%d", i.n) }

func newAPI() agents.API {
	return agents.New(agents.Deps{
		Store:    memory.NewStore(),
		Registry: memory.Registry{"acme": {"echo"}},
		Clock:    clock{time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)},
		IDs:      &ids{},
	})
}

var def = agents.Definition{Name: "demo", Model: "mock", Tools: []agents.ToolGrant{{Name: "echo", Allowlist: map[string][]string{"city": {"London"}}}}}

// 1.1: creating an agent makes version 1; saving it unchanged makes no new version.
func TestCreateIsV1AndUnchangedSaveIsNoop(t *testing.T) {
	api := newAPI()
	ctx := context.Background()
	a, err := api.Create(ctx, "acme", def)
	if err != nil || a.Version != 1 || a.ID == "" {
		t.Fatalf("Create = %+v, %v; want version 1", a, err)
	}
	same := def
	same.Tools = []agents.ToolGrant{{Name: "echo", Allowlist: map[string][]string{"city": {"London"}}}}
	b, err := api.Update(ctx, "acme", a.ID, same)
	if err != nil || b.Version != 1 || b.Hash != a.Hash {
		t.Fatalf("unchanged Update = %+v, %v; want version 1", b, err)
	}
	vs, _ := api.Versions(ctx, "acme", a.ID)
	if len(vs) != 1 {
		t.Errorf("versions = %d, want 1", len(vs))
	}
}

// 1.2: editing makes the next version and leaves earlier versions unchanged.
func TestEditMakesNextVersion(t *testing.T) {
	api := newAPI()
	ctx := context.Background()
	a, _ := api.Create(ctx, "acme", def)
	edited := def
	edited.Instructions = "be brief"
	b, err := api.Update(ctx, "acme", a.ID, edited)
	if err != nil || b.Version != 2 {
		t.Fatalf("Update = %+v, %v; want version 2", b, err)
	}
	v1, err := api.Get(ctx, "acme", a.ID, 1)
	if err != nil || v1.Instructions != "" || v1.Hash != a.Hash {
		t.Errorf("v1 changed: %+v, %v", v1, err)
	}
	latest, _ := api.Latest(ctx, "acme", a.ID)
	if latest.Version != 2 || latest.Instructions != "be brief" {
		t.Errorf("latest = %+v, want v2", latest)
	}
	list, _ := api.List(ctx, "acme")
	if len(list) != 1 || list[0].Version != 2 {
		t.Errorf("list = %+v, want one agent at v2", list)
	}
}

func TestUnknownToolIsRefused(t *testing.T) {
	api := newAPI()
	bad := def
	bad.Tools = []agents.ToolGrant{{Name: "delete_repo"}}
	if _, err := api.Create(context.Background(), "acme", bad); !errors.Is(err, agents.ErrInvalid) {
		t.Errorf("err = %v, want ErrInvalid", err)
	}
}

func TestOtherTenantSeesNothing(t *testing.T) {
	api := newAPI()
	ctx := context.Background()
	a, _ := api.Create(ctx, "acme", def)
	if _, err := api.Latest(ctx, "globex", a.ID); !errors.Is(err, agents.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
	if list, _ := api.List(ctx, "globex"); len(list) != 0 {
		t.Errorf("list = %+v, want none", list)
	}
}
