// Copyright 2026 PostFinance AG
// SPDX-License-Identifier: MIT

package decryption

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestReadFile(t *testing.T) {
	t.Run("non-existent file returns fs.ErrNotExist", func(t *testing.T) {
		c := NewCache()
		_, _, err := c.ReadFile("/nonexistent/path/file.yaml")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("expected fs.ErrNotExist, got: %v", err)
		}
	})

	t.Run("plain file returns content without secrets", func(t *testing.T) {
		c := NewCache()
		dir := t.TempDir()
		path := filepath.Join(dir, "plain.yaml")

		if err := os.WriteFile(path, []byte("foo: bar\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		content, secrets, err := c.ReadFile(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(content) != "foo: bar\n" {
			t.Errorf("unexpected content: %q", string(content))
		}
		if len(secrets) != 0 {
			t.Errorf("expected no secrets, got %v", secrets)
		}
	})

	t.Run("nil cache reads a plain file on every call", func(t *testing.T) {
		var c *Cache

		dir := t.TempDir()
		path := filepath.Join(dir, "plain.yaml")

		if err := os.WriteFile(path, []byte("foo: bar\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		// twice: a nil cache keeps no state, so both calls must read afresh
		for i := range 2 {
			content, secrets, err := c.ReadFile(path)
			if err != nil {
				t.Fatalf("call %d: unexpected error: %v", i, err)
			}
			if string(content) != "foo: bar\n" {
				t.Errorf("call %d: unexpected content: %q", i, string(content))
			}
			if len(secrets) != 0 {
				t.Errorf("call %d: expected no secrets, got %v", i, secrets)
			}
		}
	})

	t.Run("nil cache returns fs.ErrNotExist for a non-existent file", func(t *testing.T) {
		var c *Cache

		_, _, err := c.ReadFile("/nonexistent/path/file.yaml")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("expected fs.ErrNotExist, got: %v", err)
		}
	})
}
