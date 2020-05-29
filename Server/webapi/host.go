package webapi

import (
	"net/http"
	"os"
	"time"

	"github.com/cdutwhu/n3-util/n3csv"
	eg "github.com/cdutwhu/n3-util/n3errs"
	"github.com/labstack/echo"
	"github.com/labstack/echo-contrib/jaegertracing"
	"github.com/labstack/echo/middleware"
	"github.com/nats-io/nats.go"
	cfg "github.com/nsip/n3-csv2json/Server/config"
)

// HostHTTPAsync : Host a HTTP Server for CSV or JSON
func HostHTTPAsync() {
	e := echo.New()
	defer e.Close()

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.BodyLimit("2G"))

	// Add Jaeger Tracer into Middleware
	c := jaegertracing.New(e, nil)
	defer c.Close()

	// CORS
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{echo.GET, echo.POST},
		AllowCredentials: true,
	}))

	Cfg := env2Struct("Cfg", &cfg.Config{}).(*cfg.Config)
	port := Cfg.WebService.Port
	fullIP := localIP() + fSf(":%d", port)
	route := Cfg.Route
	file := Cfg.File
	mMtx := initMutex(route)

	defer e.Start(fSf(":%d", port))

	// *************************************** List all API, FILE *************************************** //

	path := route.HELP
	e.GET(path, func(c echo.Context) error {
		defer func() { mMtx[path].Unlock() }()
		mMtx[path].Lock()

		return c.String(http.StatusOK,
			fSf("wget %-55s-> %s\n", fullIP+"/client-linux64", "Get Client(Linux64)")+
				fSf("wget %-55s-> %s\n", fullIP+"/client-mac", "Get Client(Mac)")+
				fSf("wget %-55s-> %s\n", fullIP+"/client-win64", "Get Client(Windows64)")+
				fSf("wget -O config.toml %-40s-> %s\n", fullIP+"/client-config", "Get Client Config")+
				fSf("\n")+
				fSf("POST %-55s-> %s\n"+
					"POST %-55s-> %s\n",
					fullIP+route.CSV2JSON, "Upload CSV, return JSON.",
					fullIP+route.JSON2CSV, "Upload JSON, return CSV."))
	})

	// ------------------------------------------------------------------------------------ //

	mRouteRes := map[string]string{
		"/client-linux64": file.ClientLinux64,
		"/client-mac":     file.ClientMac,
		"/client-win64":   file.ClientWin64,
		"/client-config":  file.ClientConfig,
	}

	routeFun := func(rt, res string) func(c echo.Context) error {
		return func(c echo.Context) (err error) {
			if _, err = os.Stat(res); err == nil {
				fPln(rt, res)
				return c.File(res)
			}
			fPf("%v\n", warnOnErr("%v: [%s]  get [%s]", eg.FILE_NOT_FOUND, rt, res))
			return err
		}
	}

	for rt, res := range mRouteRes {
		e.GET(rt, routeFun(rt, res))
	}

	// ------------------------------------------------------------------------------------ //

	path = route.CSV2JSON
	e.POST(path, func(c echo.Context) error {
		defer func() { mMtx[path].Unlock() }()
		mMtx[path].Lock()

		var errSvr error
		pub2nats := false
		if ok, n := url1Value(c.QueryParams(), 0, "nats"); ok && n != "" {
			pub2nats = true
		}

		// jsonstr, headers := n3csv.Reader2JSON(c.Request().Body, "")

		// Trace [n3csv.Reader2JSON]
		results := jaegertracing.TraceFunction(c, n3csv.Reader2JSON, c.Request().Body, "")
		jsonstr := results[0].Interface().(string)
		// headers := results[1].Interface().([]string)

		info := "[n3csv.Reader2JSON]"

		// send a copy to NATS
		if pub2nats {
			url := Cfg.NATS.URL
			subj := Cfg.NATS.Subject
			timeout := time.Duration(Cfg.NATS.Timeout)

			info += fSf(" | To NATS@Subject: [%s@%s]", url, subj)
			nc, err := nats.Connect(url)
			if err != nil {
				errSvr = err
				goto ERR
			}

			msg, err := nc.Request(subj, []byte(jsonstr), timeout*time.Millisecond)
			if msg != nil {
				info += fSf(" | NATS responded: [%s]", string(msg.Data))
			}
			if err != nil {
				errSvr = err
				goto ERR
			}
		}

	ERR:
		if errSvr != nil {
			return c.JSON(http.StatusInternalServerError, result{
				Data:  nil,
				Info:  info,
				Error: errSvr.Error(),
			})
		}

		return c.JSON(http.StatusOK, result{
			Data:  &jsonstr,
			Info:  info,
			Error: "",
		})
	})

	path = route.JSON2CSV
	e.POST(path, func(c echo.Context) error {
		defer func() { mMtx[path].Unlock() }()
		mMtx[path].Lock()

		return c.JSON(http.StatusInternalServerError, result{
			Data:  nil,
			Info:  "Not implemented",
			Error: "Not implemented",
		})
	})
}