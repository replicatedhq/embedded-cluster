package docker

import "math/rand"

var alphabet = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

func generateID() string {
	b := make([]rune, 32)
	for i := range b {
		b[i] = alphabet[rand.Intn(len(alphabet))]
	}
	return "ece2e-" + string(b)
}
