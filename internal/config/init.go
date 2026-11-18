package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// InitProjectConfig creates .gandalf.json in dir with the given project ID.
// It opens the file exclusively (O_CREATE|O_EXCL) and never overwrites an
// existing configuration. It returns the created file path.
func InitProjectConfig(dir, projectID string) (string, error) {
	if err := ValidateProjectID(projectID); err != nil {
		return "", &ConfigError{Reason: "identificador de proyecto inválido", Err: ErrInvalidProjectID}
	}

	path := filepath.Join(dir, ProjectConfigFileName)
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if os.IsExist(err) {
			return "", &ConfigError{Path: path, Reason: "ya existe la configuración del proyecto", Err: ErrConfigExists}
		}
		return "", &ConfigError{Path: path, Reason: "no se puede crear el archivo", Err: ErrInvalidConfig}
	}
	defer f.Close()

	payload, err := json.Marshal(map[string]string{"project": projectID})
	if err != nil {
		return "", &ConfigError{Path: path, Reason: "no se puede codificar la configuración", Err: ErrInvalidConfig}
	}
	if _, err := f.Write(payload); err != nil {
		return "", &ConfigError{Path: path, Reason: "no se puede escribir el archivo", Err: ErrInvalidConfig}
	}
	return path, nil
}
