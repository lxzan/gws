package gws

import (
	"compress/flate"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestNewUpgrader(t *testing.T) {
	var as = assert.New(t)
	var config = NewUpgrader()
	as.Equal(false, config.CompressEnabled)
	as.Equal(false, config.AsyncReadEnabled)
	as.Equal(false, config.CheckTextEncoding)
	as.Equal(defaultAsyncIOGoLimit, config.AsyncIOGoLimit)
	as.Equal(defaultMaxContentLength, config.MaxContentLength)
	as.NotNil(config.EventHandler)
	as.NotNil(config.ResponseHeader)
	as.NotNil(config.CheckOrigin)
}

func TestOptions(t *testing.T) {
	var as = assert.New(t)
	var config = NewUpgrader(
		WithCompress(flate.BestSpeed, 128),
		WithAsyncReadEnabled(),
		WithAsyncIOGoLimit(16),
		WithMaxContentLength(256),
		WithCheckTextEncoding(),
	)
	as.Equal(true, config.CompressEnabled)
	as.Equal(flate.BestSpeed, config.CompressLevel)
	as.Equal(128, config.CompressionThreshold)

	as.Equal(true, config.AsyncReadEnabled)
	as.Equal(16, config.AsyncIOGoLimit)
	as.Equal(256, config.MaxContentLength)
	as.Equal(true, config.CheckTextEncoding)
}
