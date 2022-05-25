package utils

import (
	"strings"

	"github.com/google/uuid"
)

func GenUUID() string {
	return strings.ReplaceAll(uuid.New().String(), "-", "")
}
