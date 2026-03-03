package gws

import (
	"errors"
	"net"
	"strings"
	"time"
	"unsafe"

	"github.com/lxzan/gws/internal"
	"github.com/valyala/fasthttp"
)

// b2s converts byte slice to string without memory allocation.
// The returned string shares the underlying bytes and must not outlive the byte slice.
func b2s(b []byte) string {
	return unsafe.String(unsafe.SliceData(b), len(b))
}

// Upgrades a fasthttp connection to a WebSocket connection.
// It validates the handshake, sets response headers, hijacks the connection,
// and starts ReadLoop in the hijack callback.
// Authorization should be handled at the application/middleware level before calling this method.
// An optional SessionStorage can be passed to pre-populate session data.
func (c *Upgrader) UpgradeFastHTTP(ctx *fasthttp.RequestCtx, sessions ...SessionStorage) error {
	// Check HTTP method
	if !ctx.IsGet() {
		return ErrHandshake
	}

	// Check request headers
	if !internal.HttpHeaderContains(b2s(ctx.Request.Header.Peek(internal.Connection.Key)), internal.Connection.Val) {
		return ErrHandshake
	}
	if !strings.EqualFold(b2s(ctx.Request.Header.Peek(internal.Upgrade.Key)), internal.Upgrade.Val) {
		return ErrHandshake
	}
	if !strings.EqualFold(b2s(ctx.Request.Header.Peek(internal.SecWebSocketVersion.Key)), internal.SecWebSocketVersion.Val) {
		return errors.New("gws: websocket version not supported")
	}

	websocketKey := b2s(ctx.Request.Header.Peek(internal.SecWebSocketKey.Key))
	if websocketKey == "" {
		return ErrHandshake
	}

	// Permessage-deflate negotiation
	extensions := b2s(ctx.Request.Header.Peek(internal.SecWebSocketExtensions.Key))
	pd := c.getPermessageDeflate(extensions)

	// Subprotocol negotiation
	var subprotocol string
	if len(c.option.SubProtocols) > 0 {
		clientProtos := internal.Split(b2s(ctx.Request.Header.Peek(internal.SecWebSocketProtocol.Key)), ",")
		subprotocol = internal.GetIntersectionElem(c.option.SubProtocols, clientProtos)
		if subprotocol == "" {
			return ErrSubprotocolNegotiation
		}
	}

	// Set 101 Switching Protocols response headers
	ctx.SetStatusCode(fasthttp.StatusSwitchingProtocols)
	ctx.Response.Header.Set(internal.Upgrade.Key, internal.Upgrade.Val)
	ctx.Response.Header.Set(internal.Connection.Key, internal.Connection.Val)
	ctx.Response.Header.Set(internal.SecWebSocketAccept.Key, internal.ComputeAcceptKey(websocketKey))
	if pd.Enabled {
		ctx.Response.Header.Set(internal.SecWebSocketExtensions.Key, pd.genResponseHeader())
	}
	if subprotocol != "" {
		ctx.Response.Header.Set(internal.SecWebSocketProtocol.Key, subprotocol)
	}

	// Extra response headers from ServerOption
	for k := range c.option.ResponseHeader {
		ctx.Response.Header.Set(k, c.option.ResponseHeader.Get(k))
	}

	// Hijack callback runs after the handler returns and after fasthttp sends the 101 response.
	ctx.Hijack(func(conn net.Conn) {
		_ = conn.SetDeadline(time.Time{})

		br := c.option.config.brPool.Get()
		br.Reset(conn)

		session := c.option.NewSession()
		if len(sessions) > 0 && sessions[0] != nil {
			session = sessions[0]
		}

		config := c.option.getConfig()
		socket := &Conn{
			ss:                session,
			isServer:          true,
			subprotocol:       subprotocol,
			pd:                pd,
			conn:              conn,
			config:            config,
			br:                br,
			continuationFrame: continuationFrame{},
			fh:                frameHeader{},
			handler:           c.eventHandler,
			closed:            0,
			writeQueue:        workerQueue{maxConcurrency: 1},
			readQueue:         make(channel, c.option.ParallelGolimit),
		}

		if pd.Enabled {
			socket.deflater = c.deflaterPool.Select()
			if pd.ServerContextTakeover {
				socket.cpsWindow.initialize(config.cswPool, pd.ServerMaxWindowBits)
			}
			if pd.ClientContextTakeover {
				socket.dpsWindow.initialize(config.dswPool, pd.ClientMaxWindowBits)
			}
		}

		socket.ReadLoop()
	})

	return nil
}
