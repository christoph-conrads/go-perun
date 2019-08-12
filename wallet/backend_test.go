// Copyright (c) 2019 The Perun Authors. All rights reserved.
// This file is part of go-perun. Use of this source code is governed by a
// MIT-style license that can be found in the LICENSE file.

package wallet

import (
	"io"
	"testing"

	"perun.network/go-perun/pkg/test"
)

type mockBackend struct {
	test.WrapMock
}

// wallet.Backend interface

func (m *mockBackend) NewAddressFromString(string) (Address, error) {
	m.AssertWrapped()
	return nil, nil
}

func (m *mockBackend) NewAddressFromBytes([]byte) (Address, error) {
	m.AssertWrapped()
	return nil, nil
}

// DecodeAddress reads and decodes an address from an io.Writer
func (m *mockBackend) DecodeAddress(io.Reader) (Address, error) {
	m.AssertWrapped()
	return nil, nil
}

func (m *mockBackend) VerifySignature([]byte, []byte, Address) (bool, error) {
	m.AssertWrapped()
	return false, nil
}

// compile-time check that mockBackend imlements Backend
var _ Backend = (*mockBackend)(nil)

// TestGlobalBackend tests all global backend wrappers
func TestGlobalBackend(t *testing.T) {
	b := &mockBackend{test.NewWrapMock(t)}
	SetBackend(b)
	NewAddressFromString("")
	b.AssertCalled()
	NewAddressFromBytes(nil)
	b.AssertCalled()
	DecodeAddress(nil)
	b.AssertCalled()
	VerifySignature(nil, nil, nil)
	b.AssertCalled()
}