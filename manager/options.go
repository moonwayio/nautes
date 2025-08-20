package manager

type options struct{}

// OptionFunc is a function that configures manager options.
type OptionFunc func(*options)

func (o *options) setDefaults() error {
	return nil
}
