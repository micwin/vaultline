package api

// UnsealRequest carries the passphrase needed to unlock the daemon.
type UnsealRequest struct {
	Passphrase         string `json:"passphrase"`
	RememberPassphrase bool   `json:"remember_passphrase"`
}

// UnsealResponse mirrors sealed state after an unseal attempt.
type UnsealResponse struct {
	Store  string `json:"store,omitempty"`
	Sealed bool   `json:"sealed"`
}

type SealRequest struct {
	KeepKeys bool `json:"keep_keys"`
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

type StoreCreateRequest struct {
	Name               string `json:"name"`
	Path               string `json:"path"`
	Initialize         bool   `json:"initialize"`
	Passphrase         string `json:"passphrase,omitempty"`
	RememberPassphrase bool   `json:"remember_passphrase"`
}

type StoreCreateResponse struct {
	Name       string `json:"name"`
	Path       string `json:"path"`
	Default    bool   `json:"default"`
	Available  bool   `json:"available"`
	Sealed     bool   `json:"sealed"`
	HasKey     bool   `json:"has_key"`
	Passphrase string `json:"passphrase,omitempty"`
}

type DaemonBindRequest struct {
	Addr string `json:"addr"`
}

type DaemonAllowRequest struct {
	Rule string `json:"rule"`
}

// ErrorResponse is returned when the API needs to describe a fault.
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}
