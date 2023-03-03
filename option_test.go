package gws

import (
	"compress/flate"
	"github.com/stretchr/testify/assert"
	"testing"
)

func validateUpgrader(as *assert.Assertions, u *Upgrader) {
	var option = u.option
	var config = u.option.getConfig()
	as.Equal(config.ReadAsyncEnabled, option.ReadAsyncEnabled)
	as.Equal(config.ReadAsyncGoLimit, option.ReadAsyncGoLimit)
	as.Equal(config.ReadAsyncCap, option.ReadAsyncCap)
	as.Equal(config.ReadMaxPayloadSize, option.ReadMaxPayloadSize)
	as.Equal(config.WriteAsyncCap, option.WriteAsyncCap)
	as.Equal(config.WriteMaxPayloadSize, option.WriteMaxPayloadSize)
	as.Equal(config.CompressEnabled, option.CompressEnabled)
	as.Equal(config.CompressLevel, option.CompressLevel)
	as.Equal(config.CompressThreshold, option.CompressThreshold)
	as.Equal(config.CheckUtf8Enabled, option.CheckUtf8Enabled)
	as.Equal(config.ReadBufferSize, option.ReadBufferSize)
	as.Equal(config.WriteBufferSize, option.WriteBufferSize)
}

// 检查默认配置
func TestDefaultUpgrader(t *testing.T) {
	var as = assert.New(t)
	var updrader = NewUpgrader(new(BuiltinEventHandler), nil)
	var config = updrader.option.getConfig()
	as.Equal(false, config.CompressEnabled)
	as.Equal(false, config.ReadAsyncEnabled)
	as.Equal(false, config.CheckUtf8Enabled)
	as.Equal(defaultReadAsyncGoLimit, config.ReadAsyncGoLimit)
	as.Equal(defaultReadAsyncCap, config.ReadAsyncCap)
	as.Equal(defaultReadMaxPayloadSize, config.ReadMaxPayloadSize)
	as.Equal(defaultWriteMaxPayloadSize, config.WriteMaxPayloadSize)
	as.Equal(defaultWriteAsyncCap, config.WriteAsyncCap)
	as.NotNil(updrader.eventHandler)
	as.NotNil(config)
	as.NotNil(updrader.option)
	as.NotNil(updrader.option.ResponseHeader)
	as.NotNil(updrader.option.CheckOrigin)
	validateUpgrader(as, updrader)
}

func TestCompressServerOption(t *testing.T) {
	var as = assert.New(t)

	t.Run("", func(t *testing.T) {
		var updrader = NewUpgrader(new(BuiltinEventHandler), &ServerOption{
			CompressEnabled: true,
		})
		var config = updrader.option.getConfig()
		as.Equal(true, config.CompressEnabled)
		as.Equal(defaultCompressLevel, config.CompressLevel)
		as.Equal(defaultCompressThreshold, config.CompressThreshold)
		validateUpgrader(as, updrader)
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
		validateUpgrader(as, updrader)
	})
}

func TestReadServerOption(t *testing.T) {
	var as = assert.New(t)
	var updrader = NewUpgrader(new(BuiltinEventHandler), &ServerOption{
		ReadAsyncEnabled:   true,
		ReadAsyncGoLimit:   4,
		ReadMaxPayloadSize: 1024,
	})
	var config = updrader.option.getConfig()
	as.Equal(true, config.ReadAsyncEnabled)
	as.Equal(4, config.ReadAsyncGoLimit)
	as.Equal(1024, config.ReadMaxPayloadSize)
	validateUpgrader(as, updrader)
}
