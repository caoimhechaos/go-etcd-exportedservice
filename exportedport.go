/*
Package exportedservice provides exported named etcd ports.
This binds to an anonymous port, exports the host:port pair through etcd
and returns the port to the caller.

There are convenience methods for exporting a TLS port and an HTTP service.
*/
package exportedservice

import (
	"crypto/tls"
	"fmt"
	"net"

	"github.com/caoimhechaos/go-etcd-clientbuilder/autoconf"
	etcd "github.com/coreos/etcd/clientv3"
	"golang.org/x/net/context"
)

// ServiceExporter exists because we need to initialize our etcd client
// beforehand and keep it somewhere.
type ServiceExporter struct {
	conn               *etcd.Client
	path               string
	leaseID            etcd.LeaseID
	keepaliveResponses <-chan *etcd.LeaseKeepAliveResponse
}

func consumeKeepaliveResponses(ch <-chan *etcd.LeaseKeepAliveResponse) {
	for _ = range ch {
	}
}

/*
NewExporter creates a new exporter object which can later be used to create
exported ports and services. This will create a client connection to etcd.
If the connection is severed, once the etcd lease is going to expire the
port will stop being exported.
The specified ttl (which must be at least 5 (seconds)) determines how frequently
the lease will be renewed.
*/
func NewExporter(ctx context.Context, etcdURL string, ttl int64) (
	*ServiceExporter, error) {
	var self *ServiceExporter
	var client *etcd.Client
	var err error

	client, err = etcd.NewFromURL(etcdURL)
	if err != nil {
		return nil, err
	}

	self = &ServiceExporter{
		conn: client,
	}

	return self, self.initLease(ctx, ttl)
}

/*
NewFromDefault creates a new exporter object which can later be used to create
exported ports and services. This will create a client connection to etcd using
the flags based DNS autoconfiguration.

If the connection is severed, once the etcd lease is going to expire the
port will stop being exported.

The specified ttl (which must be at least 5 (seconds)) determines how frequently
the lease will be renewed.
*/
func NewFromDefault(ctx context.Context, ttl int64) (*ServiceExporter, error) {
	var self *ServiceExporter
	var client *etcd.Client
	var err error

	client, err = autoconf.DefaultEtcdClient()
	if err != nil {
		return nil, err
	}

	self = &ServiceExporter{
		conn: client,
	}

	return self, self.initLease(ctx, ttl)
}

/*
NewExporterFromClient creates a new exporter by reading etcd flags from the
specified configuration file.
*/
func NewExporterFromClient(
	ctx context.Context, client *etcd.Client, ttl int64) (
	*ServiceExporter, error) {
	var rv = &ServiceExporter{
		conn: client,
	}

	return rv, rv.initLease(ctx, ttl)
}

/*
initLease initializes the lease on the etcd service which will be used to export
ports in the future.
*/
func (e *ServiceExporter) initLease(ctx context.Context, ttl int64) error {
	var lease *etcd.LeaseGrantResponse
	var err error

	lease, err = e.conn.Grant(ctx, ttl)
	if err != nil {
		return err
	}

	e.keepaliveResponses, err = e.conn.KeepAlive(context.Background(), lease.ID)
	if err != nil {
		return err
	}

	e.leaseID = lease.ID

	go consumeKeepaliveResponses(e.keepaliveResponses)

	return nil
}

/*
NewExportedPort opens a new anonymous port on "ip" and export it through etcd
as "servicename". If "ip" is not a host:port pair, the port will be chosen at
random.
*/
func (e *ServiceExporter) NewExportedPort(
	ctx context.Context, network, ip, service string) (net.Listener, error) {
	var path string
	var host, hostport string
	var l net.Listener
	var err error

	if _, _, err = net.SplitHostPort(ip); err != nil {
		// Apparently, it's not in host:port format.
		host = ip
		hostport = net.JoinHostPort(host, "0")
	} else {
		hostport = ip
	}

	if l, err = net.Listen(network, hostport); err != nil {
		return nil, err
	}

	// Use the lease ID as part of the path; it would be reasonable to expect
	// it to be unique.
	path = fmt.Sprintf("/ns/service/%s/%16x", service, e.leaseID)

	// Now write our host:port pair to etcd. Let etcd choose the file name.
	_, err = e.conn.Put(ctx, path, l.Addr().String(), etcd.WithLease(e.leaseID))
	if err != nil {
		return nil, err
	}

	e.path = path

	return l, nil
}

/*
NewExportedTLSPort opens a new anonymous port on "ip" and export it through
etcd as "servicename" (see NewExportedPort). Associates the TLS configuration
"config". If "ip" is a host:port pair, the port will be overridden.
*/
func (e *ServiceExporter) NewExportedTLSPort(
	ctx context.Context, network, ip, servicename string,
	config *tls.Config) (net.Listener, error) {
	var l net.Listener
	var err error

	// We can just create a new port as above...
	l, err = e.NewExportedPort(ctx, network, ip, servicename)
	if err != nil {
		return nil, err
	}

	// ... and inject a TLS context.
	return tls.NewListener(l, config), nil
}

/*
UnexportPort removes the associated exported port. This will only delete the
most recently exported port. Exported ports will disappear by themselves once
the process dies, but this will expedite the process.
*/
func (e *ServiceExporter) UnexportPort(ctx context.Context) error {
	var err error

	if len(e.path) == 0 {
		return nil
	}

	if _, err = e.conn.Delete(ctx, e.path); err != nil {
		return err
	}

	return nil
}
