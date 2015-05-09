/**
 * Exported named Doozer port.
 * This binds to an anonymous port, exports the host:port pair through Doozer
 * and returns the port to the caller.
 */
package exportedservice

import (
	"crypto/tls"
	"fmt"
	"github.com/coreos/go-etcd/etcd"
	"net"
)

// We need to initialize our Doozer client beforehand and keep it somewhere.
type ServiceExporter struct {
	conn *etcd.Client
	path string
}

/**
 * Try to create a new exporter by connecting to etcd.
 */
func NewExporter(servers []string) *ServiceExporter {
	var self *ServiceExporter = &ServiceExporter{}

	self.conn = etcd.NewClient(servers)

	return self
}

/**
 * Try to create a new exporter by connecting to etcd via TLS.
 */
func NewTLSExporter(servers []string, cert, key, ca string) (*ServiceExporter, error) {
	var self *ServiceExporter = &ServiceExporter{}
	var err error

	self.conn, err = etcd.NewTLSClient(servers, cert, key, ca)

	return self, err
}

/**
 * Open a new anonymous port on "ip" and export it through Doozer as
 * "servicename". If "ip" is a host:port pair, the port will be overridden.
 */
func (self *ServiceExporter) NewExportedPort(
	network, ip, servicename string) (net.Listener, error) {
	var path string = fmt.Sprintf("/ns/service/%s", servicename)
	var host, hostport string
	var resp *etcd.Response
	var l net.Listener
	var err error

	if host, _, err = net.SplitHostPort(ip); err != nil {
		// Apparently, it's not in host:port format.
		host = ip
	}

	hostport = net.JoinHostPort(host, "0")
	if l, err = net.Listen(network, hostport); err != nil {
		return nil, err
	}

	// Now write our host:port pair to etcd. Let etcd choose the file name.
	resp, err = self.conn.AddChild(path, hostport, 0)
	if err != nil {
		return nil, err
	}

	if resp.Node != nil {
		self.path = resp.Node.Key
	}

	return l, nil
}

/**
 * Open a new anonymous port on "ip" and export it through Doozer as
 * "servicename". Associate the TLS configuration "config". If "ip" is
 * a host:port pair, the port will be overridden.
 */
func (self *ServiceExporter) NewExportedTLSPort(
	network, ip, servicename string,
	config *tls.Config) (net.Listener, error) {
	var l net.Listener
	var err error

	// We can just create a new port as above...
	l, err = self.NewExportedPort(network, ip, servicename)
	if err != nil {
		return nil, err
	}

	// ... and inject a TLS context.
	return tls.NewListener(l, config), nil
}

/**
 * Remove the associated exported port. This will only delete the most
 * recently exported port.
 */
func (self *ServiceExporter) UnexportPort() error {
	var err error

	if len(self.path) == 0 {
		return nil
	}

	if _, err = self.conn.Delete(self.path, false); err != nil {
		return err
	}

	return nil
}
