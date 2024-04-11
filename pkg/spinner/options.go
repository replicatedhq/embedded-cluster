package spinner

// Option is a function that sets an option on a MessageWriter.
type Option func(*MessageWriter)

// WithWriter sets the WriteFn on the MessageWriter.
func WithWriter(w WriteFn) Option {
	return func(m *MessageWriter) {
		m.printf = w
	}
}

// WithLineBreaker sets a function that determines if if is time
// to break the line thus creating a new spinner line. The previous
// step is flagged as success.
func WithLineBreaker(lb LineBreakerFn) Option {
	return func(m *MessageWriter) {
		m.lbreak = lb
	}
}

// WithMask sets the MaskFn on the MessageWriter.
func WithMask(mfn MaskFn) Option {
	return func(m *MessageWriter) {
		m.mask = mfn
	}
}
