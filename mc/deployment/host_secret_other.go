//go:build !darwin && !linux

package deployment

type unavailableHostSecretStore struct{}

// hostSecretNamePrefix keeps the shared identity well-defined on platforms
// whose store refuses every operation.
const hostSecretNamePrefix = "Culler-"

func platformHostSecretStore() hostSecretStore { return unavailableHostSecretStore{} }
func (unavailableHostSecretStore) Read(string, string) ([]byte, error) {
	return nil, ErrHostCredentialUnavailable
}
func (unavailableHostSecretStore) Write(string, string, []byte) error {
	return ErrHostCredentialUnavailable
}
func (unavailableHostSecretStore) Delete(string, string) error { return ErrHostCredentialUnavailable }
