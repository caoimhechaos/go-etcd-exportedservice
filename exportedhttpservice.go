package exportedservice

import (
	"net"
	"net/http"

	"golang.org/x/net/context"
)

/*
ListenAndServeNamedHTTP makes the default HTTP server listen on "addr" and
exports the given "handler". Registers as "servicename".
*/
func (e *ServiceExporter) ListenAndServeNamedHTTP(
	ctx context.Context, servicename, addr string, handler http.Handler) error {
	var l net.Listener
	var err error

	// We can just create a new port as above...
	l, err = e.NewExportedPort(ctx, "tcp", addr, servicename)
	if err != nil {
		return err
	}

	return http.Serve(l, handler)
}
