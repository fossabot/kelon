package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/Foundato/kelon/configs"
	"github.com/Foundato/kelon/pkg/api"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

type restProxy struct {
	pathPrefix string
	port       int32
	configured bool
	appConf    *configs.AppConfig
	config     *api.ClientProxyConfig
	router     *mux.Router
	server     *http.Server

	telemetryHandler    http.Handler
	telemetryMiddleware func(handler http.Handler) http.Handler
}

// Implements api.ClientProxy by providing OPA's Data-REST-API.
func NewRestProxy(pathPrefix string, port int32) api.ClientProxy {
	return &restProxy{
		pathPrefix: pathPrefix,
		port:       port,
		configured: false,
		appConf:    nil,
		config:     nil,
		router:     mux.NewRouter(),
	}
}

// See Configure() of api.ClientProxy
func (proxy *restProxy) Configure(appConf *configs.AppConfig, serverConf *api.ClientProxyConfig) error {
	// Exit if already configured
	if proxy.configured {
		return nil
	}

	// Configure subcomponents
	if serverConf.Compiler == nil {
		return errors.New("RestProxy: Compiler not configured! ")
	}
	compiler := *serverConf.Compiler
	if err := compiler.Configure(appConf, &serverConf.PolicyCompilerConfig); err != nil {
		return err
	}

	// Configure telemetry (if set)
	if appConf.TelemetryProvider != nil {
		if telemetryHandler, handlerErr := appConf.TelemetryProvider.GetHTTPMetricsHandler(); handlerErr == nil {
			proxy.telemetryHandler = telemetryHandler
		}

		telemetryMiddleware, middErr := appConf.TelemetryProvider.GetHTTPMiddleware()
		if middErr != nil {
			return errors.Wrap(middErr, "RestProxy was configured with TelemetryProvider that does not implement 'GetHTTPMiddleware()' correctly.")
		}
		proxy.telemetryMiddleware = telemetryMiddleware
	}

	// Assign variables
	proxy.appConf = appConf
	proxy.config = serverConf
	proxy.configured = true
	log.Infoln("Configured RestProxy")
	return nil
}

// See Start() of api.ClientProxy
func (proxy *restProxy) Start() error {
	if !proxy.configured {
		err := errors.New("RestProxy was not configured! Please call Configure(). ")
		proxy.handleErrorMetrics(err)
		return err
	}

	// Endpoints to validate queries
	proxy.router.PathPrefix(proxy.pathPrefix + "/data").Handler(proxy.applyHandlerMiddlewareIfSet(proxy.handleV1DataGet)).Methods("GET")
	proxy.router.PathPrefix(proxy.pathPrefix + "/data").Handler(proxy.applyHandlerMiddlewareIfSet(proxy.handleV1DataPost)).Methods("POST")

	// Endpoints to update policies and data
	proxy.router.PathPrefix(proxy.pathPrefix + "/data").Handler(proxy.applyHandlerMiddlewareIfSet(proxy.handleV1DataPut)).Methods("PUT")
	proxy.router.PathPrefix(proxy.pathPrefix + "/data").Handler(proxy.applyHandlerMiddlewareIfSet(proxy.handleV1DataPatch)).Methods("PATCH")
	proxy.router.PathPrefix(proxy.pathPrefix + "/data").Handler(proxy.applyHandlerMiddlewareIfSet(proxy.handleV1DataDelete)).Methods("DELETE")
	proxy.router.PathPrefix(proxy.pathPrefix + "/policies").Handler(proxy.applyHandlerMiddlewareIfSet(proxy.handleV1PolicyPut)).Methods("PUT")
	proxy.router.PathPrefix(proxy.pathPrefix + "/policies").Handler(proxy.applyHandlerMiddlewareIfSet(proxy.handleV1PolicyDelete)).Methods("DELETE")
	if proxy.telemetryHandler != nil {
		log.Infoln("Registered /metrics endpoint")
		proxy.router.PathPrefix("/metrics").Handler(proxy.telemetryHandler)
	}
	proxy.router.PathPrefix("/health").Methods("GET").HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write([]byte("{\"status\": \"healthy\"}"))
	})

	proxy.server = &http.Server{
		Handler:      proxy.router,
		Addr:         fmt.Sprintf(":%d", proxy.port),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	// Start Server
	go func() {
		log.Infof("Starting server at: http://0.0.0.0:%d%s", proxy.port, proxy.pathPrefix)
		if err := proxy.server.ListenAndServe(); err != nil {
			proxy.handleErrorMetrics(err)
			log.Fatal(err)
		}
	}()
	return nil
}

func (proxy restProxy) applyHandlerMiddlewareIfSet(handlerFunc func(http.ResponseWriter, *http.Request)) http.Handler {
	if proxy.telemetryMiddleware != nil {
		return proxy.telemetryMiddleware(http.HandlerFunc(handlerFunc))
	} else {
		return http.HandlerFunc(handlerFunc)
	}
}

func (proxy restProxy) handleErrorMetrics(err error) {
	if proxy.appConf.TelemetryProvider != nil {
		proxy.appConf.TelemetryProvider.CheckError(err)
	}
}

// See Stop() of api.ClientProxy
func (proxy *restProxy) Stop(deadline time.Duration) error {
	if proxy.server == nil {
		return errors.New("RestProxy has not bin started yet")
	}

	log.Infof("Stopping server at: http://localhost:%d%s", proxy.port, proxy.pathPrefix)
	ctx, cancel := context.WithTimeout(context.Background(), deadline)
	defer cancel()
	if err := proxy.server.Shutdown(ctx); err != nil {
		proxy.handleErrorMetrics(err)
		return errors.Wrap(err, "Error while shutting down server")
	}
	return nil
}
