package webhook

import "errors"

type options struct {
	tls      *bool
	certFile string
	keyFile  string
}

var definitelyTrue = true

// OptionFunc is a function that configures webhook server options.
type OptionFunc func(*options)

func (o *options) setDefaults() error {
	if o.tls == nil {
		o.tls = &definitelyTrue
	}

	if *o.tls && o.certFile == "" {
		return errors.New("certFile is required")
	}

	if *o.tls && o.keyFile == "" {
		return errors.New("keyFile is required")
	}

	return nil
}

// WithTLS enables or disables TLS for the webhook server.
func WithTLS(enable bool) OptionFunc {
	return func(o *options) {
		o.tls = &enable
	}
}

// WithCertFile sets the path to the TLS certificate file.
func WithCertFile(certFile string) OptionFunc {
	return func(o *options) {
		o.certFile = certFile
	}
}

// WithKeyFile sets the path to the TLS private key file.
func WithKeyFile(keyFile string) OptionFunc {
	return func(o *options) {
		o.keyFile = keyFile
	}
}
