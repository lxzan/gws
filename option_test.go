package gws

import (
	"compress/flate"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func validateServerOption(as *assert.Assertions, u *Upgrader) {
	var option = u.option
	var config = u.option.getConfig()
	as.Equal(config.ParallelEnabled, option.ParallelEnabled)
	as.Equal(config.ParallelGolimit, option.ParallelGolimit)
	as.Equal(config.ReadMaxPayloadSize, option.ReadMaxPayloadSize)
	as.Equal(config.WriteMaxPayloadSize, option.WriteMaxPayloadSize)
	as.Equal(config.CheckUtf8Enabled, option.CheckUtf8Enabled)
	as.Equal(config.ReadBufferSize, option.ReadBufferSize)
	as.Equal(config.WriteBufferSize, option.WriteBufferSize)
	as.NotNil(config.brPool)
	as.NotNil(config.Recovery)
	as.Equal(config.Logger, defaultLogger)

	_, ok := u.option.NewSession().(*smap)
	as.True(ok)
}

func validateClientOption(as *assert.Assertions, option *ClientOption) {
	var config = option.getConfig()
	as.Equal(config.ParallelEnabled, option.ParallelEnabled)
	as.Equal(config.ParallelGolimit, option.ParallelGolimit)
	as.Equal(config.ReadMaxPayloadSize, option.ReadMaxPayloadSize)
	as.Equal(config.WriteMaxPayloadSize, option.WriteMaxPayloadSize)
	as.Equal(config.CheckUtf8Enabled, option.CheckUtf8Enabled)
	as.Equal(config.ReadBufferSize, option.ReadBufferSize)
	as.Equal(config.WriteBufferSize, option.WriteBufferSize)
	as.Nil(config.brPool)
	as.NotNil(config.Recovery)
	as.Equal(config.Logger, defaultLogger)

	_, ok := option.NewSession().(*smap)
	as.True(ok)
}

// 检查默认配置
func TestDefaultUpgrader(t *testing.T) {
	var as = assert.New(t)
	var updrader = NewUpgrader(new(BuiltinEventHandler), &ServerOption{
		ResponseHeader: http.Header{
			"Sec-Websocket-Extensions": []string{"chat"},
			"X-Server":                 []string{"gws"},
		},
	})
	var config = updrader.option.getConfig()
	as.Nil(config.cswPool)
	as.Nil(config.dswPool)
	as.Equal(false, config.ParallelEnabled)
	as.Equal(false, config.CheckUtf8Enabled)
	as.Equal(defaultParallelGolimit, config.ParallelGolimit)
	as.Equal(defaultReadMaxPayloadSize, config.ReadMaxPayloadSize)
	as.Equal(defaultWriteMaxPayloadSize, config.WriteMaxPayloadSize)
	as.Equal(defaultHandshakeTimeout, updrader.option.HandshakeTimeout)
	as.NotNil(updrader.eventHandler)
	as.NotNil(config)
	as.NotNil(updrader.option)
	as.NotNil(updrader.option.ResponseHeader)
	as.NotNil(updrader.option.Authorize)
	as.NotNil(updrader.option.NewSession)
	as.Nil(updrader.option.SubProtocols)
	as.Equal("", updrader.option.ResponseHeader.Get("Sec-Websocket-Extensions"))
	as.Equal("gws", updrader.option.ResponseHeader.Get("X-Server"))
	as.Equal(updrader.option.PermessageDeflate.ServerMaxWindowBits, 0)
	as.Equal(updrader.option.PermessageDeflate.ClientMaxWindowBits, 0)
	validateServerOption(as, updrader)
}

func TestCompressServerOption(t *testing.T) {
	var as = assert.New(t)

	t.Run("", func(t *testing.T) {
		var updrader = NewUpgrader(new(BuiltinEventHandler), &ServerOption{
			PermessageDeflate: PermessageDeflate{
				Enabled:               true,
				PoolSize:              60,
				ServerContextTakeover: false,
				ClientContextTakeover: false,
			},
		})
		as.Equal(true, updrader.option.PermessageDeflate.Enabled)
		as.Equal(defaultCompressLevel, updrader.option.PermessageDeflate.Level)
		as.Equal(defaultCompressThreshold, updrader.option.PermessageDeflate.Threshold)
		as.Equal(64, updrader.option.PermessageDeflate.PoolSize)
		as.Equal(updrader.option.PermessageDeflate.ServerMaxWindowBits, 15)
		as.Equal(updrader.option.PermessageDeflate.ClientMaxWindowBits, 15)
		validateServerOption(as, updrader)
	})

	t.Run("", func(t *testing.T) {
		var updrader = NewUpgrader(new(BuiltinEventHandler), &ServerOption{
			PermessageDeflate: PermessageDeflate{
				Enabled:               true,
				ServerContextTakeover: true,
				ClientContextTakeover: true,
				ServerMaxWindowBits:   10,
				ClientMaxWindowBits:   12,
				Level:                 flate.BestCompression,
				Threshold:             1024,
			},
		})
		as.Equal(updrader.option.PermessageDeflate.ServerMaxWindowBits, 10)
		as.Equal(updrader.option.PermessageDeflate.ClientMaxWindowBits, 12)
		as.Equal(true, updrader.option.PermessageDeflate.Enabled)
		as.Equal(flate.BestCompression, updrader.option.PermessageDeflate.Level)
		as.Equal(1024, updrader.option.PermessageDeflate.Threshold)
		as.Equal(defaultCompressorPoolSize, updrader.option.PermessageDeflate.PoolSize)
		validateServerOption(as, updrader)

		as.Equal(cap(updrader.option.config.cswPool.Get()), 1024)
		as.Equal(cap(updrader.option.config.dswPool.Get()), 4*1024)
		as.Equal(len(updrader.option.config.cswPool.Get()), 0)
		as.Equal(len(updrader.option.config.dswPool.Get()), 0)
	})
}

func TestReadServerOption(t *testing.T) {
	var as = assert.New(t)
	var updrader = NewUpgrader(new(BuiltinEventHandler), &ServerOption{
		ParallelEnabled:    true,
		ParallelGolimit:    4,
		ReadMaxPayloadSize: 1024,
		HandshakeTimeout:   10 * time.Second,
	})
	var config = updrader.option.getConfig()
	as.Equal(true, config.ParallelEnabled)
	as.Equal(4, config.ParallelGolimit)
	as.Equal(1024, config.ReadMaxPayloadSize)
	as.Equal(10*time.Second, updrader.option.HandshakeTimeout)
	validateServerOption(as, updrader)
}

func TestDefaultClientOption(t *testing.T) {
	var as = assert.New(t)
	var option = &ClientOption{}
	NewClient(new(BuiltinEventHandler), option)

	var config = option.getConfig()
	as.Nil(config.brPool)
	as.Nil(config.cswPool)
	as.Nil(config.dswPool)
	as.Equal(false, config.ParallelEnabled)
	as.Equal(false, config.CheckUtf8Enabled)
	as.Equal(defaultParallelGolimit, config.ParallelGolimit)
	as.Equal(defaultReadMaxPayloadSize, config.ReadMaxPayloadSize)
	as.Equal(defaultWriteMaxPayloadSize, config.WriteMaxPayloadSize)
	as.NotNil(config)
	as.Equal(0, len(option.RequestHeader))
	as.NotNil(option.NewSession)
	validateClientOption(as, option)
}

func TestCompressClientOption(t *testing.T) {
	var as = assert.New(t)

	t.Run("", func(t *testing.T) {
		var option = &ClientOption{PermessageDeflate: PermessageDeflate{Enabled: true}}
		NewClient(new(BuiltinEventHandler), option)
		as.Equal(true, option.PermessageDeflate.Enabled)
		as.Equal(defaultCompressLevel, option.PermessageDeflate.Level)
		as.Equal(defaultCompressThreshold, option.PermessageDeflate.Threshold)
		as.Equal(option.PermessageDeflate.ServerMaxWindowBits, 15)
		as.Equal(option.PermessageDeflate.ClientMaxWindowBits, 15)
		validateClientOption(as, option)
	})

	t.Run("", func(t *testing.T) {
		var option = &ClientOption{
			PermessageDeflate: PermessageDeflate{
				Enabled:               true,
				ServerContextTakeover: true,
				ClientContextTakeover: true,
				Level:                 flate.BestCompression,
				Threshold:             1024,
			},
		}
		initClientOption(option)

		as.Equal(true, option.PermessageDeflate.Enabled)
		as.Equal(flate.BestCompression, option.PermessageDeflate.Level)
		as.Equal(1024, option.PermessageDeflate.Threshold)
		validateClientOption(as, option)

		var cfg = option.getConfig()
		as.Nil(cfg.cswPool)
		as.Nil(cfg.dswPool)
	})
}

func TestNewSession(t *testing.T) {
	{
		var option = &ServerOption{
			NewSession: func() SessionStorage { return NewConcurrentMap[string, any](16) },
		}
		initServerOption(option)
		_, ok := option.NewSession().(*ConcurrentMap[string, any])
		assert.True(t, ok)
	}

	{
		var option = &ClientOption{
			NewSession: func() SessionStorage { return NewConcurrentMap[string, any](16) },
		}
		initClientOption(option)
		_, ok := option.NewSession().(*ConcurrentMap[string, any])
		assert.True(t, ok)
	}
}
