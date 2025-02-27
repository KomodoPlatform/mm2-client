package mm2_tools_server

import (
	"fmt"
	"github.com/kpango/glg"
	"github.com/ulule/limiter/v3"
	mfasthttp "github.com/ulule/limiter/v3/drivers/middleware/fasthttp"
	"github.com/ulule/limiter/v3/drivers/store/memory"
	"github.com/valyala/fasthttp"
	"runtime"
)

var gAppName = ""

func LaunchServer(appName string, onlyPriceService bool) {
	if runtime.GOOS == "ios" {
		glg.Get().SetMode(glg.STD)
		glg.Info("Launch MM2 Tools Server from ios")
	}

	if runtime.GOOS == "android" {
		glg.Get().SetMode(glg.STD)
		glg.Info("Launch MM2 Tools Server from android")
	}

	gAppName = appName
	router := InitRooter(onlyPriceService)
	rate, err := limiter.NewRateFromFormatted("10-S")
	if err != nil {
		glg.Fatalf("error on limiter: %v", err)
		return
	}

	store := memory.NewStore()
	glg.Info("Memory store created")

	// Create a fasthttp middleware.
	middleware := mfasthttp.NewMiddleware(limiter.New(store, rate, limiter.WithTrustForwardHeader(true)))
	glg.Info("Middleware created")

    glg.Fatal(fasthttp.ListenAndServe(":"+fmt.Sprintf("%d", 13579), middleware.Handle(router.Handler)))
}
