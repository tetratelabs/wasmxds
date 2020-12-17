package ociregistory

import "fmt"

type LocalRegistry struct{ *imagePuller }

func NewLocalRegistry(username, password, port string) LocalRegistry {
	return LocalRegistry{newImagePuller(fmt.Sprintf("localhost:%s", port), func() (string, string, error) {
		return username, password, nil
	})}
}
