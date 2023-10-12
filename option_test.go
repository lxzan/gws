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
	as.Equal(config.ReadAsyncEnabled, option.ReadAsyncEnabled)
	as.Equal(config.ReadAsyncGoLimit, option.ReadAsyncGoLimit)
	as.Equal(config.ReadMaxPayloadSize, option.ReadMaxPayloadSize)
	as.Equal(config.WriteMaxPayloadSize, option.WriteMaxPayloadSize)
	as.Equal(config.CompressEnabled, option.CompressEnabled)
	as.Equal(config.CompressLevel, option.CompressLevel)
	as.Equal(config.CompressThreshold, option.CompressThreshold)
	as.Equal(config.CheckUtf8Enabled, option.CheckUtf8Enabled)
	as.Equal(config.ReadBufferSize, option.ReadBufferSize)
	as.Equal(config.WriteBufferSize, option.WriteBufferSize)
	as.Equal(config.CompressorNum, option.CompressorNum)
	as.NotNil(config.readerPool)
	as.NotNil(config.Caller)

	_, ok := u.option.NewSessionStorage().(*sliceMap)
	as.True(ok)
}

func validateClientOption(as *assert.Assertions, option *ClientOption) {
	var config = option.getConfig()
	as.Equal(config.ReadAsyncEnabled, option.ReadAsyncEnabled)
	as.Equal(config.ReadAsyncGoLimit, option.ReadAsyncGoLimit)
	as.Equal(config.ReadMaxPayloadSize, option.ReadMaxPayloadSize)
	as.Equal(config.WriteMaxPayloadSize, option.WriteMaxPayloadSize)
	as.Equal(config.CompressEnabled, option.CompressEnabled)
	as.Equal(config.CompressLevel, option.CompressLevel)
	as.Equal(config.CompressThreshold, option.CompressThreshold)
	as.Equal(config.CheckUtf8Enabled, option.CheckUtf8Enabled)
	as.Equal(config.ReadBufferSize, option.ReadBufferSize)
	as.Equal(config.WriteBufferSize, option.WriteBufferSize)
	as.Nil(config.readerPool)
	as.NotNil(config.Caller)

	_, ok := option.NewSessionStorage().(*sliceMap)
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
	as.Equal(false, config.CompressEnabled)
	as.Equal(false, config.ReadAsyncEnabled)
	as.Equal(false, config.CheckUtf8Enabled)
	as.Equal(defaultReadAsyncGoLimit, config.ReadAsyncGoLimit)
	as.Equal(defaultReadMaxPayloadSize, config.ReadMaxPayloadSize)
	as.Equal(defaultWriteMaxPayloadSize, config.WriteMaxPayloadSize)
	as.Equal(defaultCompressorNum, config.CompressorNum)
	as.Equal(defaultHandshakeTimeout, updrader.option.HandshakeTimeout)
	as.NotNil(updrader.eventHandler)
	as.NotNil(config)
	as.NotNil(updrader.option)
	as.NotNil(updrader.option.ResponseHeader)
	as.NotNil(updrader.option.Authorize)
	as.NotNil(updrader.option.NewSessionStorage)
	as.Nil(updrader.option.SubProtocols)
	as.Equal("", updrader.option.ResponseHeader.Get("Sec-Websocket-Extensions"))
	as.Equal("gws", updrader.option.ResponseHeader.Get("X-Server"))
	validateServerOption(as, updrader)
}

