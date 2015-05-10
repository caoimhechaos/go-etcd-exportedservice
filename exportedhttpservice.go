/*
Exported named etcd HTTP service.
Creates an HTTP server and exports it to etcd.
*/
package exportedservice

import (
	"net"
	"net/http"
)

/*
Make the default HTTP server listen on "addr" and export the given
"handler". Register as "servicename".
*/
func (self *ServiceExporter) ListenAndServeNamedHTTP(
	servicename, addr string, handler http.Handler) error {
	var l net.Listener
	var err error

	// We can just create a new port as above...
	l, err = self.NewExportedPort("tcp", addr, servicename)
	if err != nil {
		return err
	}

	return http.Serve(l, handler)
}
