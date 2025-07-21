package gws

import (
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"

	"github.com/elazarl/goproxy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewProxyConnectDialer(t *testing.T) {
	u, err := url.Parse("http://localhost:8080")
	require.NoError(t, err)

	dialer, err := NewProxyConnectDialer(u, &net.Dialer{})
	require.NoError(t, err)
	require.NotNil(t, dialer)
}

func TestNewProxyConnectDialerWithOptions(t *testing.T) {
	u, err := url.Parse("https://localhost:8080")
	require.NoError(t, err)
	headers := http.Header{}
	headers.Add("test-header", "test-value")

	dialer, err := NewProxyConnectDialerWithOptions(u, &net.Dialer{}, &ProxyConnectDialerOptions{ProxyConnect: headers})
	require.NoError(t, err)
	require.NotNil(t, dialer)
}

func Test_proxyConnectDialer_Dial(t *testing.T) {
	t.Run("http proxy", func(t *testing.T) {
		var serverResponded atomic.Bool
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if req.Method == http.MethodGet {
				serverResponded.Store(true)
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`ok`))
				return
			}
			w.WriteHeader(http.StatusMethodNotAllowed)
			w.Write([]byte(`error`))
		}))
		defer srv.Close()

		var proxyConnected atomic.Bool
		proxy := goproxy.NewProxyHttpServer()
		proxy.OnRequest().HandleConnect(goproxy.FuncHttpsHandler(func(host string, ctx *goproxy.ProxyCtx) (*goproxy.ConnectAction, string) {
			proxyConnected.Store(true)
			return goproxy.OkConnect, host
		}))
		proxyServer := httptest.NewServer(proxy)
		defer proxyServer.Close()
		t.Logf("Server URL: %s", srv.URL)
		t.Logf("Proxy URL: %s", proxyServer.URL)

		// Create HTTP client that uses custom dialer
		pURL, err := url.Parse(proxyServer.URL)
		require.NoError(t, err)
		dialer, err := NewProxyConnectDialer(pURL, &net.Dialer{})
		require.NoError(t, err)
		client := &http.Client{
			Transport: &http.Transport{
				Dial: dialer.Dial,
			},
		}

		resp, err := client.Get(srv.URL)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.True(t, proxyConnected.Load(), "Expected proxy to recieve CONNECT request.")
		assert.True(t, serverResponded.Load(), "Expected server response.")
	})

	t.Run("https proxy", func(t *testing.T) {
		var serverResponded atomic.Bool
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if req.Method == http.MethodGet {
				serverResponded.Store(true)
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`ok`))
				return
			}
			w.WriteHeader(http.StatusMethodNotAllowed)
			w.Write([]byte(`error`))
		}))
		defer srv.Close()

		var proxyConnected atomic.Bool
		proxy := goproxy.NewProxyHttpServer()
		proxy.OnRequest().HandleConnect(goproxy.FuncHttpsHandler(func(host string, ctx *goproxy.ProxyCtx) (*goproxy.ConnectAction, string) {
			proxyConnected.Store(true)
			return goproxy.OkConnect, host
		}))
		proxyServer := httptest.NewTLSServer(proxy)
		defer proxyServer.Close()

		t.Logf("Server URL: %s", srv.URL)
		t.Logf("Proxy URL: %s", proxyServer.URL)

		// Create HTTP client that uses custom dialer
		tr, ok := proxyServer.Client().Transport.(*http.Transport)
		require.True(t, ok, "expected *http.Transport")
		cfg := tr.TLSClientConfig.Clone()

		pURL, err := url.Parse(proxyServer.URL)
		require.NoError(t, err)
		proxyHost, _, err := net.SplitHostPort(pURL.Host)
		require.NoError(t, err)
		cfg.ServerName = proxyHost

		dialer, err := NewProxyConnectDialerWithOptions(pURL, &net.Dialer{}, &ProxyConnectDialerOptions{
			TLS: cfg,
		})
		require.NoError(t, err)

		// The client that connects through the proxy does not need TLS configuration as the dialer handles it.
		client := &http.Client{
			Transport: &http.Transport{
				Dial: dialer.Dial,
			},
		}

		resp, err := client.Get(srv.URL)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.True(t, serverResponded.Load(), "Expected server response.")
		assert.True(t, proxyConnected.Load(), "Expected proxy to recieve CONNECT request.")
	})

	t.Run("proxy requires auth", func(t *testing.T) {
		var serverResponded atomic.Bool
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if req.Method == http.MethodGet {
				serverResponded.Store(true)
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`ok`))
				return
			}
			w.WriteHeader(http.StatusMethodNotAllowed)
			w.Write([]byte(`error`))
		}))
		defer srv.Close()

		var proxyConnected atomic.Bool
		proxy := goproxy.NewProxyHttpServer()
		proxy.OnRequest().HandleConnect(goproxy.FuncHttpsHandler(func(host string, ctx *goproxy.ProxyCtx) (*goproxy.ConnectAction, string) {
			proxyConnected.Store(true)
			req := ctx.Req
			if req == nil {
				return goproxy.RejectConnect, host
			}
			if auth := req.Header.Get("Proxy-Authorization"); auth == "Basic dGVzdDp0ZXN0" { // test:test
				return goproxy.OkConnect, host
			}
			return goproxy.RejectConnect, host
		}))
		proxyServer := httptest.NewServer(proxy)
		defer proxyServer.Close()
		t.Logf("Server URL: %s", srv.URL)
		t.Logf("Proxy URL: %s", proxyServer.URL)

		// Create HTTP client that uses custom dialer
		pURL, err := url.Parse(proxyServer.URL)
		require.NoError(t, err)
		dialer, err := NewProxyConnectDialerWithOptions(pURL, &net.Dialer{}, &ProxyConnectDialerOptions{
			Username: "test",
			Password: "test",
		})
		require.NoError(t, err)
		client := &http.Client{
			Transport: &http.Transport{
				Dial: dialer.Dial,
			},
		}

		resp, err := client.Get(srv.URL)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.True(t, proxyConnected.Load(), "Expected proxy to recieve CONNECT request.")
		assert.True(t, serverResponded.Load(), "Expected server response.")
	})

	t.Run("proxy failure", func(t *testing.T) {
		var serverResponded atomic.Bool
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if req.Method == http.MethodGet {
				serverResponded.Store(true)
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`ok`))
				return
			}
			w.WriteHeader(http.StatusMethodNotAllowed)
			w.Write([]byte(`error`))
		}))
		defer srv.Close()

		var proxyConnected atomic.Bool
		proxy := goproxy.NewProxyHttpServer()
		proxy.OnRequest().HandleConnect(goproxy.FuncHttpsHandler(func(host string, ctx *goproxy.ProxyCtx) (*goproxy.ConnectAction, string) {
			proxyConnected.Store(true)
			return goproxy.RejectConnect, host
		}))
		proxyServer := httptest.NewServer(proxy)
		defer proxyServer.Close()
		t.Logf("Server URL: %s", srv.URL)
		t.Logf("Proxy URL: %s", proxyServer.URL)

		// Create HTTP client that uses custom dialer
		pURL, err := url.Parse(proxyServer.URL)
		require.NoError(t, err)
		dialer, err := NewProxyConnectDialer(pURL, &net.Dialer{})
		require.NoError(t, err)
		client := &http.Client{
			Transport: &http.Transport{
				Dial: dialer.Dial,
			},
		}

		_, err = client.Get(srv.URL)
		require.Error(t, err)
		assert.True(t, proxyConnected.Load(), "Expected proxy to recieve CONNECT request.")
		assert.False(t, serverResponded.Load(), "Expected no server response.")
	})
	t.Run("server failure", func(t *testing.T) {
		var serverResponded atomic.Bool
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if req.Method == http.MethodGet {
				serverResponded.Store(true)
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(`ok`))
				return
			}
			w.WriteHeader(http.StatusMethodNotAllowed)
			w.Write([]byte(`error`))
		}))
		defer srv.Close()

		var proxyConnected atomic.Bool
		proxy := goproxy.NewProxyHttpServer()
		proxy.OnRequest().HandleConnect(goproxy.FuncHttpsHandler(func(host string, ctx *goproxy.ProxyCtx) (*goproxy.ConnectAction, string) {
			proxyConnected.Store(true)
			return goproxy.OkConnect, host
		}))
		proxyServer := httptest.NewServer(proxy)
		defer proxyServer.Close()
		t.Logf("Server URL: %s", srv.URL)
		t.Logf("Proxy URL: %s", proxyServer.URL)

		// Create HTTP client that uses custom dialer
		pURL, err := url.Parse(proxyServer.URL)
		require.NoError(t, err)
		dialer, err := NewProxyConnectDialer(pURL, &net.Dialer{})
		require.NoError(t, err)
		client := &http.Client{
			Transport: &http.Transport{
				Dial: dialer.Dial,
			},
		}

		resp, err := client.Get(srv.URL)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
		assert.True(t, proxyConnected.Load(), "Expected proxy to recieve CONNECT request.")
		assert.True(t, serverResponded.Load(), "Expected server response.")
	})
}
