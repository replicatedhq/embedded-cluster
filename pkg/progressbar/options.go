package progressbar

// Option is a function that sets an option on a MessageWriter.
type Option func(*MessageWriter)

// WithWriter sets the WriteFn on the MessageWriter.
func WithWriter(w WriteFn) Option {
	return func(m *MessageWriter) {
		m.printf = w
	}
}

// WithMask sets the MaskFn on the MessageWriter.
func WithMask(mfn MaskFn) Option {
	return func(m *MessageWriter) {
		m.mask = mfn
	}
}