func TestCompressServerOption(t *testing.T) {
	var as = assert.New(t)

	t.Run("", func(t *testing.T) {
		var updrader = NewUpgrader(new(BuiltinEventHandler), &ServerOption{
			CompressEnabled: true,
			CompressorNum:   60,
		})
		var config = updrader.option.getConfig()
		as.Equal(true, config.CompressEnabled)
		as.Equal(defaultCompressLevel, config.CompressLevel)
		as.Equal(defaultCompressThreshold, config.CompressThreshold)
		as.Equal(64, config.CompressorNum)
		validateServerOption(as, updrader)
	})

	t.Run("", func(t *testing.T) {
		var updrader = NewUpgrader(new(BuiltinEventHandler), &ServerOption{
			CompressEnabled:   true,
			CompressLevel:     flate.BestCompression,
			CompressThreshold: 1024,
		})
		var config = updrader.option.getConfig()
		as.Equal(true, config.CompressEnabled)
		as.Equal(flate.BestCompression, config.CompressLevel)
		as.Equal(1024, config.CompressThreshold)
		as.Equal(defaultCompressorNum, config.CompressorNum)
		validateServerOption(as, updrader)
	})
}

func TestReadServerOption(t *testing.T) {
	var as = assert.New(t)
	var updrader = NewUpgrader(new(BuiltinEventHandler), &ServerOption{
		ReadAsyncEnabled:   true,
		ReadAsyncGoLimit:   4,
		ReadMaxPayloadSize: 1024,
		HandshakeTimeout:   10 * time.Second,
	})
	var config = updrader.option.getConfig()
	as.Equal(true, config.ReadAsyncEnabled)
	as.Equal(4, config.ReadAsyncGoLimit)
	as.Equal(1024, config.ReadMaxPayloadSize)
	as.Equal(10*time.Second, updrader.option.HandshakeTimeout)
	validateServerOption(as, updrader)
}

func TestDefaultClientOption(t *testing.T) {
	var as = assert.New(t)
	var option = &ClientOption{}
	NewClient(new(BuiltinEventHandler), option)

	var config = option.getConfig()
	as.Equal(false, config.CompressEnabled)
	as.Equal(false, config.ReadAsyncEnabled)
	as.Equal(false, config.CheckUtf8Enabled)
	as.Equal(defaultReadAsyncGoLimit, config.ReadAsyncGoLimit)
	as.Equal(defaultReadMaxPayloadSize, config.ReadMaxPayloadSize)
	as.Equal(defaultWriteMaxPayloadSize, config.WriteMaxPayloadSize)
	as.Equal(1, config.CompressorNum)
	as.NotNil(config)
	as.Equal(0, len(option.RequestHeader))
	as.NotNil(option.NewSessionStorage)
	validateClientOption(as, option)
}

func TestCompressClientOption(t *testing.T) {
	var as = assert.New(t)

	t.Run("", func(t *testing.T) {
		var option = &ClientOption{CompressEnabled: true}
		NewClient(new(BuiltinEventHandler), option)
		var config = option.getConfig()
		as.Equal(true, config.CompressEnabled)
		as.Equal(defaultCompressLevel, config.CompressLevel)
		as.Equal(defaultCompressThreshold, config.CompressThreshold)
		validateClientOption(as, option)
	})

	t.Run("", func(t *testing.T) {
		var option = &ClientOption{
			CompressEnabled:   true,
			CompressLevel:     flate.BestCompression,
			CompressThreshold: 1024,
		}
		initClientOption(option)
		var config = option.getConfig()
		as.Equal(true, config.CompressEnabled)
		as.Equal(flate.BestCompression, config.CompressLevel)
		as.Equal(1024, config.CompressThreshold)
		validateClientOption(as, option)
	})
}

func TestNewSessionStorage(t *testing.T) {
	{
		var option = &ServerOption{
			NewSessionStorage: func() SessionStorage { return NewConcurrentMap[string, any](16) },
		}
		initServerOption(option)
		_, ok := option.NewSessionStorage().(*ConcurrentMap[string, any])
		assert.True(t, ok)
	}

	{
		var option = &ClientOption{
			NewSessionStorage: func() SessionStorage { return NewConcurrentMap[string, any](16) },
		}
		initClientOption(option)
		_, ok := option.NewSessionStorage().(*ConcurrentMap[string, any])
		assert.True(t, ok)
	}
}
