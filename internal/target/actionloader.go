package target

import (
	"context"
	"fmt"

	"github.com/knadh/koanf/v2"
	"github.com/mad-weaver/duck/internal/actions"
	dummyaction "github.com/mad-weaver/duck/internal/actions/dummy"
	localstateaction "github.com/mad-weaver/duck/internal/actions/localstate"
	printaction "github.com/mad-weaver/duck/internal/actions/print"
	restaction "github.com/mad-weaver/duck/internal/actions/rest"
	shellaction "github.com/mad-weaver/duck/internal/actions/shell"
	sleepaction "github.com/mad-weaver/duck/internal/actions/sleep"
	templateaction "github.com/mad-weaver/duck/internal/actions/template"
)

func (t *Target) LoadAction(ctx context.Context, k *koanf.Koanf) (actions.Action, error) {
	switch k.String("type") {
	case "dummy":
		return dummyaction.NewAction(ctx, k)
	case "print":
		return printaction.NewAction(ctx, k)
	case "sleep":
		return sleepaction.NewAction(ctx, k)
	case "localstate":
		return localstateaction.NewAction(ctx, k)
	case "rest":
		return restaction.NewAction(ctx, k)
	case "shell":
		return shellaction.NewAction(ctx, k)
	case "template":
		return templateaction.NewAction(ctx, k)
	default:
		return nil, fmt.Errorf("unknown action type: %s", k.String("type"))
	}
}
