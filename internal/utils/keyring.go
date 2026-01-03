package utils

import (
	"fmt"

	"github.com/zalando/go-keyring"
)

const serviceName = "PerSSH"

// StorePassword securely saves a password for a host.
func StorePassword(host, user, password string) error {
	key := fmt.Sprintf("%s@%s", user, host)
	return keyring.Set(serviceName, key, password)
}

// GetPassword retrieves a secure password.
func GetPassword(host, user string) (string, error) {
	key := fmt.Sprintf("%s@%s", user, host)
	return keyring.Get(serviceName, key)
}

// DeletePassword removes a password.
func DeletePassword(host, user string) error {
	key := fmt.Sprintf("%s@%s", user, host)
	return keyring.Delete(serviceName, key)
}
