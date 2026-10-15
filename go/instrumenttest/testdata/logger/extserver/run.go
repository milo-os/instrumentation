package extserver

import (
	"log/slog"
	"os"

	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"
)

func Run() {
	ctrl.SetLogger(logr.FromSlogHandler(slog.NewJSONHandler(os.Stderr, nil)))
}
