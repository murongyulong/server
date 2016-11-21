package g

import (
	"github.com/smartcaas/common/model"
	"log"
	"runtime"
)

func init() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
}

var (
	RealState = model.NewSafeRealState()
)
