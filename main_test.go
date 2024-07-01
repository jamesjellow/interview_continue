package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/dominikbraun/graph"
)

type mockHandlers struct{}

func (m mockHandlers) HandleAdd(args []string, depGraph *graph.Graph[string, string]) error {
	return mockHandleAdd(args)
}

func (m mockHandlers) HandleInstall(depGraph *graph.Graph[string, string]) error {
	return mockHandleInstall()
}

var mockHandleAdd func(args []string) error
var mockHandleInstall func() error

func setup() func() {
	originalHandlers := handlerInstance
	handlerInstance = mockHandlers{}
	return func() { handlerInstance = originalHandlers }
}

func TestRunNoArguments(t *testing.T) {
	teardown := setup()
	defer teardown()

	err := run([]string{"fpm"})
	if err == nil {
		t.Errorf("expected error, got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "expected 'add' or 'install' subcommand") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestRunUnknownSubcommand(t *testing.T) {
	teardown := setup()
	defer teardown()

	err := run([]string{"fpm", "unknown"})
	if err == nil {
		t.Errorf("expected error, got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "unknown subcommand: unknown") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestRunAddCommand(t *testing.T) {
	teardown := setup()
	defer teardown()

	mockHandleAdd = func(args []string) error {
		return nil
	}

	err := run([]string{"fpm", "add", "package"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunInstallCommand(t *testing.T) {
	teardown := setup()
	defer teardown()

	mockHandleInstall = func() error {
		return nil
	}

	err := run([]string{"fpm", "install"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunAddCommandError(t *testing.T) {
	teardown := setup()
	defer teardown()

	mockHandleAdd = func(_ []string) error {
		return errors.New("add error")
	}

	err := run([]string{"fpm", "add", "package"})
	if err == nil {
		t.Errorf("expected error, got nil")
	}
	if err != nil && err.Error() != "add error" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestRunInstallCommandError(t *testing.T) {
	teardown := setup()
	defer teardown()

	mockHandleInstall = func() error {
		return errors.New("install error")
	}

	err := run([]string{"fpm", "install"})
	if err == nil {
		t.Errorf("expected error, got nil")
	}
	if err != nil && err.Error() != "install error" {
		t.Errorf("unexpected error message: %v", err)
	}
}
