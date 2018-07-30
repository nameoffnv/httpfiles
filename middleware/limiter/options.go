package limiter

const (
	defaultMaxConnectionPerIP  = 1
	defaultMaxRequestPerSecond = 1
	defaultMaxBytesPerIP       = 10 * 1024 * 1024
)

type Options struct {
	MaxConnectionPerIP  int
	MaxRequestPerSecond int
	MaxBytesPerIP       int64
}

func (o *Options) Setup() {
	if o.MaxConnectionPerIP == 0 {
		o.MaxConnectionPerIP = defaultMaxConnectionPerIP
	}
	if o.MaxRequestPerSecond == 0 {
		o.MaxRequestPerSecond = defaultMaxRequestPerSecond
	}
	if o.MaxBytesPerIP == 0 {
		o.MaxBytesPerIP = defaultMaxBytesPerIP
	}
}
