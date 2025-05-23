package target

import (
	"context"
	"fmt"

	"github.com/knadh/koanf/v2"
	"github.com/mad-weaver/duck/internal/checks"
	croncheck "github.com/mad-weaver/duck/internal/checks/cron"
	dummycheck "github.com/mad-weaver/duck/internal/checks/dummy"
	filecheck "github.com/mad-weaver/duck/internal/checks/file"
	localstatecheck "github.com/mad-weaver/duck/internal/checks/localstate"
	restcheck "github.com/mad-weaver/duck/internal/checks/rest"
	shellcheck "github.com/mad-weaver/duck/internal/checks/shell"
)

func (t *Target) LoadCheck(ctx context.Context, k *koanf.Koanf) (checks.Check, error) {
	switch k.String("type") {
	case "dummy":
		return dummycheck.NewCheck(ctx, k)
	case "file":
		return filecheck.NewCheck(ctx, k)
	case "cron":
		return croncheck.NewCheck(ctx, k)
	case "localstate":
		return localstatecheck.NewCheck(ctx, k)
	case "shell":
		return shellcheck.NewCheck(ctx, k)
	case "rest":
		return restcheck.NewCheck(ctx, k)
	default:
		return nil, fmt.Errorf("unknown check type: %s", k.String("type"))
	}
}
