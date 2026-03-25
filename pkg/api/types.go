package api

// UnsealRequest carries the passphrase needed to unlock the daemon.
type UnsealRequest struct {
	Passphrase string `json:"passphrase"`
}

// UnsealResponse mirrors sealed state after an unseal attempt.
type UnsealResponse struct {
	Sealed bool `json:"sealed"`
}

// SecretRequest allows clients to create or update a secret. Value must be base64 encoded.
type SecretRequest struct {
	Value string `json:"value"`
}

// SecretResponse returns the stored secret (still base64 encoded) and version identifier.
type SecretResponse struct {
	Value   string `json:"value"`
	Version string `json:"version"`
}

// VersionResponse is returned when only the version string is needed.
type VersionResponse struct {
	Version string `json:"version"`
}

// ErrorResponse is returned when the API needs to describe a fault.
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}
