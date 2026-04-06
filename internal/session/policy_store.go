package session

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/hironow/amadeus/internal/domain"
)

const policyFileName = "routing.yaml"

// LoadRoutingPolicy loads a routing policy from {stateDir}/.policy/routing.yaml.
// Returns DefaultRoutingPolicy when the file does not exist (graceful fallback).
func LoadRoutingPolicy(stateDir string) (domain.RoutingPolicy, error) {
	path := filepath.Join(stateDir, ".policy", policyFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return domain.DefaultRoutingPolicy(), nil
		}
		return domain.RoutingPolicy{}, fmt.Errorf("load routing policy: %w", err)
	}

	var policy domain.RoutingPolicy
	if err := yaml.Unmarshal(data, &policy); err != nil {
		return domain.RoutingPolicy{}, fmt.Errorf("parse routing policy: %w", err)
	}
	return policy, nil
}

// SaveRoutingPolicy writes a routing policy to {stateDir}/.policy/routing.yaml.
func SaveRoutingPolicy(stateDir string, policy domain.RoutingPolicy) error {
	dir := filepath.Join(stateDir, ".policy")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("save routing policy: create dir: %w", err)
	}

	data, err := yaml.Marshal(policy)
	if err != nil {
		return fmt.Errorf("save routing policy: marshal: %w", err)
	}

	path := filepath.Join(dir, policyFileName)
	return os.WriteFile(path, data, 0o644)
}
