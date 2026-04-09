package httpserver

import (
	"log/slog"

	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
	iamtokendual "github.com/cobo/cobo_iam_services/internal/iam/infra/token/dual"
	iamtokenjwt "github.com/cobo/cobo_iam_services/internal/iam/infra/token/jwt"
	iamtokenopaque "github.com/cobo/cobo_iam_services/internal/iam/infra/token/opaque"
	"github.com/cobo/cobo_iam_services/internal/platform/config"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
)

type tokenManager interface {
	iamapp.TokenIssuer
	iamapp.TokenInspector
}

func buildTokenManager(log *slog.Logger, cfg config.Config, id idgen.Generator) tokenManager {
	opaque := iamtokenopaque.NewManager(id)
	mode := cfg.AccessTokenMode
	if mode == "" {
		mode = "opaque"
	}
	switch mode {
	case "jwt":
		log.Info("access token mode: jwt")
		return iamtokenjwt.NewManager(cfg, id)
	case "dual":
		log.Info("access token mode: dual (issue jwt, inspect jwt then opaque)")
		j := iamtokenjwt.NewManager(cfg, id)
		return iamtokendual.NewManager(j, opaque, j)
	default:
		log.Info("access token mode: opaque")
		return opaque
	}
}
