package gws

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
)

// Dialer 拨号器接口
// Dialer interface
type Dialer interface {
	// Dial 连接到指定网络上的地址
	// Connects to the address on the named network
	Dial(network, addr string) (c net.Conn, err error)
}

// contextDialer interface
// currently unexported as library does not generally use DialContext
type contextDialer interface {
	DialContext(ctx context.Context, network, addr string) (net.Conn, error)
}

var (
	ErrUnsupportedScheme  = errors.New("requested scheme is unsupported")
	ErrUnsupportedNetwork = errors.New("requested network is unsupported")
)

type proxyConnectDialer struct {
	url     *url.URL
	forward Dialer
	options *ProxyConnectDialerOptions
}

func (p *proxyConnectDialer) Dial(network, addr string) (net.Conn, error) {
	return p.DialContext(context.Background(), network, addr)
}

func (p *proxyConnectDialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	if network != "tcp" {
		return nil, ErrUnsupportedNetwork
	}

	var conn net.Conn
	var err error
	if contextForward, ok := p.forward.(contextDialer); ok {
		conn, err = contextForward.DialContext(ctx, network, p.url.Host)
	} else {
		conn, err = p.forward.Dial(network, p.url.Host)
	}
	if err != nil {
		return nil, err
	}

	if p.url.Scheme == "https" {
		conn = tls.Client(conn, p.options.TLS)
	}

	req := &http.Request{
		Method: "CONNECT",
		URL:    &url.URL{Opaque: addr},
		Host:   addr,
		Header: p.options.ProxyConnect.Clone(),
		Close:  false,
	}
	if req.Header == nil {
		req.Header = http.Header{}
	}
	req = req.WithContext(ctx)

	// Add a (basic) proxy-auth header if username and password are non-empty.
	if p.options.Username != "" && p.options.Password != "" {
		basicAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte(p.options.Username+":"+p.options.Password))
		req.Header.Add("Proxy-Authorization", basicAuth)
	}

	resp, err := p.roundTrip(conn, req)
	if err != nil {
		conn.Close()
		return nil, err
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		conn.Close()
		return nil, fmt.Errorf("proxyDialer recieved non-200 response for CONNECT request: %d", resp.StatusCode)
	}
	return conn, nil
}

func (p *proxyConnectDialer) roundTrip(conn net.Conn, req *http.Request) (*http.Response, error) {
	if err := req.WriteProxy(conn); err != nil {
		return nil, err
	}
	return http.ReadResponse(bufio.NewReader(conn), req)
}

// ProxyConnectDialerOptions are optional options that can be specified when creating a ProxyConnectDialer.
type ProxyConnectDialerOptions struct {
	// TLS specifies proxy-specific TLS config.
	// At a minimum ServerName or InsecureSkipVerify must be set.
	TLS *tls.Config
	// ProxyConnect is the optional set of http.Header values for the CONNECT request.
	ProxyConnect http.Header

	// Username is used in the Proxy-Authorization header for the CONNECT request.
	// If specified directly in options, and as part of the url, the url value is used.
	Username string
	// Password is used in the Proxy-Authorization header for the CONNECT request.
	// If specified directly in options, and as part of the url, the url value is used.
	Password string
}

// NewProxyConnectDialer creates an HTTP proxy dialer that uses a CONNECT request for the initial request to the proxy.
// The url's scheme must be set to http, or https.
// The passed Dialer is used as a forwarding dialer to make network requests.
func NewProxyConnectDialer(u *url.URL, forward Dialer) (Dialer, error) {
	return NewProxyConnectDialerWithOptions(u, forward, nil)
}

// NewProxyConnectDialerWithOptions creates an HTTP proxy dialer that uses a CONNECT request for the initial request to the proxy.
// The url's scheme must be set to http, or https.
// The passed Dialer is used as a forwarding dialer to make network requests.
// ProxyConnectDialerOptions specifies optional configuration options.
func NewProxyConnectDialerWithOptions(u *url.URL, forward Dialer, options *ProxyConnectDialerOptions) (Dialer, error) {
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedScheme, u.Scheme)
	}

	if options == nil {
		options = &ProxyConnectDialerOptions{}
	}

	// Set Username and password to those associated with the url if present
	if u.User != nil {
		options.Username = u.User.Username()
		options.Password, _ = u.User.Password()
	}

	// Ensure that both username and password are set
	if options.Username == "" || options.Password == "" {
		options.Username = ""
		options.Password = ""
	}

	// If it's an https connection, and there's no tls.Config we need to determine the ServerName
	if u.Scheme == "https" && options.TLS == nil {
		serverName, _, err := net.SplitHostPort(u.Host)
		if err != nil && err.Error() == "missing port in address" {
			serverName = u.Host
		}
		if serverName == "" {
			return nil, fmt.Errorf("unable to create tls.Config: could not detect ServerName from url: %w", err)
		}
		options.TLS = &tls.Config{
			ServerName: serverName,
		}
	}

	return &proxyConnectDialer{
		url:     u,
		forward: forward,
		options: options,
	}, nil
}
