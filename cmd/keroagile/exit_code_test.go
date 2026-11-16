package main

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tbdtechpro/KeroAgile/internal/domain"
)

// The documented contract: 0 success, 1 not-found (and other failures),
// 2 validation error.
func TestExitCode(t *testing.T) {
	assert.Equal(t, 0, exitCode(nil))
	assert.Equal(t, 2, exitCode(fmt.Errorf("%w status %q", domain.ErrInvalid, "doing")))
	assert.Equal(t, 1, exitCode(fmt.Errorf("ticket %q: %w", "KA-999", domain.ErrNotFound)))
	assert.Equal(t, 1, exitCode(errors.New("disk full")))
}
