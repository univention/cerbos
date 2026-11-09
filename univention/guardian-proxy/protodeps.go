// SPDX-FileCopyrightText: 2026 Univention GmbH
// SPDX-License-Identifier: AGPL-3.0-only

package main

// Registers the error detail types that Cerbos returns, so that the gateway can
// marshal them, as in internal/server/protodeps.go.
import _ "buf.build/gen/go/bufbuild/protovalidate/protocolbuffers/go/buf/validate"
