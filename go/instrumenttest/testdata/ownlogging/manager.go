package manager

import (
	"example.com/nso/internal/logging"
	ctrl "sigs.k8s.io/controller-runtime"
)

func Run() { ctrl.SetLogger(logging.New(nil)) }
