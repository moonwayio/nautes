package webhook

import "errors"

// options holds the configuration parameters for the webhook server.
//
// The options struct contains all configurable settings that determine how
// the webhook server behaves, including TLS configuration and certificate
// file paths. It uses the functional options pattern to allow flexible
// configuration while maintaining backward compatibility.
type options struct {
	// tls controls whether TLS is enabled for the webhook server
	//
	// When true, the server will use TLS encryption for all connections.
	// When false, the server will accept unencrypted HTTP connections.
	tls *bool

	// certFile is the path to the TLS certificate file
	//
	// This file contains the public key certificate used for TLS encryption.
	// Required when TLS is enabled.
	certFile string

	// keyFile is the path to the TLS private key file
	//
	// This file contains the private key used for TLS encryption.
	// Required when TLS is enabled.
	keyFile string

	// needsLeaderElection indicates if the webhook server needs leader election
	//
	// When set to true, the webhook server will only start when the leader election
	// is active. If set to false, the webhook server will start immediately when the
	// manager starts.
	needsLeaderElection bool
}

// definitelyTrue is a helper variable for setting default TLS configuration.
//
// This variable is used to provide a default value for the tls option
// when no explicit TLS configuration is provided.
var definitelyTrue = true

// OptionFunc is a function that configures webhook server options.
//
// OptionFunc is a functional option pattern that allows for flexible configuration
// of webhook server instances. Each option function modifies the internal options
// struct to customize the server's behavior.
type OptionFunc func(*options)

// setDefaults validates and sets default values for webhook server options.
//
// This method ensures that all required options are set and applies sensible
// defaults for optional parameters. It validates the configuration and returns
// an error if any required options are missing or invalid.
//
// Returns:
//   - error: Any error encountered during validation or default setting
func (o *options) setDefaults() error {
	// Set default TLS configuration if not specified
	if o.tls == nil {
		o.tls = &definitelyTrue
	}

	// Validate TLS configuration when TLS is enabled
	if *o.tls && o.certFile == "" {
		return errors.New("certFile is required")
	}

	if *o.tls && o.keyFile == "" {
		return errors.New("keyFile is required")
	}

	return nil
}

// WithTLS enables or disables TLS for the webhook server.
//
// When enabled, the server will use TLS encryption for all connections.
// When disabled, the server will accept unencrypted HTTP connections.
// TLS is enabled by default for security reasons.
//
// Parameters:
//   - enable: Whether to enable TLS encryption
//
// Returns:
//   - OptionFunc: A function that sets the TLS configuration
func WithTLS(enable bool) OptionFunc {
	return func(o *options) {
		o.tls = &enable
	}
}

// WithCertFile sets the path to the TLS certificate file.
//
// The certificate file should contain a valid X.509 certificate in PEM format.
// This certificate is used to establish TLS connections with clients.
//
// Parameters:
//   - certFile: The path to the TLS certificate file
//
// Returns:
//   - OptionFunc: A function that sets the certificate file path
func WithCertFile(certFile string) OptionFunc {
	return func(o *options) {
		o.certFile = certFile
	}
}

// WithKeyFile sets the path to the TLS private key file.
//
// The key file should contain the private key corresponding to the certificate
// in PEM format. This key is used to establish TLS connections with clients.
//
// Parameters:
//   - keyFile: The path to the TLS private key file
//
// Returns:
//   - OptionFunc: A function that sets the private key file path
func WithKeyFile(keyFile string) OptionFunc {
	return func(o *options) {
		o.keyFile = keyFile
	}
}

// WithNeedsLeaderElection sets if the webhook server needs leader election.
//
// The leader election configuration determines whether the webhook server will
// only start when the leader election is active. If set to false, the webhook server
// will start immediately when the manager starts.
//
// Parameters:
//   - needsLeaderElection: true if the webhook server needs leader election, false otherwise
//
// Returns:
//   - OptionFunc: A function that sets the leader election configuration
func WithNeedsLeaderElection(needsLeaderElection bool) OptionFunc {
	return func(o *options) {
		o.needsLeaderElection = needsLeaderElection
	}
}
