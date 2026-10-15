package bypass

import (
	"github.com/go-logr/zapr"
	"go.miloapis.com/instrumentation/logging"
	"go.uber.org/zap"
	crzap "sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
	a    = crzap.New()
	b, _ = zap.NewProduction()
	c    = zap.NewDevelopmentConfig()
	d    = zapr.NewLogger(logging.NewRaw(nil))
)
