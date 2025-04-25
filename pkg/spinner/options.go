package spinner

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

// SetMask sets the MaskFn for a MessageWriter.
func (m *MessageWriter) SetMask(mfn MaskFn) {
	m.mask = mfn
}

// WithTTY sets the TTY flag on the MessageWriter.
func WithTTY(tty bool) Option {
	return func(m *MessageWriter) {
		m.tty = tty
	}
}

// SetTTY sets the TTY flag for a MessageWriter.
func (m *MessageWriter) SetTTY(tty bool) {
	m.tty = tty
}
