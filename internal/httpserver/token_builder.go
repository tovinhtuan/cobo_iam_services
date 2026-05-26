package httpserver

import (
	"log/slog"

	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
	iamtokendual "github.com/cobo/cobo_iam_services/internal/iam/infra/token/dual"
	iamtokenjwt "github.com/cobo/cobo_iam_services/internal/iam/infra/token/jwt"
	iamtokenopaque "github.com/cobo/cobo_iam_services/internal/iam/infra/token/opaque"
	iamtokensessionbound "github.com/cobo/cobo_iam_services/internal/iam/infra/token/sessionbound"
	"github.com/cobo/cobo_iam_services/internal/platform/config"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
)

type tokenManager interface {
	iamapp.TokenIssuer
	iamapp.TokenInspector
}

// TokenManager is exported for optional dependency injection in tests.
type TokenManager = tokenManager

func buildTokenManager(log *slog.Logger, cfg config.Config, id idgen.Generator, sessions iamapp.SessionRepository) tokenManager {
	opaque := iamtokenopaque.NewManager(id)
	mode := cfg.AccessTokenMode
	if mode == "" {
		mode = "opaque"
	}
	var mgr tokenManager = opaque
	switch mode {
	case "jwt":
		log.Info("access token mode: jwt")
		mgr = iamtokenjwt.NewManager(cfg, id, opaque)
	case "dual":
		log.Info("access token mode: dual (issue jwt, inspect jwt then opaque)")
		j := iamtokenjwt.NewManager(cfg, id, opaque)
		mgr = iamtokendual.NewManager(j, opaque, j)
	default:
		log.Info("access token mode: opaque")
	}
	if sessions != nil {
		log.Info("access token inspect bound to session store (revoked/expired sessions rejected)")
		return iamtokensessionbound.New(mgr, sessions)
	}
	return mgr
}
