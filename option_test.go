package gws

//func TestNewUpgrader(t *testing.T) {
//	var as = assert.New(t)
//	var config = NewUpgrader()
//	as.Equal(false, config.CompressEnabled)
//	as.Equal(false, config.AsyncReadEnabled)
//	as.Equal(false, config.CheckTextEncoding)
//	as.Equal(defaultAsyncReadGoLimit, config.AsyncReadGoLimit)
//	as.Equal(defaultMaxContentLength, config.MaxContentLength)
//	as.Equal(defaultAsyncWriteCap, config.AsyncWriteCap)
//	as.NotNil(config.EventHandler)
//	as.NotNil(config.ResponseHeader)
//	as.NotNil(config.CheckOrigin)
//}
//
//func TestOptions(t *testing.T) {
//	var as = assert.New(t)
//	var config = NewUpgrader(
//		WithCompress(flate.BestSpeed, 128),
//		WithAsyncReadEnabled(),
//		WithAsyncReadGoLimit(16),
//		WithMaxContentLength(256),
//		WithCheckTextEncoding(),
//		WithAsyncWriteCap(64),
//	)
//	as.Equal(true, config.CompressEnabled)
//	as.Equal(flate.BestSpeed, config.CompressLevel)
//	as.Equal(128, config.CompressionThreshold)
//	as.Equal(64, config.AsyncWriteCap)
//
//	as.Equal(true, config.AsyncReadEnabled)
//	as.Equal(16, config.AsyncReadGoLimit)
//	as.Equal(256, config.MaxContentLength)
//	as.Equal(true, config.CheckTextEncoding)
//}
